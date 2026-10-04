package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"badmintonhub/internal/platform/clock"
	"badmintonhub/internal/platform/id"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Enqueue must receive the business transaction. This guarantees the domain
// write and its durable delivery obligation commit or roll back together.
func Enqueue(ctx context.Context, tx pgx.Tx, topic, idempotencyKey string, payload any, occurredAt time.Time) (string, error) {
	if tx == nil || topic == "" || idempotencyKey == "" {
		return "", errors.New("transaction, topic, and idempotency key are required")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode outbox payload: %w", err)
	}
	messageID, err := id.New()
	if err != nil {
		return "", err
	}
	var storedID string
	err = tx.QueryRow(ctx, `INSERT INTO platform.outbox_messages
        (id, topic, payload, idempotency_key, occurred_at, available_at)
        VALUES ($1,$2,$3,$4,$5,$5)
        ON CONFLICT (topic, idempotency_key) DO UPDATE SET topic=EXCLUDED.topic
        RETURNING id`, messageID, topic, body, idempotencyKey, occurredAt.UTC()).Scan(&storedID)
	if err != nil {
		return "", fmt.Errorf("enqueue outbox message: %w", err)
	}
	return storedID, nil
}

type Message struct {
	ID             string
	Topic          string
	Payload        json.RawMessage
	IdempotencyKey string
	OccurredAt     time.Time
	Attempt        int
	LeaseToken     string
}

// Handler can receive the same message more than once after a crash or lease
// expiry. It must make its externally visible effect idempotent using Message.ID
// or IdempotencyKey.
type Handler func(context.Context, Message) error

type Worker struct {
	Pool         *pgxpool.Pool
	Handlers     map[string]Handler
	Logger       *slog.Logger
	Clock        clock.Clock
	PollInterval time.Duration
	Lease        time.Duration
	BatchSize    int
}

func (w Worker) Run(ctx context.Context) error {
	if err := w.defaults(); err != nil {
		return err
	}
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			if err := w.processBatch(ctx); err != nil && !errors.Is(err, context.Canceled) {
				w.Logger.Error("outbox batch failed", "error", err)
			}
			timer.Reset(w.PollInterval)
		}
	}
}

func (w *Worker) defaults() error {
	if w.Pool == nil {
		return errors.New("outbox pool is required")
	}
	if w.Logger == nil {
		w.Logger = slog.Default()
	}
	if w.Clock == nil {
		w.Clock = clock.System{}
	}
	if w.PollInterval <= 0 {
		w.PollInterval = time.Second
	}
	if w.Lease <= 0 {
		w.Lease = 30 * time.Second
	}
	if w.BatchSize <= 0 {
		w.BatchSize = 20
	}
	return nil
}

func (w Worker) processBatch(ctx context.Context) error {
	messages, err := w.claim(ctx)
	if err != nil {
		return err
	}
	for _, message := range messages {
		handler, ok := w.Handlers[message.Topic]
		if !ok {
			err = fmt.Errorf("no handler registered for topic %q", message.Topic)
		} else {
			err = handler(ctx, message)
		}
		if err == nil {
			err = w.complete(ctx, message)
		} else {
			w.Logger.Warn("outbox handler failed", "message_id", message.ID, "topic", message.Topic, "attempt", message.Attempt)
			err = w.retry(ctx, message, err)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (w Worker) claim(ctx context.Context) ([]Message, error) {
	now := w.Clock.Now().UTC()
	leaseToken, err := id.New()
	if err != nil {
		return nil, err
	}
	rows, err := w.Pool.Query(ctx, `WITH candidates AS (
        SELECT id FROM platform.outbox_messages
        WHERE (status='pending' AND available_at <= $1)
           OR (status='processing' AND lease_until <= $1)
        ORDER BY available_at, occurred_at
        FOR UPDATE SKIP LOCKED LIMIT $2
    )
    UPDATE platform.outbox_messages o
    SET status='processing', lease_token=$3, lease_until=$4,
        attempt_count=attempt_count+1, last_error=NULL
    FROM candidates c WHERE o.id=c.id
    RETURNING o.id, o.topic, o.payload, o.idempotency_key, o.occurred_at,
              o.attempt_count, o.lease_token`, now, w.BatchSize, leaseToken, now.Add(w.Lease))
	if err != nil {
		return nil, fmt.Errorf("claim outbox messages: %w", err)
	}
	defer rows.Close()
	var messages []Message
	for rows.Next() {
		var message Message
		if err := rows.Scan(&message.ID, &message.Topic, &message.Payload, &message.IdempotencyKey, &message.OccurredAt, &message.Attempt, &message.LeaseToken); err != nil {
			return nil, fmt.Errorf("scan outbox message: %w", err)
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (w Worker) complete(ctx context.Context, message Message) error {
	tag, err := w.Pool.Exec(ctx, `UPDATE platform.outbox_messages
        SET status='processed', processed_at=$1, lease_token=NULL, lease_until=NULL
        WHERE id=$2 AND status='processing' AND lease_token=$3`, w.Clock.Now().UTC(), message.ID, message.LeaseToken)
	if err != nil {
		return fmt.Errorf("complete outbox message: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return errors.New("outbox lease lost before completion")
	}
	return nil
}

func (w Worker) retry(ctx context.Context, message Message, handlerErr error) error {
	delay := retryDelay(message.Attempt)
	errorText := handlerErr.Error()
	if len(errorText) > 500 {
		errorText = errorText[:500]
	}
	tag, err := w.Pool.Exec(ctx, `UPDATE platform.outbox_messages
        SET status='pending', available_at=$1, lease_token=NULL, lease_until=NULL, last_error=$2
        WHERE id=$3 AND status='processing' AND lease_token=$4`, w.Clock.Now().UTC().Add(delay), errorText, message.ID, message.LeaseToken)
	if err != nil {
		return fmt.Errorf("release outbox message: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return errors.New("outbox lease lost before retry")
	}
	return nil
}

func retryDelay(attempt int) time.Duration {
	const maximum = 5 * time.Minute
	if attempt <= 1 {
		return time.Second
	}
	delay := time.Second
	for current := 1; current < attempt && delay < maximum; current++ {
		if delay > maximum/2 {
			return maximum
		}
		delay *= 2
	}
	if delay > maximum {
		return maximum
	}
	return delay
}
