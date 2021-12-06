package main

import (
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"
)

func setJsonContentType(header http.Header) {
	header.Set("Content-Type", "application/json")
	header.Set("X-Content-Type-Options", "nosniff")
}

type ErrorResponse struct {
	ErrorMessage string `json:"error_message"`
}

// Write http internal error status code and error message wrapped in json
func writeInternalError(w http.ResponseWriter, errorMessage string) error {
	setJsonContentType(w.Header())
	w.WriteHeader(http.StatusInternalServerError)
	return json.NewEncoder(w).Encode(&ErrorResponse{ErrorMessage: errorMessage})
}

func requestLog(r *http.Request) *logrus.Entry {
	return logrus.
		WithField("remote_addr", r.RemoteAddr).
		WithField("url", r.URL).
		WithField("z_referer", r.Header.Get("Referer")).
		WithField("z_user_agent", r.Header.Get("User-Agent")).
		WithField("z_x_forwared_for", r.Header.Get("X-Forwarded-For"))
}
