package httpapi

import (
	"net/http"
	"time"

	"badmintonhub/internal/modules/notifications"
	"badmintonhub/internal/modules/payments"
)

func (routes R1Routes) registerR2(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/participations/{participationID}/cancel", routes.auth(http.HandlerFunc(routes.cancelParticipation)))
	mux.Handle("POST /api/v1/matches/{matchID}/cancel", routes.auth(http.HandlerFunc(routes.cancelMatch)))
	mux.Handle("PATCH /api/v1/matches/{matchID}", routes.auth(http.HandlerFunc(routes.updatePublishedMatch)))
	mux.Handle("GET /api/v1/participations/{participationID}/payment-obligations", routes.auth(http.HandlerFunc(routes.paymentObligations)))
	mux.Handle("GET /api/v1/participations/{participationID}/payment-ledger", routes.auth(http.HandlerFunc(routes.paymentLedger)))
	mux.Handle("POST /api/v1/payment-obligations/{obligationID}/transfer-reports", routes.auth(http.HandlerFunc(routes.reportTransfer)))
	mux.Handle("POST /api/v1/transfer-reports/{reportID}/acknowledge", routes.auth(http.HandlerFunc(routes.acknowledgeTransfer)))
	mux.Handle("POST /api/v1/transfer-reports/{reportID}/not-found", routes.auth(http.HandlerFunc(routes.transferNotFound)))
	mux.Handle("GET /api/v1/me/refunds", routes.auth(http.HandlerFunc(routes.listRefunds)))
	mux.Handle("POST /api/v1/refunds/{refundID}/report-sent", routes.auth(http.HandlerFunc(routes.reportRefundSent)))
	mux.Handle("POST /api/v1/refunds/{refundID}/confirm", routes.auth(http.HandlerFunc(routes.confirmRefund)))
	mux.Handle("POST /api/v1/refunds/{refundID}/dispute", routes.auth(http.HandlerFunc(routes.disputeRefund)))
	mux.Handle("GET /api/v1/me/notifications", routes.auth(http.HandlerFunc(routes.listNotifications)))
	mux.Handle("POST /api/v1/notifications/{notificationID}/read", routes.auth(http.HandlerFunc(routes.readNotification)))
	mux.Handle("PUT /api/v1/me/notification-preferences", routes.auth(http.HandlerFunc(routes.updateNotificationPreferences)))
}

func (routes R1Routes) paymentLedger(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFromContext(r.Context())
	ledger, err := routes.Payments.Ledger(r.Context(), principal.AccountID, r.PathValue("participationID"))
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	obligations := make([]map[string]any, 0, len(ledger.Obligations))
	for _, item := range ledger.Obligations {
		obligations = append(obligations, obligationResponse(item))
	}
	reports := make([]map[string]any, 0, len(ledger.Reports))
	for _, item := range ledger.Reports {
		reports = append(reports, transferReportResponse(item))
	}
	receipts := make([]map[string]any, 0, len(ledger.Receipts))
	for _, item := range ledger.Receipts {
		receipts = append(receipts, receiptResponse(item))
	}
	refunds := make([]map[string]any, 0, len(ledger.Refunds))
	for _, item := range ledger.Refunds {
		refunds = append(refunds, refundResponse(item))
	}
	writeJSON(w, map[string]any{"obligations": obligations, "transferReports": reports, "receipts": receipts, "refunds": refunds})
}

func (routes R1Routes) updateNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	var body struct {
		EmailReminders bool `json:"emailReminders"`
	}
	if !DecodeJSON(w, r, &body) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	if err := routes.Notifications.SetEmailReminders(r.Context(), principal.AccountID, body.EmailReminders); err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, map[string]any{"emailReminders": body.EmailReminders})
}

func (routes R1Routes) updatePublishedMatch(w http.ResponseWriter, r *http.Request) {
	key, ok := idempotencyKey(r)
	if !ok {
		WriteError(w, r, http.StatusBadRequest, "idempotency_key_required", "A valid Idempotency-Key header is required.")
		return
	}
	var body struct {
		Description string `json:"description"`
		Capacity    int    `json:"capacity"`
	}
	if !DecodeJSON(w, r, &body) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	item, err := routes.Matches.UpdatePublished(r.Context(), principal.AccountID, r.PathValue("matchID"), key, body.Description, body.Capacity)
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, matchResponse(item))
}

func (routes R1Routes) cancelParticipation(w http.ResponseWriter, r *http.Request) {
	key, ok := idempotencyKey(r)
	if !ok {
		WriteError(w, r, http.StatusBadRequest, "idempotency_key_required", "A valid Idempotency-Key header is required.")
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if !DecodeJSON(w, r, &body) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	item, err := routes.Matches.CancelParticipation(r.Context(), principal.AccountID, r.PathValue("participationID"), key, body.Reason)
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, participationResponse(item))
}

func (routes R1Routes) cancelMatch(w http.ResponseWriter, r *http.Request) {
	key, ok := idempotencyKey(r)
	if !ok {
		WriteError(w, r, http.StatusBadRequest, "idempotency_key_required", "A valid Idempotency-Key header is required.")
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if !DecodeJSON(w, r, &body) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	item, err := routes.Matches.CancelMatch(r.Context(), principal.AccountID, r.PathValue("matchID"), key, body.Reason)
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, matchResponse(item))
}

func (routes R1Routes) paymentObligations(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFromContext(r.Context())
	items, err := routes.Payments.Obligations(r.Context(), principal.AccountID, r.PathValue("participationID"))
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	response := make([]map[string]any, 0, len(items))
	for _, item := range items {
		response = append(response, obligationResponse(item))
	}
	writeJSON(w, map[string]any{"items": response})
}

func (routes R1Routes) reportTransfer(w http.ResponseWriter, r *http.Request) {
	key, ok := idempotencyKey(r)
	if !ok {
		WriteError(w, r, http.StatusBadRequest, "idempotency_key_required", "A valid Idempotency-Key header is required.")
		return
	}
	var body struct {
		AmountMinor int64     `json:"amountMinor"`
		ClaimedAt   time.Time `json:"claimedAt"`
		Reference   string    `json:"reference"`
		EvidenceRef string    `json:"evidenceRef"`
	}
	if !DecodeJSON(w, r, &body) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	item, err := routes.Payments.ReportTransfer(r.Context(), payments.ReportTransferRequest{ActorID: principal.AccountID, ObligationID: r.PathValue("obligationID"), IdempotencyKey: key, AmountMinor: body.AmountMinor, ClaimedAt: body.ClaimedAt, Reference: body.Reference, EvidenceRef: body.EvidenceRef})
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSONStatus(w, http.StatusCreated, transferReportResponse(item))
}

func (routes R1Routes) acknowledgeTransfer(w http.ResponseWriter, r *http.Request) {
	key, ok := idempotencyKey(r)
	if !ok {
		WriteError(w, r, http.StatusBadRequest, "idempotency_key_required", "A valid Idempotency-Key header is required.")
		return
	}
	var body struct {
		AmountReceivedMinor int64 `json:"amountReceivedMinor"`
	}
	if !DecodeJSON(w, r, &body) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	item, err := routes.Payments.Acknowledge(r.Context(), payments.AcknowledgeRequest{ActorID: principal.AccountID, ReportID: r.PathValue("reportID"), IdempotencyKey: key, AmountReceivedMinor: body.AmountReceivedMinor})
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, receiptResponse(item))
}

func (routes R1Routes) transferNotFound(w http.ResponseWriter, r *http.Request) {
	key, ok := idempotencyKey(r)
	if !ok {
		WriteError(w, r, http.StatusBadRequest, "idempotency_key_required", "A valid Idempotency-Key header is required.")
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if !DecodeJSON(w, r, &body) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	item, err := routes.Payments.MarkNotFound(r.Context(), principal.AccountID, r.PathValue("reportID"), key, body.Reason)
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, transferReportResponse(item))
}

func (routes R1Routes) listRefunds(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFromContext(r.Context())
	items, err := routes.Payments.Refunds(r.Context(), principal.AccountID)
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	response := make([]map[string]any, 0, len(items))
	for _, item := range items {
		response = append(response, refundResponse(item))
	}
	writeJSON(w, map[string]any{"items": response})
}

func (routes R1Routes) reportRefundSent(w http.ResponseWriter, r *http.Request) {
	key, ok := idempotencyKey(r)
	if !ok {
		WriteError(w, r, http.StatusBadRequest, "idempotency_key_required", "A valid Idempotency-Key header is required.")
		return
	}
	var body struct {
		AmountMinor int64     `json:"amountMinor"`
		SentAt      time.Time `json:"sentAt"`
		Reference   string    `json:"reference"`
		EvidenceRef string    `json:"evidenceRef"`
	}
	if !DecodeJSON(w, r, &body) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	item, err := routes.Payments.ReportRefundSent(r.Context(), payments.ReportRefundRequest{ActorID: principal.AccountID, RefundID: r.PathValue("refundID"), IdempotencyKey: key, AmountMinor: body.AmountMinor, SentAt: body.SentAt, Reference: body.Reference, EvidenceRef: body.EvidenceRef})
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, refundResponse(item))
}

func (routes R1Routes) confirmRefund(w http.ResponseWriter, r *http.Request) {
	routes.resolveRefund(w, r, "CONFIRM")
}
func (routes R1Routes) disputeRefund(w http.ResponseWriter, r *http.Request) {
	routes.resolveRefund(w, r, "DISPUTE")
}
func (routes R1Routes) resolveRefund(w http.ResponseWriter, r *http.Request, decision string) {
	key, ok := idempotencyKey(r)
	if !ok {
		WriteError(w, r, http.StatusBadRequest, "idempotency_key_required", "A valid Idempotency-Key header is required.")
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if decision == "DISPUTE" && !DecodeJSON(w, r, &body) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	item, err := routes.Payments.ResolveRefund(r.Context(), principal.AccountID, r.PathValue("refundID"), key, decision, body.Reason)
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, refundResponse(item))
}

func (routes R1Routes) listNotifications(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFromContext(r.Context())
	limit, ok := queryInt(r, "limit", 20)
	if !ok {
		WriteError(w, r, http.StatusBadRequest, "invalid_filter", "A query filter is invalid.")
		return
	}
	items, err := routes.Notifications.List(r.Context(), principal.AccountID, limit)
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	response := make([]map[string]any, 0, len(items))
	for _, item := range items {
		response = append(response, notificationResponse(item))
	}
	writeJSON(w, map[string]any{"items": response})
}

func (routes R1Routes) readNotification(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFromContext(r.Context())
	item, err := routes.Notifications.MarkRead(r.Context(), principal.AccountID, r.PathValue("notificationID"))
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, notificationResponse(item))
}

func obligationResponse(item payments.Obligation) map[string]any {
	return map[string]any{"id": item.ID, "participationId": item.ParticipationID, "matchId": item.MatchID, "purpose": item.Purpose, "amountMinor": item.AmountMinor, "currency": item.Currency, "status": item.Status, "dueAt": item.DueAt, "paymentInstruction": map[string]any{"recipient": item.InstructionRecipient, "instructions": item.InstructionText, "version": item.InstructionVersion}, "policyVersion": item.PolicyVersion}
}
func transferReportResponse(item payments.TransferReport) map[string]any {
	return map[string]any{"id": item.ID, "obligationId": item.ObligationID, "amountMinor": item.AmountMinor, "claimedAt": item.ClaimedAt, "reference": item.Reference, "evidenceRef": item.EvidenceRef, "status": item.Status, "resolutionReason": item.ResolutionReason, "createdAt": item.CreatedAt}
}
func receiptResponse(item payments.Receipt) map[string]any {
	return map[string]any{"id": item.ID, "obligationId": item.ObligationID, "transferReportId": item.TransferReportID, "amountMinor": item.AmountMinor, "state": item.State, "revision": item.Revision, "recordedAt": item.RecordedAt}
}
func refundResponse(item payments.Refund) map[string]any {
	return map[string]any{"id": item.ID, "participationId": item.ParticipationID, "matchId": item.MatchID, "amountMinor": item.AmountMinor, "currency": item.Currency, "sourceKind": item.SourceKind, "policyVersion": item.PolicyVersion, "status": item.Status, "dueAt": item.DueAt, "sentAmountMinor": item.SentAmountMinor, "sentAt": item.SentAt, "sentReference": item.SentReference, "evidenceRef": item.EvidenceRef, "disputeReason": item.DisputeReason, "updatedAt": item.UpdatedAt}
}
func notificationResponse(item notifications.Notification) map[string]any {
	return map[string]any{"id": item.ID, "purpose": item.Purpose, "title": item.Title, "body": item.Body, "actionPath": item.ActionPath, "createdAt": item.CreatedAt, "readAt": item.ReadAt}
}
