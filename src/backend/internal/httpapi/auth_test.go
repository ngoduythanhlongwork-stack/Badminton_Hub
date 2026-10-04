package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAuth(t *testing.T) {
	authenticator := AuthenticateFunc(func(_ context.Context, token string) (Principal, error) {
		if token != "valid-token" {
			return Principal{}, errors.New("invalid")
		}
		return Principal{AccountID: "account-1"}, nil
	})
	handler := RequireAuth(authenticator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok || principal.AccountID != "account-1" {
			t.Fatal("principal missing from context")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, tc := range []struct {
		name, header string
		status       int
	}{
		{"missing", "", http.StatusUnauthorized},
		{"wrong scheme", "Basic abc", http.StatusUnauthorized},
		{"invalid", "Bearer invalid", http.StatusUnauthorized},
		{"valid", "Bearer valid-token", http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set("Authorization", tc.header)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("status=%d, want %d", response.Code, tc.status)
			}
		})
	}
}

func TestRequireAuthFailsClosedWithoutAuthenticator(t *testing.T) {
	response := httptest.NewRecorder()
	RequireAuth(nil, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler should not run")
	})).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", response.Code)
	}
}
