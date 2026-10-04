package outbox

import (
	"testing"
	"time"
)

func TestRetryDelayIsBoundedExponential(t *testing.T) {
	for _, tc := range []struct {
		attempt int
		want    time.Duration
	}{
		{-1, time.Second},
		{1, time.Second},
		{2, 2 * time.Second},
		{9, 256 * time.Second},
		{10, 5 * time.Minute},
		{1_000_000, 5 * time.Minute},
	} {
		if actual := retryDelay(tc.attempt); actual != tc.want {
			t.Fatalf("attempt %d: delay=%s want=%s", tc.attempt, actual, tc.want)
		}
	}
}
