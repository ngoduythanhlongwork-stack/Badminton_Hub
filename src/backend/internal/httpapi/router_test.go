package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoutes(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/", 200}, {"GET", "/health", 200},
		{"GET", "/missing", 404}, {"POST", "/health", 405},
		{"GET", "/health/extra", 404},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			NewHandler().ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, nil))
			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d", response.Code, tc.status)
			}
			if tc.status == http.StatusOK {
				if response.Header().Get("Content-Type") != "application/json" {
					t.Fatal("expected JSON")
				}
				var body map[string]string
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if tc.path == "/health" && body["status"] != "ok" {
					t.Fatal("unexpected health response")
				}
				if tc.path == "/" && body["architecture"] != "modular-monolith" {
					t.Fatal("unexpected service response")
				}
			}
		})
	}
}

func TestModuleRoutesAreRegisteredInsideSharedMiddleware(t *testing.T) {
	handler := NewHandlerWithOptions(Options{Register: func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api/v1/example", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, map[string]string{"status": "module"})
		})
	}})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/example", nil))
	if response.Code != http.StatusOK || response.Header().Get(RequestIDHeader) == "" {
		t.Fatalf("status=%d requestID=%q", response.Code, response.Header().Get(RequestIDHeader))
	}
}
