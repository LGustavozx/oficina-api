// Package httpx reúne utilitários HTTP compartilhados: JSON, decodificação segura e mapeamento de erros.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/LGustavozx/oficina-api/internal/shared/apperr"
)

const maxBodyBytes = 1 << 20 // 1 MiB

type errorBody struct {
	Code    string   `json:"codigo"`
	Message string   `json:"mensagem"`
	Details []string `json:"detalhes"`
}

// JSON escreve o corpo como JSON com o status informado.
func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// Error traduz um erro de domínio para a resposta HTTP padrão. Erros inesperados
// viram 500 genérico, sem vazar detalhes internos.
func Error(w http.ResponseWriter, err error) {
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		slog.Error("erro inesperado", "erro", err)
		JSON(w, http.StatusInternalServerError, errorBody{
			Code: "ERRO_INTERNO", Message: "erro interno do servidor", Details: []string{},
		})
		return
	}
	JSON(w, statusOf(appErr), errorBody{Code: appErr.Code, Message: appErr.Message, Details: []string{}})
}

func statusOf(e *apperr.Error) int {
	switch {
	case errors.Is(e, apperr.ErrValidation):
		return http.StatusUnprocessableEntity
	case errors.Is(e, apperr.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(e, apperr.ErrConflict):
		return http.StatusConflict
	case errors.Is(e, apperr.ErrUnauthorized):
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

// Decode lê o corpo JSON em dst, limitando o tamanho e rejeitando campos
// desconhecidos e conteúdo extra (mitiga mass assignment).
func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return apperr.Validation("CORPO_MUITO_GRANDE", "corpo da requisição excede o limite permitido")
		}
		return apperr.Validation("JSON_INVALIDO", "corpo da requisição inválido")
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return apperr.Validation("JSON_INVALIDO", "corpo da requisição inválido")
	}
	return nil
}
