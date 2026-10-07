package httpapi

import (
	"net/http"
	"strconv"

	"badmintonhub/internal/modules/identity"
	"badmintonhub/internal/modules/matches"
	"badmintonhub/internal/modules/moderation"
	"badmintonhub/internal/modules/recommendations"
)

func (routes R1Routes) registerR3(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/matches/{matchID}/participants/{participationID}/attendance", routes.auth(http.HandlerFunc(routes.recordAttendance)))
	mux.Handle("POST /api/v1/matches/{matchID}/reviews", routes.auth(http.HandlerFunc(routes.submitReview)))
	mux.Handle("POST /api/v1/admin/attendance/{participationID}/corrections", routes.auth(http.HandlerFunc(routes.correctAttendance)))
	mux.Handle("POST /api/v1/admin/reviews/{reviewID}/corrections", routes.auth(http.HandlerFunc(routes.correctReview)))
	mux.Handle("GET /api/v1/matches/{matchID}/room/messages", routes.auth(http.HandlerFunc(routes.listRoomMessages)))
	mux.Handle("POST /api/v1/matches/{matchID}/room/messages", routes.auth(http.HandlerFunc(routes.sendRoomMessage)))
	mux.Handle("POST /api/v1/moderation/reports", routes.auth(http.HandlerFunc(routes.reportAbuse)))
	mux.Handle("PUT /api/v1/me/blocks/{accountID}", routes.auth(http.HandlerFunc(routes.blockAccount)))
	mux.Handle("DELETE /api/v1/me/blocks/{accountID}", routes.auth(http.HandlerFunc(routes.unblockAccount)))
	mux.Handle("POST /api/v1/admin/moderation/cases/{caseID}/assign", routes.auth(http.HandlerFunc(routes.assignCase)))
	mux.Handle("POST /api/v1/admin/moderation/cases/{caseID}/decide", routes.auth(http.HandlerFunc(routes.decideCase)))
	mux.Handle("POST /api/v1/moderation/cases/{caseID}/appeal", routes.auth(http.HandlerFunc(routes.appealCase)))
	mux.Handle("GET /api/v1/recommendations/matches", routes.auth(http.HandlerFunc(routes.recommendMatches)))
	mux.Handle("GET /api/v1/admin/metrics/attendance", routes.auth(http.HandlerFunc(routes.attendanceMetrics)))
	mux.Handle("GET /api/v1/admin/metrics/funnel", routes.auth(http.HandlerFunc(routes.funnelMetrics)))
	mux.Handle("POST /api/v1/analytics/events", routes.auth(http.HandlerFunc(routes.recordAnalyticsEvent)))
	mux.Handle("GET /api/v1/me/activity", routes.auth(http.HandlerFunc(routes.myActivity)))
	mux.Handle("GET /api/v1/matches/{matchID}/participants", routes.auth(http.HandlerFunc(routes.listMatchParticipants)))
}

func (routes R1Routes) myActivity(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFromContext(r.Context())
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, e := strconv.Atoi(raw)
		if e != nil || value < 1 || value > 100 {
			WriteError(w, r, http.StatusBadRequest, "validation_failed", "Giới hạn phải từ 1 đến 100.")
			return
		}
		limit = value
	}
	items, e := routes.Matches.Activity(r.Context(), p.AccountID, r.URL.Query().Get("view"), limit)
	if e != nil {
		routes.writeDomainError(w, r, e)
		return
	}
	writeJSON(w, map[string]any{"items": items})
}
func (routes R1Routes) listMatchParticipants(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFromContext(r.Context())
	items, e := routes.Matches.Participants(r.Context(), p.AccountID, r.PathValue("matchID"))
	if e != nil {
		routes.writeDomainError(w, r, e)
		return
	}
	writeJSON(w, map[string]any{"items": items})
}

func (routes R1Routes) listRoomMessages(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFromContext(r.Context())
	messages, err := routes.Communication.List(r.Context(), principal.AccountID, r.PathValue("matchID"), 50)
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, map[string]any{"items": messages})
}
func (routes R1Routes) sendRoomMessage(w http.ResponseWriter, r *http.Request) {
	key, ok := idempotencyKey(r)
	if !ok {
		WriteError(w, r, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Thiếu Idempotency-Key.")
		return
	}
	var body struct {
		Body string `json:"body"`
	}
	if !DecodeJSON(w, r, &body) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	message, err := routes.Communication.Send(r.Context(), principal.AccountID, r.PathValue("matchID"), key, body.Body)
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSONStatus(w, http.StatusCreated, message)
}
func (routes R1Routes) reportAbuse(w http.ResponseWriter, r *http.Request) {
	var body moderation.Case
	if !DecodeJSON(w, r, &body) {
		return
	}
	p, _ := PrincipalFromContext(r.Context())
	body.ReporterID = p.AccountID
	c, e := routes.Moderation.Report(r.Context(), body)
	if e != nil {
		routes.writeDomainError(w, r, e)
		return
	}
	writeJSONStatus(w, http.StatusCreated, c)
}
func (routes R1Routes) blockAccount(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFromContext(r.Context())
	if e := routes.Moderation.Block(r.Context(), p.AccountID, r.PathValue("accountID")); e != nil {
		routes.writeDomainError(w, r, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (routes R1Routes) unblockAccount(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFromContext(r.Context())
	if e := routes.Moderation.Unblock(r.Context(), p.AccountID, r.PathValue("accountID")); e != nil {
		routes.writeDomainError(w, r, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (routes R1Routes) assignCase(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFromContext(r.Context())
	if !routes.requireAdmin(w, r, p.AccountID) {
		return
	}
	c, e := routes.Moderation.Assign(r.Context(), r.PathValue("caseID"), p.AccountID)
	if e != nil {
		routes.writeDomainError(w, r, e)
		return
	}
	writeJSON(w, c)
}
func (routes R1Routes) decideCase(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFromContext(r.Context())
	if !routes.requireAdmin(w, r, p.AccountID) {
		return
	}
	var body struct {
		Decision string `json:"decision"`
		Resolved bool   `json:"resolved"`
	}
	if !DecodeJSON(w, r, &body) {
		return
	}
	c, e := routes.Moderation.Decide(r.Context(), r.PathValue("caseID"), p.AccountID, body.Decision, body.Resolved)
	if e != nil {
		routes.writeDomainError(w, r, e)
		return
	}
	writeJSON(w, c)
}
func (routes R1Routes) appealCase(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Reason string `json:"reason"`
	}
	if !DecodeJSON(w, r, &body) {
		return
	}
	p, _ := PrincipalFromContext(r.Context())
	c, e := routes.Moderation.Appeal(r.Context(), r.PathValue("caseID"), p.AccountID, body.Reason)
	if e != nil {
		routes.writeDomainError(w, r, e)
		return
	}
	writeJSON(w, c)
}
func (routes R1Routes) requireAdmin(w http.ResponseWriter, r *http.Request, accountID string) bool {
	ok, e := routes.Identity.HasPermission(r.Context(), identity.PermissionQuery{AccountID: accountID, Permission: identity.PermissionAccountManage, ScopeType: "GLOBAL"})
	if e != nil {
		routes.writeDomainError(w, r, e)
		return false
	}
	if !ok {
		routes.writeDomainError(w, r, matches.ErrForbidden)
		return false
	}
	return true
}
func (routes R1Routes) recommendMatches(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFromContext(r.Context())
	result, e := routes.Recommendations.Recommend(r.Context(), p.AccountID)
	if e != nil {
		routes.writeDomainError(w, r, e)
		return
	}
	writeJSON(w, result)
}
func (routes R1Routes) attendanceMetrics(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFromContext(r.Context())
	if !routes.requireAdmin(w, r, p.AccountID) {
		return
	}
	result, e := routes.Measurement.Attendance(r.Context())
	if e != nil {
		routes.writeDomainError(w, r, e)
		return
	}
	writeJSON(w, map[string]any{"metricVersion": "D24-V1", "attendance": result})
}
func (routes R1Routes) funnelMetrics(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFromContext(r.Context())
	if !routes.requireAdmin(w, r, p.AccountID) {
		return
	}
	result, e := routes.Measurement.Funnel(r.Context())
	if e != nil {
		routes.writeDomainError(w, r, e)
		return
	}
	writeJSON(w, map[string]any{"metricVersion": "D24-V1", "funnel": result})
}
func (routes R1Routes) recordAnalyticsEvent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ClientEventID string `json:"clientEventId"`
		EventType     string `json:"eventType"`
		MatchID       string `json:"matchId"`
		SearchSession string `json:"searchSessionId"`
	}
	if !DecodeJSON(w, r, &body) {
		return
	}
	p, _ := PrincipalFromContext(r.Context())
	e := routes.Measurement.RecordInteraction(r.Context(), recommendations.InteractionEvent{ClientEventID: body.ClientEventID, EventType: body.EventType, ActorID: p.AccountID, MatchID: body.MatchID, SearchSession: body.SearchSession})
	if e != nil {
		routes.writeDomainError(w, r, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (routes R1Routes) recordAttendance(w http.ResponseWriter, r *http.Request) {
	key, ok := idempotencyKey(r)
	if !ok {
		WriteError(w, r, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Thiếu Idempotency-Key.")
		return
	}
	var body struct {
		Status matches.ParticipationStatus `json:"status"`
		Reason string                      `json:"reason"`
	}
	if !DecodeJSON(w, r, &body) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	part, err := routes.Matches.RecordAttendance(r.Context(), principal.AccountID, r.PathValue("matchID"), r.PathValue("participationID"), key, body.Status, body.Reason)
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, participationResponse(part))
}

func (routes R1Routes) submitReview(w http.ResponseWriter, r *http.Request) {
	key, ok := idempotencyKey(r)
	if !ok {
		WriteError(w, r, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Thiếu Idempotency-Key.")
		return
	}
	var body struct {
		TargetPlayerID string   `json:"targetPlayerId"`
		MatchQuality   int      `json:"matchQuality"`
		HostRating     int      `json:"hostRating"`
		Tags           []string `json:"tags"`
		SkillFeedback  string   `json:"skillFeedback"`
	}
	if !DecodeJSON(w, r, &body) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	review, err := routes.Matches.SubmitReview(r.Context(), principal.AccountID, r.PathValue("matchID"), body.TargetPlayerID, key, body.MatchQuality, body.HostRating, body.Tags, body.SkillFeedback)
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSONStatus(w, http.StatusCreated, review)
}

func (routes R1Routes) correctAttendance(w http.ResponseWriter, r *http.Request) {
	key, ok := idempotencyKey(r)
	if !ok {
		WriteError(w, r, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Thiếu Idempotency-Key.")
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	allowed, err := routes.Identity.HasPermission(r.Context(), identity.PermissionQuery{AccountID: principal.AccountID, Permission: identity.PermissionAccountManage, ScopeType: "GLOBAL"})
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	if !allowed {
		routes.writeDomainError(w, r, matches.ErrForbidden)
		return
	}
	var body struct {
		CaseID string                      `json:"caseId"`
		Status matches.ParticipationStatus `json:"status"`
		Reason string                      `json:"reason"`
	}
	if !DecodeJSON(w, r, &body) {
		return
	}
	if err = routes.Moderation.AuthorizeOwnerAction(r.Context(), body.CaseID, principal.AccountID, "ATTENDANCE", r.PathValue("participationID")); err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	part, err := routes.Matches.CorrectAttendance(r.Context(), principal.AccountID, r.PathValue("participationID"), body.CaseID, key, body.Status, body.Reason)
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	if err = routes.Moderation.ResolveOwnerAction(r.Context(), body.CaseID, principal.AccountID, "attendance corrected"); err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, participationResponse(part))
}

func (routes R1Routes) correctReview(w http.ResponseWriter, r *http.Request) {
	key, ok := idempotencyKey(r)
	if !ok {
		WriteError(w, r, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Thiếu Idempotency-Key.")
		return
	}
	p, _ := PrincipalFromContext(r.Context())
	if !routes.requireAdmin(w, r, p.AccountID) {
		return
	}
	var body struct {
		CaseID        string   `json:"caseId"`
		MatchQuality  int      `json:"matchQuality"`
		HostRating    int      `json:"hostRating"`
		Tags          []string `json:"tags"`
		SkillFeedback string   `json:"skillFeedback"`
		Reason        string   `json:"reason"`
	}
	if !DecodeJSON(w, r, &body) {
		return
	}
	reviewID := r.PathValue("reviewID")
	if e := routes.Moderation.AuthorizeOwnerAction(r.Context(), body.CaseID, p.AccountID, "REVIEW", reviewID); e != nil {
		routes.writeDomainError(w, r, e)
		return
	}
	review, e := routes.Matches.CorrectReview(r.Context(), p.AccountID, reviewID, body.CaseID, key, body.MatchQuality, body.HostRating, body.Tags, body.SkillFeedback, body.Reason)
	if e != nil {
		routes.writeDomainError(w, r, e)
		return
	}
	if e = routes.Moderation.ResolveOwnerAction(r.Context(), body.CaseID, p.AccountID, "review corrected"); e != nil {
		routes.writeDomainError(w, r, e)
		return
	}
	writeJSON(w, review)
}
