package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestIDIsAcceptedOrReplaced(t *testing.T) {
	for _, tc := range []struct {
		name, supplied string
		preserved      bool
	}{
		{"valid", "request-123", true},
		{"missing", "", false},
		{"invalid", "contains spaces", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/health", nil)
			request.Header.Set(RequestIDHeader, tc.supplied)
			response := httptest.NewRecorder()
			NewHandler().ServeHTTP(response, request)
			actual := response.Header().Get(RequestIDHeader)
			if actual == "" || tc.preserved && actual != tc.supplied || !tc.preserved && actual == tc.supplied && tc.supplied != "" {
				t.Fatalf("request ID=%q supplied=%q", actual, tc.supplied)
			}
			var body map[string]string
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStableRoutingErrors(t *testing.T) {
	for _, tc := range []struct {
		method, path, code string
		status             int
	}{
		{http.MethodGet, "/missing", "route_not_found", http.StatusNotFound},
		{http.MethodPost, "/health", "method_not_allowed", http.StatusMethodNotAllowed},
	} {
		response := httptest.NewRecorder()
		NewHandler().ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, nil))
		var body ErrorBody
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if response.Code != tc.status || body.Error.Code != tc.code || body.Error.RequestID == "" {
			t.Fatalf("status=%d body=%+v", response.Code, body)
		}
	}
}

func TestBoundedJSONBody(t *testing.T) {
	handler := middleware(slog.New(slog.NewTextHandler(io.Discard, nil)), 8, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if DecodeJSON(w, r, &body) {
			writeJSON(w, body)
		}
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"long":"value"}`)))
	var body ErrorBody
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusRequestEntityTooLarge || body.Error.Code != "request_body_too_large" {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestPanicRecoveryAndAccessLog(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	handler := middleware(logger, DefaultMaxBodySize, nil, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("secret") }))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))
	var body ErrorBody
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusInternalServerError || body.Error.Code != "internal_error" {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	logOutput := output.String()
	if !strings.Contains(logOutput, `"status":500`) || strings.Contains(logOutput, "secret") {
		t.Fatalf("unexpected log: %s", logOutput)
	}
}
