package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"badmintonhub/internal/modules/matches"
)

func TestR1ProtectedRoutesFailClosed(t *testing.T) {
	handler := NewHandlerWithOptions(Options{Register: R1Routes{}.Register})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/matches/match-1/join", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", response.Code)
	}
	var body ErrorBody
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "authentication_required" || body.Error.RequestID == "" {
		t.Fatalf("body=%+v", body)
	}
}

func TestSourceKeyUsesRemoteAddressNotForwardedHeader(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	request.RemoteAddr = "203.0.113.10:4321"
	request.Header.Set("X-Forwarded-For", "198.51.100.7")
	first := sourceKey(request)
	request.Header.Set("X-Forwarded-For", "192.0.2.1")
	if second := sourceKey(request); first != second {
		t.Fatal("untrusted forwarding header changed source key")
	}
	request.RemoteAddr = "203.0.113.11:4321"
	if sourceKey(request) == first {
		t.Fatal("different remote address should have a different source key")
	}
}

func TestIdempotencyKeyValidation(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	if _, ok := idempotencyKey(request); ok {
		t.Fatal("missing key accepted")
	}
	request.Header.Set("Idempotency-Key", "join-123")
	if key, ok := idempotencyKey(request); !ok || key != "join-123" {
		t.Fatalf("key=%q ok=%v", key, ok)
	}
	request.Header.Set("Idempotency-Key", string(make([]byte, 129)))
	if _, ok := idempotencyKey(request); ok {
		t.Fatal("oversized key accepted")
	}
}

func TestR1DomainErrorIsStableAndSafe(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	response := httptest.NewRecorder()
	R1Routes{}.writeDomainError(response, request, matches.ErrScheduleConflict)
	var body ErrorBody
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusConflict || body.Error.Code != "schedule_conflict" {
		t.Fatalf("status=%d body=%+v", response.Code, body)
	}
}
