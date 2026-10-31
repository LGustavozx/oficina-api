package infrastructure

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"oficina-api/internal/identity/application"
	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/shared/clock"
)

const testSecret = "0123456789abcdef0123456789abcdef"

var baseTime = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

func newManager(secret string, at time.Time) *JWTManager {
	return NewJWTManager(secret, time.Hour, clock.Fixed{Time: at})
}

func assertInvalidToken(t *testing.T, err error) {
	t.Helper()
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != application.CodeInvalidToken || appErr.Kind != apperr.KindUnauthorized {
		t.Fatalf("erro = %v, esperado %s", err, application.CodeInvalidToken)
	}
}

func signWith(t *testing.T, method jwt.SigningMethod, key any, claims jwt.RegisteredClaims) string {
	t.Helper()
	signed, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func TestJWTRoundTrip(t *testing.T) {
	m := newManager(testSecret, baseTime)
	id := uuid.New()

	token, err := m.Issue(id)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if !token.ExpiresAt.Equal(baseTime.Add(time.Hour)) {
		t.Errorf("ExpiresAt = %v", token.ExpiresAt)
	}

	got, err := m.Verify(token.Value)
	if err != nil || got != id {
		t.Fatalf("Verify = %v, %v; esperado %v", got, err, id)
	}
}

func TestJWTVerifyRejects(t *testing.T) {
	id := uuid.New()
	valid := func() jwt.RegisteredClaims {
		return jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			Subject:   id.String(),
			ExpiresAt: jwt.NewNumericDate(baseTime.Add(time.Hour)),
		}
	}
	m := newManager(testSecret, baseTime)

	issued, err := m.Issue(id)
	if err != nil {
		t.Fatal(err)
	}

	otherIssuer := valid()
	otherIssuer.Issuer = "outro-sistema"

	noExpiry := valid()
	noExpiry.ExpiresAt = nil

	badSubject := valid()
	badSubject.Subject = "não-é-uuid"

	tests := []struct {
		name  string
		token string
	}{
		{"vazio", ""},
		{"lixo", "isso.nao.e.jwt"},
		{"expirado", issued.Value},
		{"assinado com outro segredo", signWith(t, jwt.SigningMethodHS256, []byte("outro-segredo-com-32-caracteres!!"), valid())},
		{"algoritmo none", signWith(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, valid())},
		{"algoritmo HS512", signWith(t, jwt.SigningMethodHS512, []byte(testSecret), valid())},
		{"emissor diferente", signWith(t, jwt.SigningMethodHS256, []byte(testSecret), otherIssuer)},
		{"sem expiração", signWith(t, jwt.SigningMethodHS256, []byte(testSecret), noExpiry)},
		{"subject inválido", signWith(t, jwt.SigningMethodHS256, []byte(testSecret), badSubject)},
		{"payload adulterado", tamper(issued.Value)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier := m
			if tt.name == "expirado" {
				verifier = newManager(testSecret, baseTime.Add(2*time.Hour))
			}
			_, err := verifier.Verify(tt.token)
			assertInvalidToken(t, err)
		})
	}
}

func tamper(token string) string {
	parts := strings.Split(token, ".")
	payload := []byte(parts[1])
	if payload[0] == 'A' {
		payload[0] = 'B'
	} else {
		payload[0] = 'A'
	}
	parts[1] = string(payload)
	return strings.Join(parts, ".")
}
