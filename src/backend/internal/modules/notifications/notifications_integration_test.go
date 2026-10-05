//go:build integration

package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"badmintonhub/internal/platform/clock"
	platformmigrations "badmintonhub/internal/platform/migrations"
	"badmintonhub/internal/platform/outbox"
	"badmintonhub/internal/platform/testdb"
)

type fakeDirectory struct{ email string }

func (f fakeDirectory) EmailForAccount(context.Context, string) (string, error) { return f.email, nil }

type failingSender struct {
	err  error
	sent int
}

func (f *failingSender) Send(context.Context, EmailMessage) error { f.sent++; return f.err }

func notificationIntegrationService(t *testing.T, now time.Time, sender EmailSender) *Service {
	t.Helper()
	pool := testdb.Open(t)
	runner, err := platformmigrations.NewRunner(pool, Catalog())
	if err != nil {
		t.Fatal(err)
	}
	if err = runner.Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(pool, fakeDirectory{email: "player@example.com"}, sender, clock.Fixed{Time: now})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestEventRedeliveryCreatesOneObligationPerChannelAndReadIsIdempotent(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	sender := &CaptureEmailSender{}
	service := notificationIntegrationService(t, now, sender)
	payload, _ := json.Marshal(map[string]any{"eventId": "receipt-1", "kind": "RECEIPT_ACKNOWLEDGED", "recipientId": "10000000-0000-4000-8000-000000000001", "matchId": "20000000-0000-4000-8000-000000000001", "occurredAt": now})
	handler := service.PaymentEventHandler()
	if err := handler(context.Background(), outbox.Message{ID: "message-1", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if err := handler(context.Background(), outbox.Message{ID: "message-1", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := service.pool.QueryRow(context.Background(), `SELECT count(*) FROM notifications.notifications WHERE source_event_id='receipt-1'`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("count=%d error=%v", count, err)
	}
	items, err := service.List(context.Background(), "10000000-0000-4000-8000-000000000001", 20)
	if err != nil || len(items) != 1 {
		t.Fatalf("items=%+v error=%v", items, err)
	}
	first, err := service.MarkRead(context.Background(), items[0].RecipientID, items[0].ID)
	if err != nil || first.ReadAt == nil {
		t.Fatalf("first=%+v error=%v", first, err)
	}
	second, err := service.MarkRead(context.Background(), items[0].RecipientID, items[0].ID)
	if err != nil || !second.ReadAt.Equal(*first.ReadAt) {
		t.Fatalf("second=%+v error=%v", second, err)
	}
}

func TestEmailFailureRetriesThreeTimesWithoutChangingInAppOutcome(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	sender := &failingSender{err: errors.New("mail unavailable")}
	service := notificationIntegrationService(t, now, sender)
	payload, _ := json.Marshal(map[string]any{"eventId": "transfer-1", "kind": "TRANSFER_REPORTED", "recipientId": "30000000-0000-4000-8000-000000000001", "occurredAt": now})
	if err := service.PaymentEventHandler()(context.Background(), outbox.Message{Payload: payload}); err != nil {
		t.Fatal(err)
	}
	var emailID string
	if err := service.pool.QueryRow(context.Background(), `SELECT id FROM notifications.notifications WHERE source_event_id='transfer-1' AND channel='EMAIL'`).Scan(&emailID); err != nil {
		t.Fatal(err)
	}
	emailPayload, _ := json.Marshal(map[string]string{"notificationId": emailID})
	handler := service.EmailDeliveryHandler()
	for attempt := 1; attempt <= 2; attempt++ {
		if err := handler(context.Background(), outbox.Message{Payload: emailPayload, Attempt: attempt}); err == nil {
			t.Fatalf("attempt %d unexpectedly succeeded", attempt)
		}
	}
	if err := handler(context.Background(), outbox.Message{Payload: emailPayload, Attempt: 3}); err != nil {
		t.Fatalf("third attempt should abandon cleanly: %v", err)
	}
	var emailStatus, inAppStatus string
	if err := service.pool.QueryRow(context.Background(), `SELECT delivery_status FROM notifications.notifications WHERE id=$1`, emailID).Scan(&emailStatus); err != nil {
		t.Fatal(err)
	}
	if err := service.pool.QueryRow(context.Background(), `SELECT delivery_status FROM notifications.notifications WHERE source_event_id='transfer-1' AND channel='IN_APP'`).Scan(&inAppStatus); err != nil {
		t.Fatal(err)
	}
	if emailStatus != "ABANDONED" || inAppStatus != "SENT" || sender.sent != 3 {
		t.Fatalf("email=%s in_app=%s sends=%d", emailStatus, inAppStatus, sender.sent)
	}
}

func TestCancellationInvalidatesScheduledReminders(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	service := notificationIntegrationService(t, now, &CaptureEmailSender{})
	joined, _ := json.Marshal(map[string]any{"eventId": "joined-1", "recipientId": "40000000-0000-4000-8000-000000000001", "hostId": "50000000-0000-4000-8000-000000000001", "matchId": "60000000-0000-4000-8000-000000000001", "participationId": "70000000-0000-4000-8000-000000000001", "startAt": now.Add(30 * time.Hour), "occurredAt": now})
	if err := service.JoinedHandler()(context.Background(), outbox.Message{Payload: joined}); err != nil {
		t.Fatal(err)
	}
	cancelled, _ := json.Marshal(map[string]any{"eventId": "cancel-1", "playerId": "40000000-0000-4000-8000-000000000001", "hostId": "50000000-0000-4000-8000-000000000001", "matchId": "60000000-0000-4000-8000-000000000001", "participationId": "70000000-0000-4000-8000-000000000001", "cause": "HOST_MATCH_CANCELLATION", "occurredAt": now.Add(time.Hour)})
	if err := service.CancellationHandler()(context.Background(), outbox.Message{Payload: cancelled}); err != nil {
		t.Fatal(err)
	}
	var pending int
	if err := service.pool.QueryRow(context.Background(), `SELECT count(*) FROM notifications.notifications WHERE purpose LIKE 'MATCH_REMINDER:%' AND delivery_status IN('PENDING','RETRY_PENDING')`).Scan(&pending); err != nil || pending != 0 {
		t.Fatalf("pending=%d error=%v", pending, err)
	}
}

func TestReminderEmailOptOutKeepsInAppReminders(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	service := notificationIntegrationService(t, now, &CaptureEmailSender{})
	recipient := "80000000-0000-4000-8000-000000000001"
	if err := service.SetEmailReminders(context.Background(), recipient, false); err != nil {
		t.Fatal(err)
	}
	joined, _ := json.Marshal(map[string]any{"eventId": "joined-opt-out", "recipientId": recipient, "hostId": "81000000-0000-4000-8000-000000000001", "matchId": "82000000-0000-4000-8000-000000000001", "participationId": "83000000-0000-4000-8000-000000000001", "startAt": now.Add(30 * time.Hour), "occurredAt": now})
	if err := service.JoinedHandler()(context.Background(), outbox.Message{Payload: joined}); err != nil {
		t.Fatal(err)
	}
	var reminderEmail, reminderInApp int
	if err := service.pool.QueryRow(context.Background(), `SELECT count(*) FILTER(WHERE channel='EMAIL'),count(*) FILTER(WHERE channel='IN_APP') FROM notifications.notifications WHERE purpose LIKE 'MATCH_REMINDER:%'`).Scan(&reminderEmail, &reminderInApp); err != nil {
		t.Fatal(err)
	}
	if reminderEmail != 0 || reminderInApp != 2 {
		t.Fatalf("email=%d in_app=%d", reminderEmail, reminderInApp)
	}
}
