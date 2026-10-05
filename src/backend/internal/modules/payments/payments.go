package payments

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"badmintonhub/internal/platform/clock"
	"badmintonhub/internal/platform/id"
	"badmintonhub/internal/platform/outbox"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalid             = errors.New("payments: invalid input")
	ErrForbidden           = errors.New("payments: forbidden")
	ErrNotFound            = errors.New("payments: not found")
	ErrConflict            = errors.New("payments: conflict")
	ErrIdempotencyConflict = errors.New("payments: idempotency conflict")
	ErrRefundLimit         = errors.New("payments: refund exceeds acknowledged receipts")
)

type Obligation struct {
	ID, ParticipationID, MatchID, PayerID, PayeeID string
	Purpose, Currency, Status                      string
	AmountMinor                                    int64
	DueAt                                          time.Time
	InstructionRecipient, InstructionText          string
	InstructionVersion                             int
	PolicyVersion                                  string
	CreatedAt, UpdatedAt                           time.Time
}

type TransferReport struct {
	ID, ObligationID, ReporterID, Reference, EvidenceRef string
	AmountMinor                                          int64
	ClaimedAt                                            time.Time
	Status, ResolutionReason                             string
	CreatedAt, UpdatedAt                                 time.Time
}

type Receipt struct {
	ID, ObligationID, TransferReportID, AcknowledgedBy string
	AmountMinor                                        int64
	State, Source                                      string
	Revision                                           int
	RecordedAt                                         time.Time
}

type Refund struct {
	ID, ParticipationID, MatchID, PayerID, PayeeID string
	SourceKind, PolicyVersion, Currency, Status    string
	AmountMinor                                    int64
	DueAt                                          time.Time
	SentAmountMinor                                *int64
	SentAt                                         *time.Time
	SentReference, EvidenceRef, DisputeReason      string
	CreatedAt, UpdatedAt                           time.Time
}

type PaymentRequiredEvent struct {
	EventID, ParticipationID, MatchID, PayerID, PayeeID string
	FeeMinor, DepositMinor                              int64
	Currency, PaymentRecipient, PaymentInstructions     string
	PolicyVersion                                       string
	InstructionVersion                                  int
	HoldExpiresAt, MatchStartAt, OccurredAt             time.Time
}

type CancellationEvent struct {
	EventID, MatchID, ParticipationID, PlayerID, HostID string
	Cause, Reason, RefundOutcome, PolicyVersion         string
	OccurredAt                                          time.Time
}

type MatchSettlement interface {
	ApplyTransferReport(context.Context, string, string, time.Time) error
	ConfirmPayment(context.Context, string, string, time.Time) (bool, error)
}

type Service struct {
	pool       *pgxpool.Pool
	clock      clock.Clock
	settlement MatchSettlement
}

func NewService(pool *pgxpool.Pool, settlement MatchSettlement, c clock.Clock) (*Service, error) {
	if pool == nil {
		return nil, errors.New("payments pool is required")
	}
	if c == nil {
		c = clock.System{}
	}
	return &Service{pool: pool, clock: c, settlement: settlement}, nil
}

func (s *Service) Obligations(ctx context.Context, actor, participationID string) ([]Obligation, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,participation_id,match_id,payer_id,payee_id,purpose,amount_minor,currency,status,due_at,instruction_recipient,instruction_text,instruction_version,policy_version,created_at,updated_at FROM payments.obligations WHERE participation_id=$1 AND (payer_id=$2 OR payee_id=$2) ORDER BY created_at,purpose`, participationID, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Obligation, 0)
	for rows.Next() {
		var item Obligation
		if err = scanObligation(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, ErrNotFound
	}
	return items, nil
}

type ReportTransferRequest struct {
	ActorID, ObligationID, IdempotencyKey, Reference, EvidenceRef string
	AmountMinor                                                   int64
	ClaimedAt                                                     time.Time
}

func (s *Service) ReportTransfer(ctx context.Context, request ReportTransferRequest) (TransferReport, error) {
	request.Reference = strings.TrimSpace(request.Reference)
	request.EvidenceRef = strings.TrimSpace(request.EvidenceRef)
	if request.ActorID == "" || request.ObligationID == "" || request.IdempotencyKey == "" || request.AmountMinor <= 0 || request.Reference == "" || request.ClaimedAt.IsZero() {
		return TransferReport{}, ErrInvalid
	}
	now := s.clock.Now().UTC()
	if request.ClaimedAt.After(now.Add(5 * time.Minute)) {
		return TransferReport{}, ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TransferReport{}, err
	}
	defer tx.Rollback(context.Background())
	const action = "report-transfer"
	if err = lockCommand(ctx, tx, request.ActorID, action, request.IdempotencyKey); err != nil {
		return TransferReport{}, err
	}
	fingerprint := fmt.Sprintf("%s:%d:%s:%s", request.ObligationID, request.AmountMinor, request.ClaimedAt.UTC().Format(time.RFC3339Nano), request.Reference)
	if report, found, loadErr := loadIdempotentReport(ctx, tx, request.ActorID, action, request.IdempotencyKey, fingerprint); loadErr != nil {
		return TransferReport{}, loadErr
	} else if found {
		return report, tx.Commit(ctx)
	}
	var obligation Obligation
	if err = scanObligation(tx.QueryRow(ctx, `SELECT id,participation_id,match_id,payer_id,payee_id,purpose,amount_minor,currency,status,due_at,instruction_recipient,instruction_text,instruction_version,policy_version,created_at,updated_at FROM payments.obligations WHERE id=$1 FOR UPDATE`, request.ObligationID), &obligation); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TransferReport{}, ErrNotFound
		}
		return TransferReport{}, err
	}
	if obligation.PayerID != request.ActorID {
		return TransferReport{}, ErrForbidden
	}
	if obligation.Status == "CANCELLED" {
		return TransferReport{}, ErrConflict
	}
	reportID, err := id.New()
	if err != nil {
		return TransferReport{}, err
	}
	report := TransferReport{ID: reportID, ObligationID: obligation.ID, ReporterID: request.ActorID, AmountMinor: request.AmountMinor, ClaimedAt: request.ClaimedAt.UTC(), Reference: request.Reference, EvidenceRef: request.EvidenceRef, Status: "SUBMITTED", CreatedAt: now, UpdatedAt: now}
	_, err = tx.Exec(ctx, `INSERT INTO payments.transfer_reports(id,obligation_id,reporter_id,amount_minor,claimed_at,reference,evidence_ref,status,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,'SUBMITTED',$8,$8)`, report.ID, report.ObligationID, report.ReporterID, report.AmountMinor, report.ClaimedAt, report.Reference, report.EvidenceRef, now)
	if err != nil {
		return TransferReport{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO payments.idempotency(actor_id,action,idempotency_key,fingerprint,resource_type,resource_id,created_at) VALUES($1,$2,$3,$4,'transfer_report',$5,$6)`, request.ActorID, action, request.IdempotencyKey, fingerprint, report.ID, now); err != nil {
		return TransferReport{}, err
	}
	event := map[string]any{"eventId": "transfer-reported:" + report.ID, "participationId": obligation.ParticipationID, "playerId": obligation.PayerID, "hostId": obligation.PayeeID, "matchId": obligation.MatchID, "reportId": report.ID, "occurredAt": now}
	if _, err = outbox.Enqueue(ctx, tx, "payments.transfer-reported", "transfer-reported:"+report.ID, event, now); err != nil {
		return TransferReport{}, err
	}
	if _, err = outbox.Enqueue(ctx, tx, "payments.notification", "payment-notification:transfer-reported:"+report.ID, map[string]any{"eventId": "transfer-reported:" + report.ID, "kind": "TRANSFER_REPORTED", "recipientId": obligation.PayeeID, "matchId": obligation.MatchID, "participationId": obligation.ParticipationID, "occurredAt": now}, now); err != nil {
		return TransferReport{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return TransferReport{}, err
	}
	return report, nil
}

type AcknowledgeRequest struct {
	ActorID, ReportID, IdempotencyKey string
	AmountReceivedMinor               int64
}

func (s *Service) Acknowledge(ctx context.Context, request AcknowledgeRequest) (Receipt, error) {
	if request.ActorID == "" || request.ReportID == "" || request.IdempotencyKey == "" || request.AmountReceivedMinor <= 0 {
		return Receipt{}, ErrInvalid
	}
	now := s.clock.Now().UTC()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Receipt{}, err
	}
	defer tx.Rollback(context.Background())
	const action = "acknowledge-transfer"
	if err = lockCommand(ctx, tx, request.ActorID, action, request.IdempotencyKey); err != nil {
		return Receipt{}, err
	}
	fingerprint := fmt.Sprintf("%s:%d", request.ReportID, request.AmountReceivedMinor)
	if receipt, found, loadErr := loadIdempotentReceipt(ctx, tx, request.ActorID, action, request.IdempotencyKey, fingerprint); loadErr != nil {
		return Receipt{}, loadErr
	} else if found {
		return receipt, tx.Commit(ctx)
	}
	var report TransferReport
	var obligation Obligation
	err = tx.QueryRow(ctx, `SELECT r.id,r.obligation_id,r.reporter_id,r.amount_minor,r.claimed_at,r.reference,r.evidence_ref,r.status,r.resolution_reason,r.created_at,r.updated_at,o.id,o.participation_id,o.match_id,o.payer_id,o.payee_id,o.purpose,o.amount_minor,o.currency,o.status,o.due_at,o.instruction_recipient,o.instruction_text,o.instruction_version,o.policy_version,o.created_at,o.updated_at FROM payments.transfer_reports r JOIN payments.obligations o ON o.id=r.obligation_id WHERE r.id=$1 FOR UPDATE OF r,o`, request.ReportID).Scan(&report.ID, &report.ObligationID, &report.ReporterID, &report.AmountMinor, &report.ClaimedAt, &report.Reference, &report.EvidenceRef, &report.Status, &report.ResolutionReason, &report.CreatedAt, &report.UpdatedAt, &obligation.ID, &obligation.ParticipationID, &obligation.MatchID, &obligation.PayerID, &obligation.PayeeID, &obligation.Purpose, &obligation.AmountMinor, &obligation.Currency, &obligation.Status, &obligation.DueAt, &obligation.InstructionRecipient, &obligation.InstructionText, &obligation.InstructionVersion, &obligation.PolicyVersion, &obligation.CreatedAt, &obligation.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Receipt{}, ErrNotFound
	}
	if err != nil {
		return Receipt{}, err
	}
	if obligation.PayeeID != request.ActorID {
		return Receipt{}, ErrForbidden
	}
	if report.Status != "SUBMITTED" {
		return Receipt{}, ErrConflict
	}
	receiptID, err := id.New()
	if err != nil {
		return Receipt{}, err
	}
	receipt := Receipt{ID: receiptID, ObligationID: obligation.ID, TransferReportID: report.ID, AcknowledgedBy: request.ActorID, AmountMinor: request.AmountReceivedMinor, State: "RECORDED", Source: "HOST_ACKNOWLEDGEMENT", Revision: 1, RecordedAt: now}
	if _, err = tx.Exec(ctx, `INSERT INTO payments.receipts(id,obligation_id,transfer_report_id,acknowledged_by,amount_minor,state,revision,source,recorded_at) VALUES($1,$2,$3,$4,$5,'RECORDED',1,$6,$7)`, receipt.ID, receipt.ObligationID, receipt.TransferReportID, receipt.AcknowledgedBy, receipt.AmountMinor, receipt.Source, now); err != nil {
		return Receipt{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE payments.transfer_reports SET status='ACKNOWLEDGED',updated_at=$2 WHERE id=$1`, report.ID, now); err != nil {
		return Receipt{}, err
	}
	var received int64
	if err = tx.QueryRow(ctx, `SELECT COALESCE(sum(amount_minor),0) FROM payments.receipts WHERE obligation_id=$1`, obligation.ID).Scan(&received); err != nil {
		return Receipt{}, err
	}
	status := "PARTIALLY_SATISFIED"
	if received >= obligation.AmountMinor {
		status = "SATISFIED"
	}
	if _, err = tx.Exec(ctx, `UPDATE payments.obligations SET status=$2,updated_at=$3 WHERE id=$1`, obligation.ID, status, now); err != nil {
		return Receipt{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO payments.idempotency(actor_id,action,idempotency_key,fingerprint,resource_type,resource_id,created_at) VALUES($1,$2,$3,$4,'receipt',$5,$6)`, request.ActorID, action, request.IdempotencyKey, fingerprint, receipt.ID, now); err != nil {
		return Receipt{}, err
	}
	if received > obligation.AmountMinor {
		if _, err = s.insertRefund(ctx, tx, obligation, "overpayment:"+obligation.ID, "OVERPAYMENT", received-obligation.AmountMinor, now); err != nil {
			return Receipt{}, err
		}
	}
	if obligation.Purpose == "MATCH_DEPOSIT" && status == "SATISFIED" {
		if _, err = outbox.Enqueue(ctx, tx, "payments.deposit-satisfied", "deposit-satisfied:"+obligation.ID, map[string]any{"eventId": "deposit-satisfied:" + obligation.ID, "participationId": obligation.ParticipationID, "hostId": obligation.PayeeID, "occurredAt": now}, now); err != nil {
			return Receipt{}, err
		}
	}
	if _, err = outbox.Enqueue(ctx, tx, "payments.notification", "payment-notification:receipt:"+receipt.ID, map[string]any{"eventId": "receipt:" + receipt.ID, "kind": "RECEIPT_ACKNOWLEDGED", "recipientId": obligation.PayerID, "matchId": obligation.MatchID, "participationId": obligation.ParticipationID, "occurredAt": now}, now); err != nil {
		return Receipt{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

func (s *Service) MarkNotFound(ctx context.Context, actor, reportID, key, reason string) (TransferReport, error) {
	reason = strings.TrimSpace(reason)
	if actor == "" || reportID == "" || key == "" || reason == "" {
		return TransferReport{}, ErrInvalid
	}
	now := s.clock.Now().UTC()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TransferReport{}, err
	}
	defer tx.Rollback(context.Background())
	if err = lockCommand(ctx, tx, actor, "not-found", key); err != nil {
		return TransferReport{}, err
	}
	fingerprint := reportID + ":" + reason
	if existing, found, loadErr := loadIdempotentReport(ctx, tx, actor, "not-found", key, fingerprint); loadErr != nil {
		return TransferReport{}, loadErr
	} else if found {
		return existing, tx.Commit(ctx)
	}
	var report TransferReport
	var payee, payer, matchID, participationID string
	err = tx.QueryRow(ctx, `SELECT r.id,r.obligation_id,r.reporter_id,r.amount_minor,r.claimed_at,r.reference,r.evidence_ref,r.status,r.resolution_reason,r.created_at,r.updated_at,o.payee_id,o.payer_id,o.match_id,o.participation_id FROM payments.transfer_reports r JOIN payments.obligations o ON o.id=r.obligation_id WHERE r.id=$1 FOR UPDATE`, reportID).Scan(&report.ID, &report.ObligationID, &report.ReporterID, &report.AmountMinor, &report.ClaimedAt, &report.Reference, &report.EvidenceRef, &report.Status, &report.ResolutionReason, &report.CreatedAt, &report.UpdatedAt, &payee, &payer, &matchID, &participationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return TransferReport{}, ErrNotFound
	}
	if err != nil {
		return TransferReport{}, err
	}
	if payee != actor {
		return TransferReport{}, ErrForbidden
	}
	if report.Status == "NOT_FOUND" {
		return report, tx.Commit(ctx)
	}
	if report.Status != "SUBMITTED" {
		return TransferReport{}, ErrConflict
	}
	report.Status, report.ResolutionReason, report.UpdatedAt = "NOT_FOUND", reason, now
	if _, err = tx.Exec(ctx, `UPDATE payments.transfer_reports SET status='NOT_FOUND',resolution_reason=$2,updated_at=$3 WHERE id=$1`, reportID, reason, now); err != nil {
		return TransferReport{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO payments.idempotency(actor_id,action,idempotency_key,fingerprint,resource_type,resource_id,created_at) VALUES($1,'not-found',$2,$3,'transfer_report',$4,$5)`, actor, key, fingerprint, report.ID, now); err != nil {
		return TransferReport{}, err
	}
	if _, err = outbox.Enqueue(ctx, tx, "payments.notification", "payment-notification:not-found:"+report.ID, map[string]any{"eventId": "not-found:" + report.ID, "kind": "TRANSFER_NOT_FOUND", "recipientId": payer, "matchId": matchID, "participationId": participationID, "occurredAt": now}, now); err != nil {
		return TransferReport{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return TransferReport{}, err
	}
	return report, nil
}

func scanObligation(row pgx.Row, item *Obligation) error {
	return row.Scan(&item.ID, &item.ParticipationID, &item.MatchID, &item.PayerID, &item.PayeeID, &item.Purpose, &item.AmountMinor, &item.Currency, &item.Status, &item.DueAt, &item.InstructionRecipient, &item.InstructionText, &item.InstructionVersion, &item.PolicyVersion, &item.CreatedAt, &item.UpdatedAt)
}

func lockCommand(ctx context.Context, tx pgx.Tx, actor, action, key string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,hashtextextended($2,hashtextextended($3,0))))`, actor, action, key)
	return err
}

func loadIdempotentReport(ctx context.Context, tx pgx.Tx, actor, action, key, fingerprint string) (TransferReport, bool, error) {
	var stored, resourceID string
	err := tx.QueryRow(ctx, `SELECT fingerprint,resource_id FROM payments.idempotency WHERE actor_id=$1 AND action=$2 AND idempotency_key=$3`, actor, action, key).Scan(&stored, &resourceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return TransferReport{}, false, nil
	}
	if err != nil {
		return TransferReport{}, false, err
	}
	if stored != fingerprint {
		return TransferReport{}, false, ErrIdempotencyConflict
	}
	var report TransferReport
	err = tx.QueryRow(ctx, `SELECT id,obligation_id,reporter_id,amount_minor,claimed_at,reference,evidence_ref,status,resolution_reason,created_at,updated_at FROM payments.transfer_reports WHERE id=$1`, resourceID).Scan(&report.ID, &report.ObligationID, &report.ReporterID, &report.AmountMinor, &report.ClaimedAt, &report.Reference, &report.EvidenceRef, &report.Status, &report.ResolutionReason, &report.CreatedAt, &report.UpdatedAt)
	return report, true, err
}

func loadIdempotentReceipt(ctx context.Context, tx pgx.Tx, actor, action, key, fingerprint string) (Receipt, bool, error) {
	var stored, resourceID string
	err := tx.QueryRow(ctx, `SELECT fingerprint,resource_id FROM payments.idempotency WHERE actor_id=$1 AND action=$2 AND idempotency_key=$3`, actor, action, key).Scan(&stored, &resourceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Receipt{}, false, nil
	}
	if err != nil {
		return Receipt{}, false, err
	}
	if stored != fingerprint {
		return Receipt{}, false, ErrIdempotencyConflict
	}
	var receipt Receipt
	err = tx.QueryRow(ctx, `SELECT id,obligation_id,transfer_report_id,acknowledged_by,amount_minor,state,revision,source,recorded_at FROM payments.receipts WHERE id=$1`, resourceID).Scan(&receipt.ID, &receipt.ObligationID, &receipt.TransferReportID, &receipt.AcknowledgedBy, &receipt.AmountMinor, &receipt.State, &receipt.Revision, &receipt.Source, &receipt.RecordedAt)
	return receipt, true, err
}

func (s *Service) insertRefund(ctx context.Context, tx pgx.Tx, obligation Obligation, sourceEventID, sourceKind string, amount int64, now time.Time) (Refund, error) {
	if amount <= 0 {
		return Refund{}, nil
	}
	refundID, err := id.New()
	if err != nil {
		return Refund{}, err
	}
	refund := Refund{ID: refundID, ParticipationID: obligation.ParticipationID, MatchID: obligation.MatchID, PayerID: obligation.PayerID, PayeeID: obligation.PayeeID, SourceKind: sourceKind, PolicyVersion: obligation.PolicyVersion, AmountMinor: amount, Currency: obligation.Currency, Status: "DUE", DueAt: now.Add(48 * time.Hour), CreatedAt: now, UpdatedAt: now}
	err = tx.QueryRow(ctx, `INSERT INTO payments.refunds(id,participation_id,match_id,payer_id,payee_id,source_event_id,source_kind,policy_version,amount_minor,currency,status,due_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'DUE',$11,$12,$12) ON CONFLICT(source_event_id) DO UPDATE SET amount_minor=GREATEST(payments.refunds.amount_minor,EXCLUDED.amount_minor),updated_at=EXCLUDED.updated_at RETURNING id,amount_minor,status,due_at,created_at,updated_at`, refund.ID, refund.ParticipationID, refund.MatchID, refund.PayerID, refund.PayeeID, sourceEventID, sourceKind, refund.PolicyVersion, refund.AmountMinor, refund.Currency, refund.DueAt, now).Scan(&refund.ID, &refund.AmountMinor, &refund.Status, &refund.DueAt, &refund.CreatedAt, &refund.UpdatedAt)
	return refund, err
}
