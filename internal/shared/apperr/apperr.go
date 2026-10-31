// Package apperr define o erro de aplicação. O domínio devolve esses erros sem
// conhecer HTTP; a camada http converte o Kind em status.
package apperr

import "errors"

type Kind string

const (
	KindValidation   Kind = "validation"
	KindNotFound     Kind = "not_found"
	KindConflict     Kind = "conflict"
	KindUnauthorized Kind = "unauthorized"
	KindInternal     Kind = "internal"
)

type Error struct {
	Kind    Kind
	Code    string // contrato externo, em inglês e estável: "INVALID_DOCUMENT"
	Message string // texto para o usuário, em português
	Err     error  // causa original; não vai para o cliente
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Message + ": " + e.Err.Error()
	}
	return e.Code + ": " + e.Message
}

func (e *Error) Unwrap() error { return e.Err }

func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

func Wrap(kind Kind, code, message string, cause error) *Error {
	return &Error{Kind: kind, Code: code, Message: message, Err: cause}
}

func Validation(code, message string) *Error   { return New(KindValidation, code, message) }
func NotFound(code, message string) *Error     { return New(KindNotFound, code, message) }
func Conflict(code, message string) *Error     { return New(KindConflict, code, message) }
func Unauthorized(code, message string) *Error { return New(KindUnauthorized, code, message) }

// As devolve false para erros que não são da aplicação; a camada http os
// trata como falha interna (500).
func As(err error) (*Error, bool) {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}
