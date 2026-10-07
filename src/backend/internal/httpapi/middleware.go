package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"badmintonhub/internal/platform/id"
	"badmintonhub/internal/platform/metrics"
)

const (
	RequestIDHeader    = "X-Request-ID"
	DefaultMaxBodySize = 1 << 20
)

type requestIDKey struct{}

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey{}).(string)
	return value
}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value := r.Header.Get(RequestIDHeader)
		if !validRequestID.MatchString(value) {
			value, _ = id.New()
		}
		w.Header().Set(RequestIDHeader, value)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, value)))
	})
}

func boundedBody(maxBytes int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		}
		next.ServeHTTP(w, r)
	})
}

func recoverPanics(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("HTTP panic recovered", "request_id", RequestID(r.Context()), "method", r.Method, "path", r.URL.Path)
				WriteError(w, r, http.StatusInternalServerError, "internal_error", "An unexpected error occurred.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *responseRecorder) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *responseRecorder) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseRecorder) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	count, err := w.ResponseWriter.Write(body)
	w.bytes += count
	return count, err
}

func accessLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		logger.Info("HTTP request", "request_id", RequestID(r.Context()), "method", r.Method,
			"path", r.URL.Path, "status", status, "bytes", recorder.bytes, "duration_ms", time.Since(started).Milliseconds())
	})
}

func measureHTTP(metric *metrics.HTTP, next http.Handler) http.Handler {
	if metric == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &responseRecorder{ResponseWriter: w}
		finish := metric.Begin()
		defer func() {
			status := recorder.status
			if status == 0 {
				status = http.StatusOK
			}
			finish(status, time.Since(started))
		}()
		next.ServeHTTP(recorder, r)
	})
}

func middleware(logger *slog.Logger, maxBodyBytes int64, metric *metrics.HTTP, next http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	if maxBodyBytes <= 0 {
		maxBodyBytes = DefaultMaxBodySize
	}
	return requestID(measureHTTP(metric, accessLog(logger, recoverPanics(logger, boundedBody(maxBodyBytes, next)))))
}
