//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	"badmintonhub/internal/modules/moderation"
	"badmintonhub/internal/modules/players"
	"badmintonhub/internal/modules/recommendations"
	"badmintonhub/internal/modules/venues"
	"badmintonhub/internal/platform/migrations"
	"badmintonhub/internal/platform/outbox"
	"badmintonhub/internal/platform/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestR1FreeMatchmakingJourneyOverHTTP(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Open(t)
	runner, err := migrations.NewRunner(pool, identity.Migrations(), players.Migrations(), venues.Migrations(), matches.Migrations(), moderation.Migrations(), recommendations.Migrations())
	if err != nil {
		t.Fatal(err)
	}
	if err = runner.Up(ctx); err != nil {
		t.Fatal(err)
	}
	secret := "test_identity_token_secret_32_bytes_minimum"
	composition, err := composeR1(pool, config.Config{IdentityTokenSecret: secret}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(httpapi.NewHandlerWithOptions(httpapi.Options{Register: composition.routes.Register, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}))
	defer server.Close()

	admin := registerVerifyLogin(t, server.URL, pool, composition, "admin@example.com")
	host := registerVerifyLogin(t, server.URL, pool, composition, "host@example.com")
	player := registerVerifyLogin(t, server.URL, pool, composition, "player@example.com")
	completeOnboarding(t, server.URL, admin.AccessToken, "Admin", "BEGINNER")
	completeOnboarding(t, server.URL, host.AccessToken, "Host", "INTERMEDIATE")
	completeOnboarding(t, server.URL, player.AccessToken, "Player", "INTERMEDIATE")

	if err = composition.routes.Identity.BootstrapAdmin(ctx, identity.BootstrapAdminRequest{AccountID: admin.AccountID, Rationale: "R1 integration bootstrap"}); err != nil {
		t.Fatal(err)
	}
	application := requestJSON(t, http.MethodPost, server.URL+"/api/v1/organizer-applications", host.AccessToken, "", map[string]any{"reason": "Tổ chức kèo cộng đồng", "contact": "host@example.com"}, http.StatusCreated)
	applicationID := stringField(t, application, "id")
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/admin/organizer-applications/"+applicationID+"/review", admin.AccessToken, "", map[string]any{"decision": "APPROVED", "rationale": "Đủ điều kiện pilot"}, http.StatusNoContent)

	venueBody := map[string]any{"name": "Sân R1", "address": "1 Nguyễn Trãi", "area": "Quận 1", "timeZone": "Asia/Ho_Chi_Minh", "latitude": 10.77, "longitude": 106.69, "openingHours": []string{"06:00-22:00"}, "courts": []string{"Court 1"}}
	venue := requestJSON(t, http.MethodPost, server.URL+"/api/v1/venues", admin.AccessToken, "", venueBody, http.StatusCreated)
	venueID := stringField(t, venue, "id")
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/venues/"+venueID+"/publish", admin.AccessToken, "", nil, http.StatusOK)

	start := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	matchBody := map[string]any{"title": "Kèo R1", "description": "Kèo miễn phí", "format": "DOUBLES", "style": "SOCIAL", "rules": "Đến đúng giờ", "venueId": venueID, "court": "Court 1", "startAt": start, "endAt": start.Add(2 * time.Hour), "minLevel": 2, "maxLevel": 5, "capacity": 4, "feeMinor": 0, "currency": "VND", "joinMode": "INSTANT", "hostPlays": true, "courtAttested": true}
	createdMatch := requestJSON(t, http.MethodPost, server.URL+"/api/v1/matches", host.AccessToken, "", matchBody, http.StatusCreated)
	matchID := stringField(t, createdMatch, "id")
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/matches/"+matchID+"/publish", host.AccessToken, "", nil, http.StatusOK)
	joined := requestJSON(t, http.MethodPost, server.URL+"/api/v1/matches/"+matchID+"/join", player.AccessToken, "join-player-r1", nil, http.StatusCreated)
	if status := stringField(t, joined, "status"); status != "JOINED" {
		t.Fatalf("participation status=%s", status)
	}
}

type testSession struct{ AccountID, AccessToken string }

func registerVerifyLogin(t *testing.T, baseURL string, pool *pgxpool.Pool, composition r1Composition, email string) testSession {
	t.Helper()
	registered := requestJSON(t, http.MethodPost, baseURL+"/api/v1/auth/register", "", "", map[string]any{"email": email, "password": "long-enough-password"}, http.StatusCreated)
	accountID := stringField(t, registered, "id")
	token := deliverLatestIdentityEmail(t, pool, composition, email)
	requestJSON(t, http.MethodPost, baseURL+"/api/v1/auth/verify-email", "", "", map[string]any{"token": token}, http.StatusOK)
	loggedIn := requestJSON(t, http.MethodPost, baseURL+"/api/v1/auth/login", "", "", map[string]any{"email": email, "password": "long-enough-password"}, http.StatusOK)
	return testSession{AccountID: accountID, AccessToken: stringField(t, loggedIn, "accessToken")}
}

func deliverLatestIdentityEmail(t *testing.T, pool *pgxpool.Pool, composition r1Composition, recipient string) string {
	t.Helper()
	var message outbox.Message
	if err := pool.QueryRow(context.Background(), `SELECT id,topic,payload,idempotency_key,occurred_at FROM platform.outbox_messages WHERE topic='identity.email' AND payload->>'Recipient'=$1 ORDER BY occurred_at DESC,id DESC LIMIT 1`, recipient).Scan(&message.ID, &message.Topic, &message.Payload, &message.IdempotencyKey, &message.OccurredAt); err != nil {
		t.Fatal(err)
	}
	if err := composition.handlers[message.Topic](context.Background(), message); err != nil {
		t.Fatal(err)
	}
	for _, delivered := range composition.email.Messages() {
		if delivered.Recipient == recipient {
			return delivered.Token
		}
	}
	t.Fatal("identity email not captured")
	return ""
}

func completeOnboarding(t *testing.T, baseURL, accessToken, displayName, level string) {
	t.Helper()
	body := map[string]any{"displayName": displayName, "dateOfBirth": "2000-01-01", "experience": "1_TO_3_YEARS", "skillLevel": level, "preferredFormats": []string{"DOUBLES"}, "playStyles": []string{"SOCIAL"}, "usualPeriods": []string{"EVENING"}, "regularArea": "Quận 1"}
	requestJSON(t, http.MethodPut, baseURL+"/api/v1/me/onboarding", accessToken, "", body, http.StatusOK)
	requestJSON(t, http.MethodPost, baseURL+"/api/v1/me/onboarding/complete", accessToken, "", nil, http.StatusOK)
}

func requestJSON(t *testing.T, method, url, accessToken, idempotencyKey string, body any, wantStatus int) map[string]any {
	t.Helper()
	var encoded io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		encoded = bytes.NewReader(payload)
	}
	request, err := http.NewRequest(method, url, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != wantStatus {
		t.Fatalf("%s %s status=%d want=%d body=%s", method, url, response.StatusCode, wantStatus, payload)
	}
	if len(payload) == 0 {
		return nil
	}
	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("decode response %s: %v", payload, err)
	}
	return result
}

func stringField(t *testing.T, value map[string]any, field string) string {
	t.Helper()
	result, ok := value[field].(string)
	if !ok || result == "" {
		t.Fatalf("field %s missing from %v", field, value)
	}
	return result
}

func ExampleR1Routes() {
	fmt.Println("onboard -> publish -> discover -> free join")
	// Output: onboard -> publish -> discover -> free join
}
