// Package erros define os erros de domínio e suas categorias, independentes de HTTP.
package erros

import (
	"errors"
	"fmt"
)

// Categorias de erro. A camada de apresentação as mapeia para códigos HTTP.
var (
	ErrValidacao     = errors.New("validação")
	ErrNaoEncontrado = errors.New("não encontrado")
	ErrConflito      = errors.New("conflito")
	ErrNaoAutorizado = errors.New("não autorizado")
)

// Erro é um erro de domínio com código estável e mensagem legível.
type Erro struct {
	Categoria error
	Codigo    string
	Mensagem  string
}

func (e *Erro) Error() string { return fmt.Sprintf("%s: %s", e.Codigo, e.Mensagem) }

// Unwrap permite errors.Is(err, erros.ErrValidacao) e similares.
func (e *Erro) Unwrap() error { return e.Categoria }

func novo(cat error, codigo, mensagem string) *Erro {
	return &Erro{Categoria: cat, Codigo: codigo, Mensagem: mensagem}
}

// Validacao cria um erro de entrada inválida.
func Validacao(codigo, mensagem string) *Erro { return novo(ErrValidacao, codigo, mensagem) }

// NaoEncontrado cria um erro de recurso inexistente.
func NaoEncontrado(codigo, mensagem string) *Erro { return novo(ErrNaoEncontrado, codigo, mensagem) }

// Conflito cria um erro de violação de estado ou unicidade.
func Conflito(codigo, mensagem string) *Erro { return novo(ErrConflito, codigo, mensagem) }

// NaoAutorizado cria um erro de credencial ou permissão.
func NaoAutorizado(codigo, mensagem string) *Erro { return novo(ErrNaoAutorizado, codigo, mensagem) }

// Codigo extrai o código estável de um erro, ou "" se não for *Erro.
func Codigo(err error) string {
	var e *Erro
	if errors.As(err, &e) {
		return e.Codigo
	}
	return ""
}
