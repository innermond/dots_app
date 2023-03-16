package http

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/innermond/dots"
)

func LogError(r *http.Request, err error) {
	log.Printf("[http] %s %s %s", r.Method, r.URL.Path, err)
}

func encodeJSON[T any](w http.ResponseWriter, r *http.Request, e *T) bool {
	if err := json.NewDecoder(r.Body).Decode(e); err != nil {
		Error(w, r, dots.Errorf(dots.EINVALID, "new entry: invalid json body"))
		return false
	}

	return true
}

func respondJSON[T any](w http.ResponseWriter, r *http.Request, status int, response *T) {
	w.Header().Set("Content-TYpe", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		LogError(r, err)
	}
}