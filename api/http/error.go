package http

import (
	"errors"
	"log"
	"net/http"

	"github.com/innermond/dots"
)

func Error(w http.ResponseWriter, r *http.Request, err error) {
	code, message, data := dots.ErrorCode(err), dots.ErrorMessage(err), dots.ErrorData(err)
	deverr := err
	logit := false
	if werr := errors.Unwrap(err); werr != nil {
		deverr = werr
		logit = true
	}
	if code == dots.EINTERNAL || logit {
		log.Printf("[http] error: %s %s %s", r.Method, r.URL.Path, deverr)
	}

	status := errorStatusFromCode(code)
	resp := errorResponse{Error: message}
	if len(data) != 0 {
		resp.Data = data
	}

	outputJSON(w, r, status, &resp)
}

type errorResponse struct {
	Error string                 `json:"error"`
	Data  map[string]interface{} `json:"data,omitempty"`
}

var codes = map[string]int{
	dots.ECONFLICT:       http.StatusConflict,
	dots.EINVALID:        http.StatusBadRequest,
	dots.ENOTFOUND:       http.StatusNotFound,
	dots.ENOTIMPLEMENTED: http.StatusNotImplemented,
	dots.EUNAUTHORIZED:   http.StatusUnauthorized,
	dots.EINTERNAL:       http.StatusInternalServerError,
}

func errorStatusFromCode(code string) int {
	if v, ok := codes[code]; ok {
		return v
	}
	return http.StatusInternalServerError
}

func codeFromErrorStatus(status int) string {
	for k, v := range codes {
		if v == status {
			return k
		}
	}
	return dots.EINTERNAL
}
