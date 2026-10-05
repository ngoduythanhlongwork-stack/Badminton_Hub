package payments

import (
	"context"
)

type Ledger struct {
	Obligations []Obligation
	Reports     []TransferReport
	Receipts    []Receipt
	Refunds     []Refund
}

func (s *Service) Ledger(ctx context.Context, actor, participationID string) (Ledger, error) {
	obligations, err := s.Obligations(ctx, actor, participationID)
	if err != nil {
		return Ledger{}, err
	}
	result := Ledger{Obligations: obligations, Reports: make([]TransferReport, 0), Receipts: make([]Receipt, 0), Refunds: make([]Refund, 0)}
	rows, err := s.pool.Query(ctx, `SELECT r.id,r.obligation_id,r.reporter_id,r.amount_minor,r.claimed_at,r.reference,r.evidence_ref,r.status,r.resolution_reason,r.created_at,r.updated_at FROM payments.transfer_reports r JOIN payments.obligations o ON o.id=r.obligation_id WHERE o.participation_id=$1 ORDER BY r.created_at`, participationID)
	if err != nil {
		return Ledger{}, err
	}
	for rows.Next() {
		var item TransferReport
		if err = scanTransferReport(rows, &item); err != nil {
			rows.Close()
			return Ledger{}, err
		}
		result.Reports = append(result.Reports, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return Ledger{}, err
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT r.id,r.obligation_id,r.transfer_report_id,r.acknowledged_by,r.amount_minor,r.state,r.revision,r.source,r.recorded_at FROM payments.receipts r JOIN payments.obligations o ON o.id=r.obligation_id WHERE o.participation_id=$1 ORDER BY r.recorded_at`, participationID)
	if err != nil {
		return Ledger{}, err
	}
	for rows.Next() {
		var item Receipt
		if err = scanReceipt(rows, &item); err != nil {
			rows.Close()
			return Ledger{}, err
		}
		result.Receipts = append(result.Receipts, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return Ledger{}, err
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT id,participation_id,match_id,payer_id,payee_id,source_kind,policy_version,amount_minor,currency,status,due_at,sent_amount_minor,sent_at,sent_reference,evidence_ref,dispute_reason,created_at,updated_at FROM payments.refunds WHERE participation_id=$1 ORDER BY created_at`, participationID)
	if err != nil {
		return Ledger{}, err
	}
	for rows.Next() {
		var item Refund
		if err = scanRefund(rows, &item); err != nil {
			rows.Close()
			return Ledger{}, err
		}
		result.Refunds = append(result.Refunds, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return Ledger{}, err
	}
	rows.Close()
	return result, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanTransferReport(row rowScanner, item *TransferReport) error {
	return row.Scan(&item.ID, &item.ObligationID, &item.ReporterID, &item.AmountMinor, &item.ClaimedAt, &item.Reference, &item.EvidenceRef, &item.Status, &item.ResolutionReason, &item.CreatedAt, &item.UpdatedAt)
}

func scanReceipt(row rowScanner, item *Receipt) error {
	return row.Scan(&item.ID, &item.ObligationID, &item.TransferReportID, &item.AcknowledgedBy, &item.AmountMinor, &item.State, &item.Revision, &item.Source, &item.RecordedAt)
}
