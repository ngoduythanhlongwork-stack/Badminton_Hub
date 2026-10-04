//go:build integration

package outbox

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"badmintonhub/internal/platform/clock"
	"badmintonhub/internal/platform/migrations"
	"badmintonhub/internal/platform/testdb"
)

func TestEnqueueAndHandleTransactionally(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Open(t)
	runner, err := migrations.NewRunner(pool)
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Up(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	rolledBack, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Enqueue(ctx, rolledBack, "sample.created", "rolled-back", map[string]string{"value": "discard"}, now); err != nil {
		t.Fatal(err)
	}
	if err := rolledBack.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var rolledBackCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM platform.outbox_messages WHERE idempotency_key='rolled-back'").Scan(&rolledBackCount); err != nil {
		t.Fatal(err)
	}
	if rolledBackCount != 0 {
		t.Fatalf("rolled-back transaction left %d outbox rows", rolledBackCount)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	first, err := Enqueue(ctx, tx, "sample.created", "sample-1", map[string]string{"value": "ok"}, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Enqueue(ctx, tx, "sample.created", "sample-1", map[string]string{"value": "duplicate"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("same idempotency key must return the existing message")
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var handled atomic.Int32
	var startedOnce sync.Once
	handlerStarted := make(chan struct{})
	releaseHandler := make(chan struct{})
	worker := Worker{
		Pool: pool, Clock: clock.Fixed{Time: now}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Handlers: map[string]Handler{"sample.created": func(_ context.Context, message Message) error {
			call := handled.Add(1)
			startedOnce.Do(func() { close(handlerStarted) })
			if message.ID != first || message.IdempotencyKey != "sample-1" {
				return fmt.Errorf("unexpected message: %+v", message)
			}
			if call == 1 {
				<-releaseHandler
			}
			return nil
		}},
	}
	if err := worker.defaults(); err != nil {
		t.Fatal(err)
	}
	firstDone := make(chan error, 1)
	go func() { firstDone <- worker.processBatch(ctx) }()
	<-handlerStarted
	// A second worker must skip the unexpired row claimed by the first worker.
	if err := worker.processBatch(ctx); err != nil {
		t.Fatal(err)
	}
	close(releaseHandler)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if handled.Load() != 1 {
		t.Fatalf("handled=%d, want 1", handled.Load())
	}
	var status string
	if err := pool.QueryRow(ctx, "SELECT status FROM platform.outbox_messages WHERE id=$1", first).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "processed" {
		t.Fatalf("status=%s", status)
	}
}
