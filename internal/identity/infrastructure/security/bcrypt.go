// Package security implementa hash de senha (bcrypt) e tokens JWT (HS256).
package security

import "golang.org/x/crypto/bcrypt"

// Bcrypt implementa domain.PasswordHasher.
type Bcrypt struct{ cost int }

// NewBcrypt cria o hasher; custos fora da faixa válida usam o padrão do bcrypt.
func NewBcrypt(cost int) Bcrypt {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = bcrypt.DefaultCost
	}
	return Bcrypt{cost: cost}
}

// Hash gera o hash bcrypt da senha.
func (b Bcrypt) Hash(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), b.cost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// Compare verifica se a senha corresponde ao hash.
func (Bcrypt) Compare(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
