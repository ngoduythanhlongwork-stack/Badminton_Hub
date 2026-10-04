//go:build integration

package identity

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"badmintonhub/internal/platform/clock"
	"badmintonhub/internal/platform/migrations"
	"badmintonhub/internal/platform/outbox"
	"badmintonhub/internal/platform/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fastHasher struct{}

func (fastHasher) Hash(value string) (string, error)          { return "hash:" + value, nil }
func (fastHasher) Verify(value, encoded string) (bool, error) { return encoded == "hash:"+value, nil }

func TestTokenExpiryRefreshReuseAndResetRevocation(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Open(t)
	runner, err := migrations.NewRunner(pool, Migrations())
	if err != nil {
		t.Fatal(err)
	}
	if err = runner.Up(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	secret := []byte("01234567890123456789012345678901")
	service, err := NewService(Config{Pool: pool, Hasher: fastHasher{}, TokenSecret: secret, Clock: clock.Fixed{Time: now}})
	if err != nil {
		t.Fatal(err)
	}

	expired, err := service.Register(ctx, RegisterRequest{Email: "expired@example.com", Password: "long-enough-password"})
	if err != nil {
		t.Fatal(err)
	}
	expiredToken := deliverLatest(t, pool, secret, "expired@example.com", "VERIFY_EMAIL")
	if _, err = pool.Exec(ctx, `UPDATE identity.action_tokens SET expires_at=$1 WHERE account_id=$2`, now.Add(-time.Second), expired.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.VerifyEmail(ctx, expiredToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expired verify error=%v", err)
	}

	_, err = service.Register(ctx, RegisterRequest{Email: "active@example.com", Password: "long-enough-password"})
	if err != nil {
		t.Fatal(err)
	}
	verifyToken := deliverLatest(t, pool, secret, "active@example.com", "VERIFY_EMAIL")
	if _, err = service.VerifyEmail(ctx, verifyToken); err != nil {
		t.Fatal(err)
	}
	tokens, err := service.Login(ctx, LoginRequest{Email: "active@example.com", Password: "long-enough-password", SourceKey: "test-source"})
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := service.Refresh(ctx, tokens.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Refresh(ctx, tokens.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("reuse error=%v", err)
	}
	if _, err = service.Authenticate(ctx, rotated.AccessToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("family was not revoked: %v", err)
	}

	activeSession, err := service.Login(ctx, LoginRequest{Email: "active@example.com", Password: "long-enough-password", SourceKey: "test-source"})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.ForgotPassword(ctx, "active@example.com"); err != nil {
		t.Fatal(err)
	}
	resetToken := deliverLatest(t, pool, secret, "active@example.com", "RESET_PASSWORD")
	if err = service.ResetPassword(ctx, resetToken, "replacement-password"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Authenticate(ctx, activeSession.AccessToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("reset did not revoke session: %v", err)
	}
	if err = service.ResetPassword(ctx, resetToken, "another-password"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("reset token reuse error=%v", err)
	}
}

func TestConcurrentLoginFailuresStopAtTen(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Open(t)
	runner, err := migrations.NewRunner(pool, Migrations())
	if err != nil {
		t.Fatal(err)
	}
	if err = runner.Up(ctx); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(Config{Pool: pool, Hasher: fastHasher{}, TokenSecret: []byte("01234567890123456789012345678901"), Clock: clock.Fixed{Time: time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)}})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, loginErr := service.Login(ctx, LoginRequest{Email: "missing@example.com", Password: "guess", SourceKey: "same-source"})
			results <- loginErr
		}()
	}
	wg.Wait()
	close(results)
	invalid, limited := 0, 0
	for result := range results {
		if errors.Is(result, ErrInvalidCredentials) {
			invalid++
		} else if errors.Is(result, ErrRateLimited) {
			limited++
		} else {
			t.Fatalf("unexpected error %v", result)
		}
	}
	if invalid != 10 || limited != 10 {
		t.Fatalf("invalid=%d limited=%d", invalid, limited)
	}
	var stored int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM identity.login_failures WHERE canonical_email='missing@example.com' AND source_key='same-source'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 10 {
		t.Fatalf("stored failures=%d", stored)
	}
}

func deliverLatest(t *testing.T, pool *pgxpool.Pool, secret []byte, recipient, kind string) string {
	t.Helper()
	var raw json.RawMessage
	var messageID string
	err := pool.QueryRow(context.Background(), `SELECT id,payload FROM platform.outbox_messages WHERE topic='identity.email' AND payload->>'Recipient'=$1 AND payload->>'Kind'=$2 ORDER BY occurred_at DESC,id DESC LIMIT 1`, recipient, kind).Scan(&messageID, &raw)
	if err != nil {
		t.Fatal(err)
	}
	var persisted map[string]any
	if err = json.Unmarshal(raw, &persisted); err != nil {
		t.Fatal(err)
	}
	if _, exists := persisted["Token"]; exists {
		t.Fatal("raw bearer token persisted in outbox")
	}
	capture := &CaptureEmailSender{}
	handler, err := NewEmailOutboxHandler(capture, secret)
	if err != nil {
		t.Fatal(err)
	}
	if err = handler(context.Background(), outbox.Message{ID: messageID, Topic: "identity.email", Payload: raw}); err != nil {
		t.Fatal(err)
	}
	messages := capture.Messages()
	if len(messages) != 1 || messages[0].Token == "" {
		t.Fatalf("messages=%v", messages)
	}
	return messages[0].Token
}
