package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPMetrics(t *testing.T) {
	metrics := &HTTP{}
	finish := metrics.Begin()
	finish(http.StatusInternalServerError, 75*time.Millisecond)
	response := httptest.NewRecorder()
	metrics.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := response.Body.String()
	for _, expected := range []string{"badminton_http_requests_total 1", "badminton_http_errors_total 1", `le="0.100"} 1`, "badminton_http_request_duration_seconds_count 1"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in %s", expected, body)
		}
	}
}
