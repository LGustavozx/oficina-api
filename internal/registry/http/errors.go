package http

import "oficina-api/internal/shared/apperr"

func isNotFound(err error) bool {
	appErr, ok := apperr.As(err)
	return ok && appErr.Kind == apperr.KindNotFound
}
