package security

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/LGustavozx/oficina-api/internal/identity/domain"
	"github.com/LGustavozx/oficina-api/internal/shared/apperr"
	"github.com/LGustavozx/oficina-api/internal/shared/clock"
)

const testSigningKey = "um-segredo-de-teste-com-mais-de-32-bytes"

var testNow = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

func testUser() *domain.User {
	return &domain.User{ID: "user-1", Email: "a@b.com", Role: domain.RoleAdmin, Active: true}
}

func newTestJWT(t *testing.T, clk *clock.Fixed) *JWT {
	t.Helper()
	j, err := NewJWT(testSigningKey, time.Hour, clk)
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func TestBcrypt(t *testing.T) {
	b := NewBcrypt(bcrypt.MinCost)
	hash, err := b.Hash("segredo123")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "segredo123" || !b.Compare(hash, "segredo123") {
		t.Error("hash deve ser diferente da senha e comparar corretamente")
	}
	if b.Compare(hash, "errada") || b.Compare("não-é-um-hash", "segredo123") {
		t.Error("senha errada ou hash inválido não pode comparar como verdadeiro")
	}
	if _, err := b.Hash(strings.Repeat("a", 100)); err == nil {
		t.Error("senha acima de 72 bytes deveria falhar no bcrypt")
	}
}

func TestNewBcrypt_InvalidCostFallsBackToDefault(t *testing.T) {
	for _, cost := range []int{0, 1, 99} {
		if got := NewBcrypt(cost).cost; got != bcrypt.DefaultCost {
			t.Errorf("custo %d: obtido %d, esperado %d", cost, got, bcrypt.DefaultCost)
		}
	}
}

func TestNewJWT_Validation(t *testing.T) {
	clk := &clock.Fixed{Instant: testNow}
	if _, err := NewJWT("curto", time.Hour, clk); err == nil {
		t.Error("segredo curto deveria falhar")
	}
	if _, err := NewJWT(testSigningKey, 0, clk); err == nil {
		t.Error("TTL zero deveria falhar")
	}
}

func TestJWT_IssueAndValidate(t *testing.T) {
	clk := &clock.Fixed{Instant: testNow}
	j := newTestJWT(t, clk)

	token, err := j.Issue(testUser())
	if err != nil {
		t.Fatal(err)
	}
	if token.TTL != time.Hour || !token.ExpiresAt.Equal(testNow.Add(time.Hour)) {
		t.Errorf("token = %+v", token)
	}

	claims, err := j.Validate(token.Value)
	if err != nil {
		t.Fatalf("token válido rejeitado: %v", err)
	}
	if claims.UserID != "user-1" || claims.Role != domain.RoleAdmin || !claims.ExpiresAt.Equal(testNow.Add(time.Hour)) {
		t.Errorf("claims = %+v", claims)
	}
}

func TestJWT_UniqueTokenIDs(t *testing.T) {
	j := newTestJWT(t, &clock.Fixed{Instant: testNow})
	a, _ := j.Issue(testUser())
	b, _ := j.Issue(testUser())
	if a.Value == b.Value {
		t.Error("cada emissão deve gerar um jti distinto")
	}
}

func TestJWT_Expired(t *testing.T) {
	clk := &clock.Fixed{Instant: testNow}
	j := newTestJWT(t, clk)
	token, _ := j.Issue(testUser())

	clk.Advance(time.Hour + time.Second)

	_, err := j.Validate(token.Value)
	if !errors.Is(err, apperr.ErrUnauthorized) || apperr.CodeOf(err) != "TOKEN_EXPIRADO" {
		t.Errorf("esperava TOKEN_EXPIRADO, obtido %v", err)
	}
}

func TestJWT_RejectsInvalidTokens(t *testing.T) {
	clk := &clock.Fixed{Instant: testNow}
	j := newTestJWT(t, clk)
	valid, _ := j.Issue(testUser())

	sign := func(method jwt.SigningMethod, claims jwt.Claims, key any) string {
		s, err := jwt.NewWithClaims(method, claims).SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	base := func(subject, issuer string) jwtClaims {
		return jwtClaims{Role: "ADMIN", RegisteredClaims: jwt.RegisteredClaims{
			Issuer: issuer, Subject: subject,
			IssuedAt:  jwt.NewNumericDate(testNow),
			ExpiresAt: jwt.NewNumericDate(testNow.Add(time.Hour)),
		}}
	}
	otherJWT, _ := NewJWT("outro-segredo-totalmente-diferente-123", time.Hour, clk)
	foreign, _ := otherJWT.Issue(testUser())

	cases := map[string]string{
		"vazio":               "",
		"lixo":                "isso.nao.e-jwt",
		"assinatura alterada": valid.Value[:len(valid.Value)-2] + "xx",
		"segredo diferente":   foreign.Value,
		"alg none":            sign(jwt.SigningMethodNone, base("user-1", issuerName), jwt.UnsafeAllowNoneSignatureType),
		"alg HS512":           sign(jwt.SigningMethodHS512, base("user-1", issuerName), []byte(testSigningKey)),
		"emissor errado":      sign(jwt.SigningMethodHS256, base("user-1", "outro"), []byte(testSigningKey)),
		"sem subject":         sign(jwt.SigningMethodHS256, base("", issuerName), []byte(testSigningKey)),
		"sem perfil": sign(jwt.SigningMethodHS256, jwtClaims{RegisteredClaims: jwt.RegisteredClaims{
			Issuer: issuerName, Subject: "user-1", ExpiresAt: jwt.NewNumericDate(testNow.Add(time.Hour)),
		}}, []byte(testSigningKey)),
		"sem expiração": sign(jwt.SigningMethodHS256, jwtClaims{Role: "ADMIN", RegisteredClaims: jwt.RegisteredClaims{
			Issuer: issuerName, Subject: "user-1",
		}}, []byte(testSigningKey)),
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := j.Validate(token)
			if !errors.Is(err, apperr.ErrUnauthorized) || apperr.CodeOf(err) != "TOKEN_INVALIDO" {
				t.Errorf("esperava TOKEN_INVALIDO, obtido %v", err)
			}
		})
	}
}
