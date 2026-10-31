package infrastructure

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestBcryptHasher(t *testing.T) {
	h := NewBcryptHasher(bcrypt.MinCost)

	hash, err := h.Hash("senha-forte-123")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if hash == "senha-forte-123" || !strings.HasPrefix(hash, "$2") {
		t.Errorf("hash não parece bcrypt: %q", hash)
	}

	if err := h.Compare(hash, "senha-forte-123"); err != nil {
		t.Errorf("senha correta rejeitada: %v", err)
	}
	if err := h.Compare(hash, "outra-senha"); err == nil {
		t.Error("senha errada aceita")
	}
}

func TestBcryptHashesDiffer(t *testing.T) {
	h := NewBcryptHasher(bcrypt.MinCost)

	a, _ := h.Hash("mesma-senha")
	b, _ := h.Hash("mesma-senha")
	if a == b {
		t.Error("o salt deveria tornar os hashes diferentes")
	}
}

func TestBcryptHashFailsWithInvalidCost(t *testing.T) {
	if _, err := NewBcryptHasher(100).Hash("x"); err == nil {
		t.Error("esperado erro com custo inválido")
	}
}
