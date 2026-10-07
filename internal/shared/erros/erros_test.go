package erros

import (
	"errors"
	"fmt"
	"testing"
)

func TestCategorias(t *testing.T) {
	casos := []struct {
		nome string
		err  *Erro
		cat  error
	}{
		{"validacao", Validacao("X", "m"), ErrValidacao},
		{"nao encontrado", NaoEncontrado("X", "m"), ErrNaoEncontrado},
		{"conflito", Conflito("X", "m"), ErrConflito},
		{"nao autorizado", NaoAutorizado("X", "m"), ErrNaoAutorizado},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if !errors.Is(c.err, c.cat) {
				t.Errorf("errors.Is deveria reconhecer %v", c.cat)
			}
			outra := ErrConflito
			if errors.Is(c.cat, ErrConflito) {
				outra = ErrValidacao
			}
			if errors.Is(c.err, outra) {
				t.Errorf("não deveria casar com %v", outra)
			}
		})
	}
}

func TestMensagemECodigo(t *testing.T) {
	err := Validacao("CPF_INVALIDO", "CPF inválido")
	if got, want := err.Error(), "CPF_INVALIDO: CPF inválido"; got != want {
		t.Errorf("Error() = %q, esperado %q", got, want)
	}
	embrulhado := fmt.Errorf("contexto: %w", err)
	if got := Codigo(embrulhado); got != "CPF_INVALIDO" {
		t.Errorf("Codigo() = %q", got)
	}
	if got := Codigo(errors.New("comum")); got != "" {
		t.Errorf("Codigo() de erro comum = %q, esperado vazio", got)
	}
}
