package infrastructure

import "golang.org/x/crypto/bcrypt"

type BcryptHasher struct {
	cost int
}

func NewBcryptHasher(cost int) BcryptHasher {
	return BcryptHasher{cost: cost}
}

func (h BcryptHasher) Hash(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), h.cost)
	return string(hash), err
}

func (BcryptHasher) Compare(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}
