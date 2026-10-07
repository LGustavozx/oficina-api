package security

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/LGustavozx/oficina-api/internal/identity/domain"
	"github.com/LGustavozx/oficina-api/internal/shared/apperr"
	"github.com/LGustavozx/oficina-api/internal/shared/clock"
)

const (
	issuerName       = "oficina-api"
	minSecretLen     = 32
	signingAlgorithm = "HS256"
	invalidAccessMsg = "token de acesso inválido"
)

// JWT implementa domain.TokenIssuer com HS256.
type JWT struct {
	secret []byte
	ttl    time.Duration
	clock  clock.Clock
}

type jwtClaims struct {
	Role string `json:"perfil"`
	jwt.RegisteredClaims
}

// NewJWT cria o emissor. O segredo precisa ter ao menos 32 bytes.
func NewJWT(secret string, ttl time.Duration, c clock.Clock) (*JWT, error) {
	if len(secret) < minSecretLen {
		return nil, fmt.Errorf("segredo JWT deve ter ao menos %d bytes", minSecretLen)
	}
	if ttl <= 0 {
		return nil, errors.New("TTL do JWT deve ser positivo")
	}
	return &JWT{secret: []byte(secret), ttl: ttl, clock: c}, nil
}

// Issue assina um token com iss, sub, perfil, iat, exp e jti.
func (j *JWT) Issue(u *domain.User) (domain.Token, error) {
	now := j.clock.Now()
	expires := now.Add(j.ttl)
	claims := jwtClaims{
		Role: string(u.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuerName,
			Subject:   u.ID,
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expires),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
	if err != nil {
		return domain.Token{}, fmt.Errorf("assinar token: %w", err)
	}
	return domain.Token{Value: signed, ExpiresAt: expires, TTL: j.ttl}, nil
}

// Validate verifica assinatura, algoritmo, emissor e expiração.
func (j *JWT) Validate(token string) (domain.Claims, error) {
	var c jwtClaims
	_, err := jwt.ParseWithClaims(token, &c,
		func(*jwt.Token) (any, error) { return j.secret, nil },
		jwt.WithValidMethods([]string{signingAlgorithm}),
		jwt.WithIssuer(issuerName),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(j.clock.Now),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return domain.Claims{}, apperr.Unauthorized("TOKEN_EXPIRADO", "token de acesso expirado")
		}
		return domain.Claims{}, apperr.Unauthorized("TOKEN_INVALIDO", invalidAccessMsg)
	}
	if c.Subject == "" || c.Role == "" || c.ExpiresAt == nil {
		return domain.Claims{}, apperr.Unauthorized("TOKEN_INVALIDO", invalidAccessMsg)
	}
	return domain.Claims{
		UserID:    c.Subject,
		Role:      domain.Role(c.Role),
		ExpiresAt: c.ExpiresAt.Time,
	}, nil
}
