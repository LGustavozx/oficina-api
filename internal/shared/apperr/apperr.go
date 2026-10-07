// Package apperr define os erros de domínio e suas categorias, independentes de HTTP.
package apperr

import (
	"errors"
	"fmt"
)

// Categorias de erro. A camada de apresentação as mapeia para códigos HTTP.
var (
	ErrValidation   = errors.New("validação")
	ErrNotFound     = errors.New("não encontrado")
	ErrConflict     = errors.New("conflito")
	ErrUnauthorized = errors.New("não autorizado")
)

// Error é um erro de domínio com código estável e mensagem legível.
type Error struct {
	Category error
	Code     string
	Message  string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// Unwrap permite errors.Is(err, apperr.ErrValidation) e similares.
func (e *Error) Unwrap() error { return e.Category }

func newError(category error, code, message string) *Error {
	return &Error{Category: category, Code: code, Message: message}
}

// Validation cria um erro de entrada inválida.
func Validation(code, message string) *Error { return newError(ErrValidation, code, message) }

// NotFound cria um erro de recurso inexistente.
func NotFound(code, message string) *Error { return newError(ErrNotFound, code, message) }

// Conflict cria um erro de violação de estado ou unicidade.
func Conflict(code, message string) *Error { return newError(ErrConflict, code, message) }

// Unauthorized cria um erro de credencial ou permissão.
func Unauthorized(code, message string) *Error { return newError(ErrUnauthorized, code, message) }

// CodeOf extrai o código estável de um erro, ou "" se não for *Error.
func CodeOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}
