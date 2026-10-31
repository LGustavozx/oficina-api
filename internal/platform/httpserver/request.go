package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/google/uuid"

	"oficina-api/internal/shared/apperr"
)

const (
	CodeInvalidRequest  = "INVALID_REQUEST"
	CodeInvalidEncoding = "INVALID_ENCODING"
	CodeInvalidID       = "INVALID_ID"
	CodeInvalidPaging   = "INVALID_PAGINATION"

	maxBodyBytes     = 1 << 20
	defaultPageLimit = 50
	maxPageLimit     = 200
)

// DecodeJSON lê o corpo como JSON, recusando campos desconhecidos e texto que
// não esteja em UTF-8. Sem essa checagem, o decoder trocaria os bytes inválidos
// por "�" em silêncio (acontece quando um terminal do Windows envia acentos em
// outra codificação), e a mensagem de erro de quem validasse depois confundiria.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		return apperr.Validation(CodeInvalidRequest, "corpo da requisição inválido")
	}
	if !utf8.Valid(body) {
		return apperr.Validation(CodeInvalidEncoding, "o corpo da requisição deve estar em UTF-8")
	}

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return apperr.Validation(CodeInvalidRequest, "corpo da requisição inválido")
	}
	return nil
}

func PathUUID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		return uuid.Nil, apperr.Validation(CodeInvalidID, "identificador inválido")
	}
	return id, nil
}

// Pagination lê ?limit= e ?offset=, com padrão 50 e máximo 200 por página.
func Pagination(r *http.Request) (limit, offset int, err error) {
	limit, err = intParam(r, "limit", defaultPageLimit)
	if err != nil || limit < 1 || limit > maxPageLimit {
		return 0, 0, apperr.Validation(CodeInvalidPaging, "limit deve estar entre 1 e 200")
	}

	offset, err = intParam(r, "offset", 0)
	if err != nil || offset < 0 {
		return 0, 0, apperr.Validation(CodeInvalidPaging, "offset deve ser maior ou igual a 0")
	}
	return limit, offset, nil
}

func intParam(r *http.Request, name string, fallback int) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, nil
	}
	return strconv.Atoi(raw)
}
