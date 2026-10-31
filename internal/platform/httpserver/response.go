package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"oficina-api/internal/shared/apperr"
)

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// WriteError traduz o erro para HTTP. Erros que não são da aplicação viram 500
// com mensagem genérica: o detalhe vai só para o log, nunca para o cliente.
func WriteError(w http.ResponseWriter, logger *slog.Logger, err error) {
	appErr, ok := apperr.As(err)
	if !ok || appErr.Kind == apperr.KindInternal {
		logger.Error("erro interno", "error", err)
		WriteJSON(w, http.StatusInternalServerError, errorResponse{
			Code:    "INTERNAL_ERROR",
			Message: "erro interno do servidor",
		})
		return
	}

	if appErr.Kind == apperr.KindUnauthorized {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	WriteJSON(w, statusFor(appErr.Kind), errorResponse{Code: appErr.Code, Message: appErr.Message})
}

func statusFor(kind apperr.Kind) int {
	switch kind {
	case apperr.KindValidation:
		return http.StatusBadRequest
	case apperr.KindNotFound:
		return http.StatusNotFound
	case apperr.KindConflict:
		return http.StatusConflict
	case apperr.KindUnauthorized:
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}
