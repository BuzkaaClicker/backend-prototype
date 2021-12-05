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
	return logrus.WithField("remote_addr", r.RemoteAddr).WithField("path", r.URL.Path)
}
