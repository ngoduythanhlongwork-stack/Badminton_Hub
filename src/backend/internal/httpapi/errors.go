package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type ErrorBody struct {
	Error APIError `json:"error"`
}

type APIError struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSONStatus(w, status, ErrorBody{Error: APIError{Code: code, Message: message, RequestID: RequestID(r.Context())}})
}

// DecodeJSON decodes one JSON value and maps syntax and bounded-body failures
// to stable client errors. Handlers remain responsible for field validation.
func DecodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var maxBytes *http.MaxBytesError
		if errors.As(err, &maxBytes) {
			WriteError(w, r, http.StatusRequestEntityTooLarge, "request_body_too_large", "Request body exceeds the allowed size.")
			return false
		}
		WriteError(w, r, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		WriteError(w, r, http.StatusBadRequest, "invalid_json", "Request body must contain exactly one JSON value.")
		return false
	}
	return true
}

func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
