package payments

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"badmintonhub/internal/platform/outbox"
	"github.com/jackc/pgx/v5"
)

func (s *Service) Refunds(ctx context.Context, actor string) ([]Refund, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,participation_id,match_id,payer_id,payee_id,source_kind,policy_version,amount_minor,currency,status,due_at,sent_amount_minor,sent_at,sent_reference,evidence_ref,dispute_reason,created_at,updated_at FROM payments.refunds WHERE payer_id=$1 OR payee_id=$1 ORDER BY created_at DESC`, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Refund, 0)
	for rows.Next() {
		var item Refund
		if err = scanRefund(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type ReportRefundRequest struct {
	ActorID, RefundID, IdempotencyKey, Reference, EvidenceRef string
	AmountMinor                                               int64
	SentAt                                                    time.Time
}

func (s *Service) ReportRefundSent(ctx context.Context, request ReportRefundRequest) (Refund, error) {
	request.Reference = strings.TrimSpace(request.Reference)
	request.EvidenceRef = strings.TrimSpace(request.EvidenceRef)
	if request.ActorID == "" || request.RefundID == "" || request.IdempotencyKey == "" || request.Reference == "" || request.AmountMinor <= 0 || request.SentAt.IsZero() {
		return Refund{}, ErrInvalid
	}
	now := s.clock.Now().UTC()
	if request.SentAt.After(now.Add(5 * time.Minute)) {
		return Refund{}, ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Refund{}, err
	}
	defer tx.Rollback(context.Background())
	const action = "report-refund"
	if err = lockCommand(ctx, tx, request.ActorID, action, request.IdempotencyKey); err != nil {
		return Refund{}, err
	}
	fingerprint := fmt.Sprintf("%s:%d:%s:%s", request.RefundID, request.AmountMinor, request.SentAt.UTC().Format(time.RFC3339Nano), request.Reference)
	if item, found, loadErr := loadIdempotentRefund(ctx, tx, request.ActorID, action, request.IdempotencyKey, fingerprint); loadErr != nil {
		return Refund{}, loadErr
	} else if found {
		return item, tx.Commit(ctx)
	}
	var item Refund
	if err = scanRefund(tx.QueryRow(ctx, `SELECT id,participation_id,match_id,payer_id,payee_id,source_kind,policy_version,amount_minor,currency,status,due_at,sent_amount_minor,sent_at,sent_reference,evidence_ref,dispute_reason,created_at,updated_at FROM payments.refunds WHERE id=$1 FOR UPDATE`, request.RefundID), &item); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Refund{}, ErrNotFound
		}
		return Refund{}, err
	}
	if item.PayeeID != request.ActorID {
		return Refund{}, ErrForbidden
	}
	if item.Status != "DUE" && item.Status != "OVERDUE" {
		return Refund{}, ErrConflict
	}
	if request.AmountMinor != item.AmountMinor {
		return Refund{}, ErrInvalid
	}
	item.Status, item.SentAmountMinor, item.SentAt, item.SentReference, item.EvidenceRef, item.UpdatedAt = "REPORTED_SENT", &request.AmountMinor, ptrTime(request.SentAt.UTC()), request.Reference, request.EvidenceRef, now
	if _, err = tx.Exec(ctx, `UPDATE payments.refunds SET status='REPORTED_SENT',sent_amount_minor=$2,sent_at=$3,sent_reference=$4,evidence_ref=$5,reported_by=$6,updated_at=$7 WHERE id=$1`, item.ID, request.AmountMinor, request.SentAt.UTC(), request.Reference, request.EvidenceRef, request.ActorID, now); err != nil {
		return Refund{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO payments.idempotency(actor_id,action,idempotency_key,fingerprint,resource_type,resource_id,created_at) VALUES($1,$2,$3,$4,'refund',$5,$6)`, request.ActorID, action, request.IdempotencyKey, fingerprint, item.ID, now); err != nil {
		return Refund{}, err
	}
	if _, err = outbox.Enqueue(ctx, tx, "payments.notification", "payment-notification:refund-sent:"+item.ID, map[string]any{"eventId": "refund-sent:" + item.ID, "kind": "REFUND_REPORTED_SENT", "recipientId": item.PayerID, "matchId": item.MatchID, "participationId": item.ParticipationID, "occurredAt": now}, now); err != nil {
		return Refund{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Refund{}, err
	}
	return item, nil
}

func (s *Service) ResolveRefund(ctx context.Context, actor, refundID, key, decision, reason string) (Refund, error) {
	decision = strings.ToUpper(strings.TrimSpace(decision))
	reason = strings.TrimSpace(reason)
	if actor == "" || refundID == "" || key == "" || (decision != "CONFIRM" && decision != "DISPUTE") || (decision == "DISPUTE" && reason == "") {
		return Refund{}, ErrInvalid
	}
	now := s.clock.Now().UTC()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Refund{}, err
	}
	defer tx.Rollback(context.Background())
	action := "resolve-refund:" + decision
	if err = lockCommand(ctx, tx, actor, action, key); err != nil {
		return Refund{}, err
	}
	fingerprint := refundID + ":" + decision + ":" + reason
	if existing, found, loadErr := loadIdempotentRefund(ctx, tx, actor, action, key, fingerprint); loadErr != nil {
		return Refund{}, loadErr
	} else if found {
		return existing, tx.Commit(ctx)
	}
	var item Refund
	if err = scanRefund(tx.QueryRow(ctx, `SELECT id,participation_id,match_id,payer_id,payee_id,source_kind,policy_version,amount_minor,currency,status,due_at,sent_amount_minor,sent_at,sent_reference,evidence_ref,dispute_reason,created_at,updated_at FROM payments.refunds WHERE id=$1 FOR UPDATE`, refundID), &item); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Refund{}, ErrNotFound
		}
		return Refund{}, err
	}
	if item.PayerID != actor {
		return Refund{}, ErrForbidden
	}
	if item.Status != "REPORTED_SENT" {
		return Refund{}, ErrConflict
	}
	if decision == "CONFIRM" {
		var received, confirmed int64
		if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT sum(r.amount_minor) FROM payments.receipts r JOIN payments.obligations o ON o.id=r.obligation_id WHERE o.participation_id=$1),0),COALESCE((SELECT sum(amount_minor) FROM payments.refunds WHERE participation_id=$1 AND status='CONFIRMED'),0)`, item.ParticipationID).Scan(&received, &confirmed); err != nil {
			return Refund{}, err
		}
		if confirmed+item.AmountMinor > received {
			return Refund{}, ErrRefundLimit
		}
		item.Status, item.UpdatedAt = "CONFIRMED", now
		if _, err = tx.Exec(ctx, `UPDATE payments.refunds SET status='CONFIRMED',confirmed_by=$2,confirmed_at=$3,updated_at=$3 WHERE id=$1`, item.ID, actor, now); err != nil {
			return Refund{}, err
		}
	} else {
		item.Status, item.DisputeReason, item.UpdatedAt = "DISPUTED", reason, now
		if _, err = tx.Exec(ctx, `UPDATE payments.refunds SET status='DISPUTED',dispute_reason=$2,updated_at=$3 WHERE id=$1`, item.ID, reason, now); err != nil {
			return Refund{}, err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO payments.idempotency(actor_id,action,idempotency_key,fingerprint,resource_type,resource_id,created_at) VALUES($1,$2,$3,$4,'refund',$5,$6)`, actor, action, key, fingerprint, item.ID, now); err != nil {
		return Refund{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Refund{}, err
	}
	return item, nil
}

func (s *Service) MarkOverdue(ctx context.Context, limit int) (int, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	now := s.clock.Now().UTC()
	tag, err := s.pool.Exec(ctx, `WITH due AS (SELECT id FROM payments.refunds WHERE status='DUE' AND due_at<=$1 ORDER BY due_at LIMIT $2 FOR UPDATE SKIP LOCKED) UPDATE payments.refunds r SET status='OVERDUE',updated_at=$1 FROM due WHERE r.id=due.id`, now, limit)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func scanRefund(row pgx.Row, item *Refund) error {
	return row.Scan(&item.ID, &item.ParticipationID, &item.MatchID, &item.PayerID, &item.PayeeID, &item.SourceKind, &item.PolicyVersion, &item.AmountMinor, &item.Currency, &item.Status, &item.DueAt, &item.SentAmountMinor, &item.SentAt, &item.SentReference, &item.EvidenceRef, &item.DisputeReason, &item.CreatedAt, &item.UpdatedAt)
}

func loadIdempotentRefund(ctx context.Context, tx pgx.Tx, actor, action, key, fingerprint string) (Refund, bool, error) {
	var stored, resourceID string
	err := tx.QueryRow(ctx, `SELECT fingerprint,resource_id FROM payments.idempotency WHERE actor_id=$1 AND action=$2 AND idempotency_key=$3`, actor, action, key).Scan(&stored, &resourceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Refund{}, false, nil
	}
	if err != nil {
		return Refund{}, false, err
	}
	if stored != fingerprint {
		return Refund{}, false, ErrIdempotencyConflict
	}
	var item Refund
	err = scanRefund(tx.QueryRow(ctx, `SELECT id,participation_id,match_id,payer_id,payee_id,source_kind,policy_version,amount_minor,currency,status,due_at,sent_amount_minor,sent_at,sent_reference,evidence_ref,dispute_reason,created_at,updated_at FROM payments.refunds WHERE id=$1`, resourceID), &item)
	return item, true, err
}

func ptrTime(value time.Time) *time.Time { return &value }
