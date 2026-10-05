//go:build integration

package payments

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

type fakeSettlement struct {
	reports       int
	confirmations int
	joined        bool
}

func (f *fakeSettlement) ApplyTransferReport(context.Context, string, string, time.Time) error {
	f.reports++
	return nil
}
func (f *fakeSettlement) ConfirmPayment(context.Context, string, string, time.Time) (bool, error) {
	f.confirmations++
	return f.joined, nil
}

func integrationService(t *testing.T, now time.Time) (*Service, *fakeSettlement) {
	t.Helper()
	pool := testdb.Open(t)
	runner, err := platformmigrations.NewRunner(pool, Catalog())
	if err != nil {
		t.Fatal(err)
	}
	if err = runner.Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	settlement := &fakeSettlement{joined: true}
	service, err := NewService(pool, settlement, clock.Fixed{Time: now})
	if err != nil {
		t.Fatal(err)
	}
	return service, settlement
}

func deliverPaymentRequired(t *testing.T, service *Service, event PaymentRequiredEvent) {
	t.Helper()
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.PaymentRequiredHandler()(context.Background(), outbox.Message{ID: event.EventID, Payload: payload}); err != nil {
		t.Fatal(err)
	}
}

func TestTransferReportsReceiptsAndOverpaymentRemainDistinct(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	service, settlement := integrationService(t, now)
	event := PaymentRequiredEvent{EventID: "pay-event-1", ParticipationID: "10000000-0000-4000-8000-000000000001", MatchID: "20000000-0000-4000-8000-000000000001", PayerID: "30000000-0000-4000-8000-000000000001", PayeeID: "40000000-0000-4000-8000-000000000001", FeeMinor: 100_000, DepositMinor: 30_000, Currency: "VND", PaymentRecipient: "Host", PaymentInstructions: "Chuyển khoản", PolicyVersion: "MVP-2026-10-D15", InstructionVersion: 1, HoldExpiresAt: now.Add(30 * time.Minute), MatchStartAt: now.Add(4 * time.Hour), OccurredAt: now}
	deliverPaymentRequired(t, service, event)
	deliverPaymentRequired(t, service, event)
	obligations, err := service.Obligations(context.Background(), event.PayerID, event.ParticipationID)
	if err != nil || len(obligations) != 2 {
		t.Fatalf("obligations=%+v error=%v", obligations, err)
	}
	if _, err = service.Obligations(context.Background(), "50000000-0000-4000-8000-000000000001", event.ParticipationID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("privacy error=%v", err)
	}
	var deposit Obligation
	for _, item := range obligations {
		if item.Purpose == "MATCH_DEPOSIT" {
			deposit = item
		}
	}
	first, err := service.ReportTransfer(context.Background(), ReportTransferRequest{ActorID: event.PayerID, ObligationID: deposit.ID, IdempotencyKey: "report-1", AmountMinor: 15_000, ClaimedAt: now, Reference: "FT-1"})
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := service.Ledger(context.Background(), event.PayeeID, event.ParticipationID)
	if err != nil || len(ledger.Reports) != 1 || ledger.Reports[0].ID != first.ID {
		t.Fatalf("host ledger=%+v error=%v", ledger, err)
	}
	if settlement.reports != 0 {
		t.Fatal("report changed match before durable event delivery")
	}
	reportPayload, _ := json.Marshal(map[string]any{"participationId": event.ParticipationID, "playerId": event.PayerID, "occurredAt": now})
	if err = service.TransferReportedHandler()(context.Background(), outbox.Message{Payload: reportPayload}); err != nil {
		t.Fatal(err)
	}
	if settlement.reports != 1 {
		t.Fatalf("report calls=%d", settlement.reports)
	}
	receipt1, err := service.Acknowledge(context.Background(), AcknowledgeRequest{ActorID: event.PayeeID, ReportID: first.ID, IdempotencyKey: "ack-1", AmountReceivedMinor: 15_000})
	if err != nil {
		t.Fatal(err)
	}
	retry, err := service.Acknowledge(context.Background(), AcknowledgeRequest{ActorID: event.PayeeID, ReportID: first.ID, IdempotencyKey: "ack-1", AmountReceivedMinor: 15_000})
	if err != nil || retry.ID != receipt1.ID {
		t.Fatalf("retry=%+v error=%v", retry, err)
	}
	second, err := service.ReportTransfer(context.Background(), ReportTransferRequest{ActorID: event.PayerID, ObligationID: deposit.ID, IdempotencyKey: "report-2", AmountMinor: 15_000, ClaimedAt: now.Add(time.Minute), Reference: "FT-2"})
	if err != nil {
		t.Fatal(err)
	}
	receipt2, err := service.Acknowledge(context.Background(), AcknowledgeRequest{ActorID: event.PayeeID, ReportID: second.ID, IdempotencyKey: "ack-2", AmountReceivedMinor: 15_000})
	if err != nil || receipt2.ID == receipt1.ID {
		t.Fatalf("second receipt=%+v error=%v", receipt2, err)
	}
	var status string
	if err = service.pool.QueryRow(context.Background(), `SELECT status FROM payments.obligations WHERE id=$1`, deposit.ID).Scan(&status); err != nil || status != "SATISFIED" {
		t.Fatalf("status=%s error=%v", status, err)
	}
	third, err := service.ReportTransfer(context.Background(), ReportTransferRequest{ActorID: event.PayerID, ObligationID: deposit.ID, IdempotencyKey: "report-3", AmountMinor: 10_000, ClaimedAt: now.Add(2 * time.Minute), Reference: "FT-3"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Acknowledge(context.Background(), AcknowledgeRequest{ActorID: event.PayeeID, ReportID: third.ID, IdempotencyKey: "ack-3", AmountReceivedMinor: 10_000}); err != nil {
		t.Fatal(err)
	}
	var overpayment int64
	if err = service.pool.QueryRow(context.Background(), `SELECT amount_minor FROM payments.refunds WHERE source_kind='OVERPAYMENT'`).Scan(&overpayment); err != nil || overpayment != 10_000 {
		t.Fatalf("overpayment=%d error=%v", overpayment, err)
	}
}

func TestLateDepositCreatesRefundAndNeverClaimsJoined(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	service, settlement := integrationService(t, now)
	settlement.joined = false
	event := PaymentRequiredEvent{EventID: "pay-event-late", ParticipationID: "60000000-0000-4000-8000-000000000001", MatchID: "61000000-0000-4000-8000-000000000001", PayerID: "62000000-0000-4000-8000-000000000001", PayeeID: "63000000-0000-4000-8000-000000000001", FeeMinor: 30_000, DepositMinor: 30_000, Currency: "VND", PaymentRecipient: "Host", PaymentInstructions: "Chuyển khoản", PolicyVersion: "MVP-2026-10-D15", InstructionVersion: 1, HoldExpiresAt: now.Add(30 * time.Minute), MatchStartAt: now.Add(time.Hour), OccurredAt: now}
	deliverPaymentRequired(t, service, event)
	obligations, _ := service.Obligations(context.Background(), event.PayerID, event.ParticipationID)
	report, err := service.ReportTransfer(context.Background(), ReportTransferRequest{ActorID: event.PayerID, ObligationID: obligations[0].ID, IdempotencyKey: "late-report", AmountMinor: 30_000, ClaimedAt: now, Reference: "LATE"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Acknowledge(context.Background(), AcknowledgeRequest{ActorID: event.PayeeID, ReportID: report.ID, IdempotencyKey: "late-ack", AmountReceivedMinor: 30_000}); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{"eventId": "deposit-satisfied:" + obligations[0].ID, "participationId": event.ParticipationID, "hostId": event.PayeeID, "occurredAt": now.Add(31 * time.Minute)})
	if err = service.DepositSatisfiedHandler()(context.Background(), outbox.Message{Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if settlement.confirmations != 1 {
		t.Fatalf("confirmations=%d", settlement.confirmations)
	}
	refunds, err := service.Refunds(context.Background(), event.PayerID)
	if err != nil || len(refunds) != 1 || refunds[0].AmountMinor != 30_000 || refunds[0].SourceKind != "LATE_MONEY" {
		t.Fatalf("refunds=%+v error=%v", refunds, err)
	}
}

func TestCancellationRefundRequiresPlayerConfirmation(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	service, _ := integrationService(t, now)
	event := PaymentRequiredEvent{EventID: "pay-event-cancel", ParticipationID: "70000000-0000-4000-8000-000000000001", MatchID: "71000000-0000-4000-8000-000000000001", PayerID: "72000000-0000-4000-8000-000000000001", PayeeID: "73000000-0000-4000-8000-000000000001", FeeMinor: 30_000, DepositMinor: 30_000, Currency: "VND", PaymentRecipient: "Host", PaymentInstructions: "Chuyển khoản", PolicyVersion: "MVP-2026-10-D15", InstructionVersion: 1, HoldExpiresAt: now.Add(30 * time.Minute), MatchStartAt: now.Add(8 * time.Hour), OccurredAt: now}
	deliverPaymentRequired(t, service, event)
	obligations, _ := service.Obligations(context.Background(), event.PayerID, event.ParticipationID)
	report, err := service.ReportTransfer(context.Background(), ReportTransferRequest{ActorID: event.PayerID, ObligationID: obligations[0].ID, IdempotencyKey: "cancel-report", AmountMinor: 30_000, ClaimedAt: now, Reference: "CANCEL"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Acknowledge(context.Background(), AcknowledgeRequest{ActorID: event.PayeeID, ReportID: report.ID, IdempotencyKey: "cancel-ack", AmountReceivedMinor: 30_000}); err != nil {
		t.Fatal(err)
	}
	cancelEvent := CancellationEvent{EventID: "cancel-event-1", MatchID: event.MatchID, ParticipationID: event.ParticipationID, PlayerID: event.PayerID, HostID: event.PayeeID, Cause: "HOST_MATCH_CANCELLATION", Reason: "Mưa lớn", RefundOutcome: "FULL", PolicyVersion: "MVP-2026-10-D15", OccurredAt: now.Add(time.Hour)}
	payload, _ := json.Marshal(cancelEvent)
	handler := service.CancellationHandler()
	if err = handler(context.Background(), outbox.Message{Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if err = handler(context.Background(), outbox.Message{Payload: payload}); err != nil {
		t.Fatal(err)
	}
	refunds, err := service.Refunds(context.Background(), event.PayerID)
	if err != nil || len(refunds) != 1 || refunds[0].AmountMinor != 30_000 || refunds[0].PolicyVersion != "MVP-2026-10-D15" {
		t.Fatalf("refunds=%+v error=%v", refunds, err)
	}
	refund, err := service.ReportRefundSent(context.Background(), ReportRefundRequest{ActorID: event.PayeeID, RefundID: refunds[0].ID, IdempotencyKey: "refund-sent", AmountMinor: 30_000, SentAt: now, Reference: "RF-1"})
	if err != nil || refund.Status != "REPORTED_SENT" {
		t.Fatalf("refund=%+v error=%v", refund, err)
	}
	if refund.Status == "CONFIRMED" {
		t.Fatal("host report incorrectly confirmed refund")
	}
	refund, err = service.ResolveRefund(context.Background(), event.PayerID, refund.ID, "refund-confirm", "CONFIRM", "")
	if err != nil || refund.Status != "CONFIRMED" {
		t.Fatalf("confirmed=%+v error=%v", refund, err)
	}
}
