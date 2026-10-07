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
	"badmintonhub/internal/modules/communication"
	"badmintonhub/internal/modules/identity"
	"badmintonhub/internal/modules/matches"
	"badmintonhub/internal/modules/moderation"
	"badmintonhub/internal/modules/notifications"
	"badmintonhub/internal/modules/payments"
	"badmintonhub/internal/modules/players"
	"badmintonhub/internal/modules/recommendations"
	"badmintonhub/internal/modules/venues"
	"badmintonhub/internal/platform/migrations"
	"badmintonhub/internal/platform/testdb"
)

func TestR3ClosedLoopJourneyOverHTTP(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Open(t)
	runner, err := migrations.NewRunner(pool, identity.Migrations(), players.Migrations(), venues.Migrations(), matches.Migrations(), payments.Migrations(), notifications.Migrations(), communication.Migrations(), moderation.Migrations(), recommendations.Migrations())
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

	admin := registerVerifyLogin(t, server.URL, pool, composition, "r3-admin@example.com")
	host := registerVerifyLogin(t, server.URL, pool, composition, "r3-host@example.com")
	player := registerVerifyLogin(t, server.URL, pool, composition, "r3-player@example.com")
	unknown := registerVerifyLogin(t, server.URL, pool, composition, "r3-unknown@example.com")
	completeOnboarding(t, server.URL, admin.AccessToken, "R3 Admin", "BEGINNER")
	completeOnboarding(t, server.URL, host.AccessToken, "R3 Host", "INTERMEDIATE")
	completeOnboarding(t, server.URL, player.AccessToken, "R3 Player", "INTERMEDIATE")
	completeOnboarding(t, server.URL, unknown.AccessToken, "R3 Unknown", "INTERMEDIATE")
	if err = composition.routes.Identity.BootstrapAdmin(ctx, identity.BootstrapAdminRequest{AccountID: admin.AccountID, Rationale: "R3 integration bootstrap"}); err != nil {
		t.Fatal(err)
	}
	application := requestJSON(t, http.MethodPost, server.URL+"/api/v1/organizer-applications", host.AccessToken, "", map[string]any{"reason": "R3 host", "contact": "r3-host@example.com"}, http.StatusCreated)
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/admin/organizer-applications/"+stringField(t, application, "id")+"/review", admin.AccessToken, "", map[string]any{"decision": "APPROVED", "rationale": "eligible"}, http.StatusNoContent)
	venue := requestJSON(t, http.MethodPost, server.URL+"/api/v1/venues", admin.AccessToken, "", map[string]any{"name": "San R3", "address": "1 Nguyen Trai", "area": "Quan 1", "timeZone": "Asia/Ho_Chi_Minh", "latitude": 10.77, "longitude": 106.69, "openingHours": []string{"06:00-22:00"}, "courts": []string{"Court 1"}}, http.StatusCreated)
	venueID := stringField(t, venue, "id")
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/venues/"+venueID+"/publish", admin.AccessToken, "", nil, http.StatusOK)

	start := time.Now().UTC().Add(48 * time.Hour)
	matchBody := map[string]any{"title": "Keo R3", "description": "Closed loop", "format": "DOUBLES", "style": "SOCIAL", "rules": "Dung gio", "venueId": venueID, "court": "Court 1", "startAt": start, "endAt": start.Add(2 * time.Hour), "minLevel": 2, "maxLevel": 5, "capacity": 3, "feeMinor": 0, "currency": "VND", "joinMode": "INSTANT", "hostPlays": true, "courtAttested": true}
	created := requestJSON(t, http.MethodPost, server.URL+"/api/v1/matches", host.AccessToken, "", matchBody, http.StatusCreated)
	matchID := stringField(t, created, "id")
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/matches/"+matchID+"/publish", host.AccessToken, "", nil, http.StatusOK)
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/matches/"+matchID+"/join", player.AccessToken, "r3-join-player", nil, http.StatusCreated)
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/matches/"+matchID+"/join", unknown.AccessToken, "r3-join-unknown", nil, http.StatusCreated)
	participants := requestJSON(t, http.MethodGet, server.URL+"/api/v1/matches/"+matchID+"/participants", host.AccessToken, "", nil, http.StatusOK)
	if items, ok := participants["items"].([]any); !ok || len(items) != 3 {
		t.Fatalf("participants=%v", participants)
	}
	requestJSON(t, http.MethodGet, server.URL+"/api/v1/matches/"+matchID+"/participants", player.AccessToken, "", nil, http.StatusForbidden)
	upcoming := requestJSON(t, http.MethodGet, server.URL+"/api/v1/me/activity?view=upcoming", player.AccessToken, "", nil, http.StatusOK)
	if items, ok := upcoming["items"].([]any); !ok || len(items) != 1 {
		t.Fatalf("upcoming=%v", upcoming)
	}
	hosted := requestJSON(t, http.MethodGet, server.URL+"/api/v1/me/activity?view=hosted", host.AccessToken, "", nil, http.StatusOK)
	if items, ok := hosted["items"].([]any); !ok || len(items) != 1 {
		t.Fatalf("hosted=%v", hosted)
	}
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/matches/"+matchID+"/room/messages", player.AccessToken, "r3-message", map[string]any{"body": "Hen moi nguoi tai san."}, http.StatusCreated)

	now := time.Now().UTC()
	if _, err = pool.Exec(ctx, `UPDATE matches.matches SET start_at=$2,end_at=$3,status='IN_PROGRESS' WHERE id=$1`, matchID, now.Add(-2*time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	var hostParticipation, playerParticipation, unknownParticipation string
	rows, err := pool.Query(ctx, `SELECT id::text,player_id::text FROM matches.participations WHERE match_id=$1`, matchID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var pid, account string
		if err = rows.Scan(&pid, &account); err != nil {
			t.Fatal(err)
		}
		switch account {
		case host.AccountID:
			hostParticipation = pid
		case player.AccountID:
			playerParticipation = pid
		case unknown.AccountID:
			unknownParticipation = pid
		}
	}
	rows.Close()
	for i, pid := range []string{hostParticipation, playerParticipation} {
		requestJSON(t, http.MethodPost, server.URL+"/api/v1/matches/"+matchID+"/participants/"+pid+"/attendance", host.AccessToken, "r3-attendance-"+string(rune('a'+i)), map[string]any{"status": "CHECKED_IN"}, http.StatusOK)
	}
	if _, err = pool.Exec(ctx, `UPDATE matches.matches SET start_at=$2,end_at=$3 WHERE id=$1`, matchID, now.Add(-27*time.Hour), now.Add(-25*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if count, completeErr := composition.routes.Matches.CompleteDueMatches(ctx, 100); completeErr != nil || count != 1 {
		t.Fatalf("complete count=%d err=%v", count, completeErr)
	}
	history := requestJSON(t, http.MethodGet, server.URL+"/api/v1/me/activity?view=history", player.AccessToken, "", nil, http.StatusOK)
	if items, ok := history["items"].([]any); !ok || len(items) != 1 {
		t.Fatalf("history=%v", history)
	}
	review := requestJSON(t, http.MethodPost, server.URL+"/api/v1/matches/"+matchID+"/reviews", player.AccessToken, "r3-review", map[string]any{"targetPlayerId": host.AccountID, "matchQuality": 5, "hostRating": 5, "tags": []string{"FRIENDLY", "ON_TIME"}, "skillFeedback": "AS_EXPECTED"}, http.StatusCreated)
	reviewID := stringField(t, review, "id")
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/matches/"+matchID+"/reviews", unknown.AccessToken, "r3-review-denied", map[string]any{"targetPlayerId": host.AccountID, "matchQuality": 5, "hostRating": 5, "tags": []string{"FRIENDLY"}, "skillFeedback": "AS_EXPECTED"}, http.StatusForbidden)
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/matches/"+matchID+"/room/messages", unknown.AccessToken, "r3-stale-message", map[string]any{"body": "stale"}, http.StatusForbidden)

	report := requestJSON(t, http.MethodPost, server.URL+"/api/v1/moderation/reports", unknown.AccessToken, "", map[string]any{"subjectType": "ATTENDANCE", "subjectId": unknownParticipation, "targetActorId": host.AccountID, "reason": "NO_SHOW", "description": "Attendance needs review"}, http.StatusCreated)
	caseID := stringField(t, report, "id")
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/admin/moderation/cases/"+caseID+"/assign", admin.AccessToken, "", nil, http.StatusOK)
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/admin/moderation/cases/"+caseID+"/decide", admin.AccessToken, "", map[string]any{"decision": "Correct attendance from evidence", "resolved": false}, http.StatusOK)
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/admin/attendance/"+unknownParticipation+"/corrections", admin.AccessToken, "r3-correction", map[string]any{"caseId": caseID, "status": "CHECKED_IN", "reason": "Evidence confirmed attendance"}, http.StatusOK)
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/admin/attendance/"+unknownParticipation+"/corrections", admin.AccessToken, "r3-correction", map[string]any{"caseId": caseID, "status": "CHECKED_IN", "reason": "Evidence confirmed attendance"}, http.StatusForbidden)
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/moderation/cases/"+caseID+"/appeal", unknown.AccessToken, "", map[string]any{"reason": "Please review the evidence again"}, http.StatusOK)
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/moderation/cases/"+caseID+"/appeal", unknown.AccessToken, "", map[string]any{"reason": "Duplicate appeal"}, http.StatusConflict)

	reviewReport := requestJSON(t, http.MethodPost, server.URL+"/api/v1/moderation/reports", host.AccessToken, "", map[string]any{"subjectType": "REVIEW", "subjectId": reviewID, "targetActorId": player.AccountID, "reason": "FAKE_SKILL", "description": "Skill feedback needs correction"}, http.StatusCreated)
	reviewCaseID := stringField(t, reviewReport, "id")
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/admin/moderation/cases/"+reviewCaseID+"/assign", admin.AccessToken, "", nil, http.StatusOK)
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/admin/moderation/cases/"+reviewCaseID+"/decide", admin.AccessToken, "", map[string]any{"decision": "Correction supported by evidence", "resolved": false}, http.StatusOK)
	correctedReview := requestJSON(t, http.MethodPost, server.URL+"/api/v1/admin/reviews/"+reviewID+"/corrections", admin.AccessToken, "r3-review-correction", map[string]any{"caseId": reviewCaseID, "matchQuality": 4, "hostRating": 4, "tags": []string{"FAIR", "ON_TIME"}, "skillFeedback": "LOWER_THAN_PROFILE", "reason": "Evidence supports the corrected assessment"}, http.StatusOK)
	if correctedReview["revision"] != float64(2) || correctedReview["skillFeedback"] != "LOWER_THAN_PROFILE" {
		t.Fatalf("corrected review=%v", correctedReview)
	}

	future := time.Now().UTC().Add(72 * time.Hour)
	matchBody["title"] = "Keo goi y"
	matchBody["startAt"] = future
	matchBody["endAt"] = future.Add(2 * time.Hour)
	second := requestJSON(t, http.MethodPost, server.URL+"/api/v1/matches", host.AccessToken, "", matchBody, http.StatusCreated)
	secondID := stringField(t, second, "id")
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/matches/"+secondID+"/publish", host.AccessToken, "", nil, http.StatusOK)
	recommended := requestJSON(t, http.MethodGet, server.URL+"/api/v1/recommendations/matches", player.AccessToken, "", nil, http.StatusOK)
	items, ok := recommended["items"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("recommendations=%v", recommended)
	}
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/analytics/events", player.AccessToken, "", map[string]any{"clientEventId": "r3-search-1", "eventType": "SearchSubmitted", "searchSessionId": "r3-session-1"}, http.StatusNoContent)
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/analytics/events", player.AccessToken, "", map[string]any{"clientEventId": "r3-detail-1", "eventType": "MatchDetailViewed", "matchId": secondID, "searchSessionId": "r3-session-1"}, http.StatusNoContent)
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/analytics/events", player.AccessToken, "", map[string]any{"clientEventId": "r3-detail-1", "eventType": "MatchDetailViewed", "matchId": secondID, "searchSessionId": "r3-session-1"}, http.StatusNoContent)
	if _, err = composition.routes.Measurement.Funnel(ctx); err != nil {
		t.Fatalf("calculate funnel: %v", err)
	}
	funnel := requestJSON(t, http.MethodGet, server.URL+"/api/v1/admin/metrics/funnel", admin.AccessToken, "", nil, http.StatusOK)
	funnelBody, ok := funnel["funnel"].(map[string]any)
	if !ok || funnelBody["searchSessions"] != float64(1) || funnelBody["searchToDetail"] != float64(1) {
		t.Fatalf("funnel=%v", funnel)
	}
	requestJSON(t, http.MethodPut, server.URL+"/api/v1/me/blocks/"+host.AccountID, player.AccessToken, "", nil, http.StatusNoContent)
	recommended = requestJSON(t, http.MethodGet, server.URL+"/api/v1/recommendations/matches", player.AccessToken, "", nil, http.StatusOK)
	items, _ = recommended["items"].([]any)
	if len(items) != 0 {
		t.Fatalf("blocked host remained recommended: %v", recommended)
	}
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/matches/"+secondID+"/join", player.AccessToken, "r3-blocked-join", nil, http.StatusForbidden)
}
