package payments

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"badmintonhub/internal/platform/id"
	"badmintonhub/internal/platform/outbox"
	"github.com/jackc/pgx/v5"
)

func (s *Service) PaymentRequiredHandler() outbox.Handler {
	return func(ctx context.Context, message outbox.Message) error {
		var event PaymentRequiredEvent
		if err := json.Unmarshal(message.Payload, &event); err != nil {
			return errors.New("decode payment-required event")
		}
		if event.EventID == "" || event.ParticipationID == "" || event.FeeMinor <= 0 || event.Currency != "VND" || event.DepositMinor < 0 || event.DepositMinor > event.FeeMinor {
			return ErrInvalid
		}
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(context.Background())
		insert := func(purpose string, amount int64, due time.Time) error {
			if amount <= 0 {
				return nil
			}
			obligationID, newErr := newID()
			if newErr != nil {
				return newErr
			}
			_, newErr = tx.Exec(ctx, `INSERT INTO payments.obligations(id,participation_id,match_id,payer_id,payee_id,purpose,amount_minor,currency,status,due_at,instruction_recipient,instruction_text,instruction_version,policy_version,source_event_id,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'OPEN',$9,$10,$11,$12,$13,$14,$15,$15) ON CONFLICT(source_event_id,purpose) DO NOTHING`, obligationID, event.ParticipationID, event.MatchID, event.PayerID, event.PayeeID, purpose, amount, event.Currency, due, event.PaymentRecipient, event.PaymentInstructions, event.InstructionVersion, event.PolicyVersion, event.EventID, event.OccurredAt.UTC())
			return newErr
		}
		if event.DepositMinor > 0 {
			if err = insert("MATCH_DEPOSIT", event.DepositMinor, event.HoldExpiresAt.UTC()); err != nil {
				return err
			}
		}
		if remainder := event.FeeMinor - event.DepositMinor; remainder > 0 {
			if err = insert("MATCH_PAYMENT", remainder, event.MatchStartAt.UTC()); err != nil {
				return err
			}
		}
		return tx.Commit(ctx)
	}
}

func (s *Service) TransferReportedHandler() outbox.Handler {
	type event struct {
		ParticipationID string    `json:"participationId"`
		PlayerID        string    `json:"playerId"`
		OccurredAt      time.Time `json:"occurredAt"`
	}
	return func(ctx context.Context, message outbox.Message) error {
		if s.settlement == nil {
			return errors.New("match settlement contract is required")
		}
		var payload event
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			return errors.New("decode transfer-reported event")
		}
		return s.settlement.ApplyTransferReport(ctx, payload.PlayerID, payload.ParticipationID, payload.OccurredAt.UTC())
	}
}

func (s *Service) DepositSatisfiedHandler() outbox.Handler {
	type event struct {
		EventID         string    `json:"eventId"`
		ParticipationID string    `json:"participationId"`
		HostID          string    `json:"hostId"`
		OccurredAt      time.Time `json:"occurredAt"`
	}
	return func(ctx context.Context, message outbox.Message) error {
		if s.settlement == nil {
			return errors.New("match settlement contract is required")
		}
		var payload event
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			return errors.New("decode deposit-satisfied event")
		}
		joined, err := s.settlement.ConfirmPayment(ctx, payload.HostID, payload.ParticipationID, payload.OccurredAt.UTC())
		if err != nil {
			return err
		}
		if joined {
			return nil
		}
		return s.createRefundForParticipation(ctx, payload.ParticipationID, "late-money:"+payload.EventID, "LATE_MONEY", "MVP-2026-10-D16", payload.OccurredAt.UTC())
	}
}

func (s *Service) CancellationHandler() outbox.Handler {
	return func(ctx context.Context, message outbox.Message) error {
		var event CancellationEvent
		if err := json.Unmarshal(message.Payload, &event); err != nil {
			return errors.New("decode cancellation event")
		}
		if event.RefundOutcome != "FULL" {
			return nil
		}
		return s.createRefundForParticipation(ctx, event.ParticipationID, event.EventID, "CANCELLATION", event.PolicyVersion, event.OccurredAt.UTC())
	}
}

func (s *Service) createRefundForParticipation(ctx context.Context, participationID, sourceEventID, sourceKind, policyVersion string, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var obligation Obligation
	var received, alreadyRefunded int64
	err = tx.QueryRow(ctx, `SELECT o.id,o.participation_id,o.match_id,o.payer_id,o.payee_id,o.purpose,o.amount_minor,o.currency,o.status,o.due_at,o.instruction_recipient,o.instruction_text,o.instruction_version,o.policy_version,o.created_at,o.updated_at,COALESCE((SELECT sum(r.amount_minor) FROM payments.receipts r JOIN payments.obligations ro ON ro.id=r.obligation_id WHERE ro.participation_id=o.participation_id),0),COALESCE((SELECT sum(f.amount_minor) FROM payments.refunds f WHERE f.participation_id=o.participation_id AND f.status IN('DUE','OVERDUE','REPORTED_SENT','CONFIRMED','DISPUTED')),0) FROM payments.obligations o WHERE o.participation_id=$1 ORDER BY o.created_at LIMIT 1 FOR UPDATE OF o`, participationID).Scan(&obligation.ID, &obligation.ParticipationID, &obligation.MatchID, &obligation.PayerID, &obligation.PayeeID, &obligation.Purpose, &obligation.AmountMinor, &obligation.Currency, &obligation.Status, &obligation.DueAt, &obligation.InstructionRecipient, &obligation.InstructionText, &obligation.InstructionVersion, &obligation.PolicyVersion, &obligation.CreatedAt, &obligation.UpdatedAt, &received, &alreadyRefunded)
	if errors.Is(err, pgx.ErrNoRows) {
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	if received <= alreadyRefunded {
		return tx.Commit(ctx)
	}
	obligation.PolicyVersion = policyVersion
	refund, err := s.insertRefund(ctx, tx, obligation, sourceEventID, sourceKind, received-alreadyRefunded, now)
	if err != nil {
		return err
	}
	if refund.ID != "" {
		if _, err = outbox.Enqueue(ctx, tx, "payments.notification", "payment-notification:refund-due:"+refund.ID, map[string]any{"eventId": "refund-due:" + refund.ID, "kind": "REFUND_DUE", "recipientId": obligation.PayeeID, "counterpartyId": obligation.PayerID, "matchId": obligation.MatchID, "participationId": obligation.ParticipationID, "occurredAt": now}, now); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func newID() (string, error) { return id.New() }
