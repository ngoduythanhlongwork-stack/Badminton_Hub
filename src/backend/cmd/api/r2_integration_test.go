//go:build integration

package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"badmintonhub/internal/config"
	"badmintonhub/internal/httpapi"
	"badmintonhub/internal/modules/identity"
	"badmintonhub/internal/modules/matches"
	"badmintonhub/internal/modules/notifications"
	"badmintonhub/internal/modules/payments"
	"badmintonhub/internal/modules/players"
	"badmintonhub/internal/modules/venues"
	"badmintonhub/internal/platform/migrations"
	"badmintonhub/internal/platform/outbox"
	"badmintonhub/internal/platform/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestR2PaidCancellationJourneyOverHTTP(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Open(t)
	runner, err := migrations.NewRunner(pool, identity.Migrations(), players.Migrations(), venues.Migrations(), matches.Migrations(), payments.Migrations(), notifications.Migrations())
	if err != nil {
		t.Fatal(err)
	}
	if err = runner.Up(ctx); err != nil {
		t.Fatal(err)
	}
	composition, err := composeR1(pool, config.Config{IdentityTokenSecret: "test_identity_token_secret_32_bytes_minimum"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(httpapi.NewHandlerWithOptions(httpapi.Options{Register: composition.routes.Register, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}))
	defer server.Close()

	admin := registerVerifyLogin(t, server.URL, pool, composition, "r2-admin@example.com")
	host := registerVerifyLogin(t, server.URL, pool, composition, "r2-host@example.com")
	player := registerVerifyLogin(t, server.URL, pool, composition, "r2-player@example.com")
	completeOnboarding(t, server.URL, admin.AccessToken, "R2 Admin", "BEGINNER")
	completeOnboarding(t, server.URL, host.AccessToken, "R2 Host", "INTERMEDIATE")
	completeOnboarding(t, server.URL, player.AccessToken, "R2 Player", "INTERMEDIATE")
	if err = composition.routes.Identity.BootstrapAdmin(ctx, identity.BootstrapAdminRequest{AccountID: admin.AccountID, Rationale: "R2 integration bootstrap"}); err != nil {
		t.Fatal(err)
	}
	application := requestJSON(t, http.MethodPost, server.URL+"/api/v1/organizer-applications", host.AccessToken, "", map[string]any{"reason": "Tổ chức kèo có cọc", "contact": "r2-host@example.com"}, http.StatusCreated)
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/admin/organizer-applications/"+stringField(t, application, "id")+"/review", admin.AccessToken, "", map[string]any{"decision": "APPROVED", "rationale": "Đủ điều kiện"}, http.StatusNoContent)
	venue := requestJSON(t, http.MethodPost, server.URL+"/api/v1/venues", admin.AccessToken, "", map[string]any{"name": "Sân R2", "address": "2 Nguyễn Trãi", "area": "Quận 1", "timeZone": "Asia/Ho_Chi_Minh", "latitude": 10.77, "longitude": 106.69, "openingHours": []string{"06:00-22:00"}, "courts": []string{"Court 2"}}, http.StatusCreated)
	venueID := stringField(t, venue, "id")
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/venues/"+venueID+"/publish", admin.AccessToken, "", nil, http.StatusOK)

	start := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	created := requestJSON(t, http.MethodPost, server.URL+"/api/v1/matches", host.AccessToken, "", map[string]any{"title": "Kèo R2", "description": "Kèo có cọc", "format": "DOUBLES", "style": "SOCIAL", "rules": "Đến đúng giờ", "venueId": venueID, "court": "Court 2", "startAt": start, "endAt": start.Add(2 * time.Hour), "minLevel": 2, "maxLevel": 5, "capacity": 4, "feeMinor": 100000, "depositMinor": 30000, "currency": "VND", "paymentRecipient": "Host R2", "paymentInstructions": "Chuyển khoản với mã lượt", "paymentInstructionVersion": 1, "joinMode": "INSTANT", "hostPlays": false, "courtAttested": true}, http.StatusCreated)
	matchID := stringField(t, created, "id")
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/matches/"+matchID+"/publish", host.AccessToken, "", nil, http.StatusOK)
	joined := requestJSON(t, http.MethodPost, server.URL+"/api/v1/matches/"+matchID+"/join", player.AccessToken, "r2-paid-join", nil, http.StatusCreated)
	if status := stringField(t, joined, "status"); status != "AWAITING_PAYMENT" {
		t.Fatalf("status=%s", status)
	}
	participationID := stringField(t, joined, "id")
	deliverLatestTopic(t, pool, composition, "matches.payment-required")
	obligations := requestJSON(t, http.MethodGet, server.URL+"/api/v1/participations/"+participationID+"/payment-obligations", player.AccessToken, "", nil, http.StatusOK)
	items := obligations["items"].([]any)
	var depositID string
	for _, raw := range items {
		item := raw.(map[string]any)
		if item["purpose"] == "MATCH_DEPOSIT" {
			depositID = item["id"].(string)
		}
	}
	if depositID == "" {
		t.Fatal("deposit obligation missing")
	}
	report := requestJSON(t, http.MethodPost, server.URL+"/api/v1/payment-obligations/"+depositID+"/transfer-reports", player.AccessToken, "r2-report", map[string]any{"amountMinor": 30000, "claimedAt": time.Now().UTC(), "reference": "R2-FT-001"}, http.StatusCreated)
	deliverLatestTopic(t, pool, composition, "payments.transfer-reported")
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/transfer-reports/"+stringField(t, report, "id")+"/acknowledge", host.AccessToken, "r2-ack", map[string]any{"amountReceivedMinor": 30000}, http.StatusOK)
	deliverLatestTopic(t, pool, composition, "payments.deposit-satisfied")
	var participationStatus string
	if err = pool.QueryRow(ctx, `SELECT status FROM matches.participations WHERE id=$1`, participationID).Scan(&participationStatus); err != nil || participationStatus != "JOINED" {
		t.Fatalf("participation=%s error=%v", participationStatus, err)
	}
	deliverLatestTopic(t, pool, composition, "matches.joined")
	notificationList := requestJSON(t, http.MethodGet, server.URL+"/api/v1/me/notifications", player.AccessToken, "", nil, http.StatusOK)
	if len(notificationList["items"].([]any)) == 0 {
		t.Fatal("join notification missing")
	}
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/matches/"+matchID+"/cancel", host.AccessToken, "r2-cancel", map[string]any{"reason": "Sân đóng cửa đột xuất"}, http.StatusOK)
	deliverLatestTopic(t, pool, composition, "matches.cancelled")
	refundList := requestJSON(t, http.MethodGet, server.URL+"/api/v1/me/refunds", player.AccessToken, "", nil, http.StatusOK)
	refundItems := refundList["items"].([]any)
	if len(refundItems) != 1 || refundItems[0].(map[string]any)["status"] != "DUE" {
		t.Fatalf("refunds=%v", refundItems)
	}
}

func deliverLatestTopic(t *testing.T, pool *pgxpool.Pool, composition r1Composition, topic string) {
	t.Helper()
	var message outbox.Message
	if err := pool.QueryRow(context.Background(), `SELECT id,topic,payload,idempotency_key,occurred_at FROM platform.outbox_messages WHERE topic=$1 ORDER BY occurred_at DESC,id DESC LIMIT 1`, topic).Scan(&message.ID, &message.Topic, &message.Payload, &message.IdempotencyKey, &message.OccurredAt); err != nil {
		t.Fatal(err)
	}
	handler := composition.handlers[topic]
	if handler == nil {
		t.Fatalf("handler missing for %s", topic)
	}
	if err := handler(context.Background(), message); err != nil {
		t.Fatalf("deliver %s: %v", topic, err)
	}
}
