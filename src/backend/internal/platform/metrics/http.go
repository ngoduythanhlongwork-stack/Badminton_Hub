package metrics

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// HTTP keeps low-cardinality process-local counters. PostgreSQL remains the
// source of truth for business metrics; these counters are operational only.
type HTTP struct {
	requests atomic.Uint64
	errors   atomic.Uint64
	inFlight atomic.Int64
	duration atomic.Uint64
	buckets  [6]atomic.Uint64
}

var bucketBounds = [...]time.Duration{50 * time.Millisecond, 100 * time.Millisecond, 250 * time.Millisecond, 500 * time.Millisecond, time.Second}

func (m *HTTP) Begin() func(int, time.Duration) {
	m.inFlight.Add(1)
	return func(status int, elapsed time.Duration) {
		m.inFlight.Add(-1)
		m.requests.Add(1)
		if status >= http.StatusInternalServerError {
			m.errors.Add(1)
		}
		m.duration.Add(uint64(elapsed.Nanoseconds()))
		index := len(bucketBounds)
		for i, bound := range bucketBounds {
			if elapsed <= bound {
				index = i
				break
			}
		}
		m.buckets[index].Add(1)
	}
}

func (m *HTTP) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	requests := m.requests.Load()
	_, _ = fmt.Fprintf(w, "# HELP badminton_http_requests_total Total HTTP requests.\n# TYPE badminton_http_requests_total counter\nbadminton_http_requests_total %d\n", requests)
	_, _ = fmt.Fprintf(w, "# HELP badminton_http_errors_total Total HTTP 5xx responses.\n# TYPE badminton_http_errors_total counter\nbadminton_http_errors_total %d\n", m.errors.Load())
	_, _ = fmt.Fprintf(w, "# HELP badminton_http_in_flight Current HTTP requests.\n# TYPE badminton_http_in_flight gauge\nbadminton_http_in_flight %d\n", m.inFlight.Load())
	_, _ = fmt.Fprintf(w, "# HELP badminton_http_request_duration_seconds HTTP request duration.\n# TYPE badminton_http_request_duration_seconds histogram\n")
	cumulative := uint64(0)
	for i, bound := range bucketBounds {
		cumulative += m.buckets[i].Load()
		_, _ = fmt.Fprintf(w, "badminton_http_request_duration_seconds_bucket{le=\"%.3f\"} %d\n", bound.Seconds(), cumulative)
	}
	cumulative += m.buckets[len(bucketBounds)].Load()
	_, _ = fmt.Fprintf(w, "badminton_http_request_duration_seconds_bucket{le=\"+Inf\"} %d\n", cumulative)
	_, _ = fmt.Fprintf(w, "badminton_http_request_duration_seconds_sum %.6f\n", float64(m.duration.Load())/float64(time.Second))
	_, _ = fmt.Fprintf(w, "badminton_http_request_duration_seconds_count %d\n", requests)
}
