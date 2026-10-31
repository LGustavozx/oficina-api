package infrastructure

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"oficina-api/internal/identity/application"
	"oficina-api/internal/shared/apperr"
	"oficina-api/internal/shared/clock"
)

const jwtIssuer = "oficina-api"

type JWTManager struct {
	secret []byte
	ttl    time.Duration
	clock  clock.Clock
}

func NewJWTManager(secret string, ttl time.Duration, clk clock.Clock) *JWTManager {
	return &JWTManager{secret: []byte(secret), ttl: ttl, clock: clk}
}

func (m *JWTManager) Issue(userID uuid.UUID) (application.Token, error) {
	now := m.clock.Now()
	expiresAt := now.Add(m.ttl)

	claims := jwt.RegisteredClaims{
		Issuer:    jwtIssuer,
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return application.Token{}, err
	}
	return application.Token{Value: signed, ExpiresAt: expiresAt}, nil
}

func (m *JWTManager) Verify(raw string) (uuid.UUID, error) {
	claims := &jwt.RegisteredClaims{}

	// WithValidMethods fixa o HS256: impede aceitar tokens com alg "none" ou
	// assinados com outro algoritmo.
	_, err := jwt.ParseWithClaims(raw, claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(jwtIssuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(m.clock.Now),
	)
	if err != nil {
		return uuid.Nil, invalidToken()
	}

	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, invalidToken()
	}
	return id, nil
}

func invalidToken() error {
	return apperr.Unauthorized(application.CodeInvalidToken, "token de acesso ausente ou inválido")
}
