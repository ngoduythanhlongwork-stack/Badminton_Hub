package matches

import (
	"badmintonhub/internal/platform/id"
	"badmintonhub/internal/platform/outbox"
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(p *pgxpool.Pool) *PostgresRepository { return &PostgresRepository{p} }

const insertMatch = `INSERT INTO matches.matches(id,host_id,title,description,venue_id,venue_name,venue_address,venue_area,venue_time_zone,court,start_at,end_at,format,style,rules,min_level,max_level,capacity,fee_minor,currency,join_mode,host_plays,court_attested,court_attested_at,status,created_at,updated_at,deposit_minor,payment_recipient,payment_instructions,payment_instruction_version,cancellation_policy_version) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32)`

func (r *PostgresRepository) Create(c context.Context, m Match) (Match, error) {
	_, e := r.pool.Exec(c, insertMatch, m.ID, m.HostID, m.Title, m.Description, m.Venue.ID, m.Venue.Name, m.Venue.Address, m.Venue.Area, m.Venue.TimeZone, m.Venue.Court, m.StartAt, m.EndAt, m.Format, m.Style, m.Rules, m.MinLevel, m.MaxLevel, m.Capacity, m.FeeMinor, m.Currency, m.JoinMode, m.HostPlays, m.CourtAttested, m.CourtAttestedAt, m.Status, m.CreatedAt, m.UpdatedAt, m.DepositMinor, m.PaymentRecipient, m.PaymentInstructions, m.PaymentInstructionVersion, m.CancellationPolicyVersion)
	return m, e
}
func (r *PostgresRepository) UpdateDraft(c context.Context, m Match) (Match, error) {
	tag, e := r.pool.Exec(c, `UPDATE matches.matches SET title=$3,description=$4,venue_id=$5,venue_name=$6,venue_address=$7,venue_area=$8,venue_time_zone=$9,court=$10,start_at=$11,end_at=$12,format=$13,style=$14,rules=$15,min_level=$16,max_level=$17,capacity=$18,fee_minor=$19,currency=$20,join_mode=$21,host_plays=$22,court_attested=$23,court_attested_at=$24,updated_at=$25,deposit_minor=$26,payment_recipient=$27,payment_instructions=$28,payment_instruction_version=$29,cancellation_policy_version=$30 WHERE id=$1 AND host_id=$2 AND status='DRAFT'`, m.ID, m.HostID, m.Title, m.Description, m.Venue.ID, m.Venue.Name, m.Venue.Address, m.Venue.Area, m.Venue.TimeZone, m.Venue.Court, m.StartAt, m.EndAt, m.Format, m.Style, m.Rules, m.MinLevel, m.MaxLevel, m.Capacity, m.FeeMinor, m.Currency, m.JoinMode, m.HostPlays, m.CourtAttested, m.CourtAttestedAt, m.UpdatedAt, m.DepositMinor, m.PaymentRecipient, m.PaymentInstructions, m.PaymentInstructionVersion, m.CancellationPolicyVersion)
	if e != nil {
		return Match{}, e
	}
	if tag.RowsAffected() != 1 {
		return Match{}, ErrForbidden
	}
	m.Status = StatusDraft
	return m, nil
}
func (r *PostgresRepository) OwnedDraft(c context.Context, host, mid string) (Match, error) {
	m, err := r.get(c, r.pool, mid, false, false)
	if err != nil {
		return Match{}, err
	}
	if m.HostID != host || m.Status != StatusDraft {
		return Match{}, ErrForbidden
	}
	return m, nil
}
func (r *PostgresRepository) Publish(c context.Context, host, mid string, now time.Time) (Match, error) {
	tx, e := r.pool.Begin(c)
	if e != nil {
		return Match{}, e
	}
	defer tx.Rollback(context.Background())
	m, e := r.get(c, tx, mid, false, true)
	if e != nil {
		return Match{}, e
	}
	if m.HostID != host || m.Status != StatusDraft {
		return Match{}, ErrForbidden
	}
	m.Status = StatusOpen
	m.UpdatedAt = now
	if m.HostPlays {
		if e = r.reserve(c, tx, host, m, now, ""); e != nil {
			return Match{}, e
		}
		pid, _ := id.New()
		_, e = tx.Exec(c, `INSERT INTO matches.participations(id,match_id,player_id,status,start_at,end_at,created_at,updated_at) VALUES($1,$2,$3,'JOINED',$4,$5,$6,$6)`, pid, m.ID, host, m.StartAt, m.EndAt, now)
		if e != nil {
			return Match{}, e
		}
		m.Occupied = 1
	}
	if m.Occupied >= m.Capacity {
		m.Status = StatusFull
	}
	_, e = tx.Exec(c, `UPDATE matches.matches SET status=$2,updated_at=$3 WHERE id=$1`, mid, m.Status, now)
	if e != nil {
		return Match{}, e
	}
	if e = tx.Commit(c); e != nil {
		return Match{}, e
	}
	return m, nil
}
func (r *PostgresRepository) PublicDetail(c context.Context, id string) (Match, error) {
	m, e := r.get(c, r.pool, id, true, false)
	if e == nil && !m.StartAt.After(time.Now().UTC()) {
		return Match{}, ErrNotOpen
	}
	return m, e
}
func (r *PostgresRepository) Participation(c context.Context, pid string) (Participation, error) {
	var p Participation
	e := scanPart(r.pool.QueryRow(c, `SELECT id,match_id,player_id,status,decision_reason,created_at,updated_at FROM matches.participations WHERE id=$1`, pid), &p)
	if errors.Is(e, pgx.ErrNoRows) {
		return Participation{}, ErrNotFound
	}
	if e == nil {
		_ = r.loadHold(c, r.pool, &p)
	}
	return p, e
}
func (r *PostgresRepository) PublicList(c context.Context, f Filters) (Page, error) {
	args := []any{f.Limit + 1}
	q := `SELECT id FROM matches.matches WHERE status IN('OPEN','FULL') AND start_at > now()`
	n := 2
	if f.From != nil {
		q += fmt.Sprintf(" AND start_at >= $%d", n)
		args = append(args, f.From.UTC())
		n++
	}
	if f.To != nil {
		q += fmt.Sprintf(" AND start_at < $%d", n)
		args = append(args, f.To.UTC())
		n++
	}
	if f.MinLevel != nil {
		q += fmt.Sprintf(" AND max_level >= $%d", n)
		args = append(args, *f.MinLevel)
		n++
	}
	if f.MaxLevel != nil {
		q += fmt.Sprintf(" AND min_level <= $%d", n)
		args = append(args, *f.MaxLevel)
		n++
	}
	if f.Area != "" {
		q += fmt.Sprintf(" AND venue_area=$%d", n)
		args = append(args, f.Area)
		n++
	}
	if f.Format != "" {
		q += fmt.Sprintf(" AND format=$%d", n)
		args = append(args, f.Format)
		n++
	}
	if f.MaxCostMinor != nil {
		q += fmt.Sprintf(" AND fee_minor <= $%d", n)
		args = append(args, *f.MaxCostMinor)
		n++
	}
	if f.Cursor != "" {
		q += fmt.Sprintf(" AND id > $%d", n)
		args = append(args, f.Cursor)
	}
	q += ` ORDER BY id LIMIT $1`
	rows, e := r.pool.Query(c, q, args...)
	if e != nil {
		return Page{}, e
	}
	var ids []string
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return Page{}, e
		}
		ids = append(ids, id)
	}
	if e = rows.Err(); e != nil {
		rows.Close()
		return Page{}, e
	}
	rows.Close()
	var out Page
	for _, id := range ids {
		m, x := r.get(c, r.pool, id, true, false)
		if x != nil {
			return Page{}, x
		}
		out.Matches = append(out.Matches, m)
	}
	if len(out.Matches) > f.Limit {
		out.NextCursor = out.Matches[f.Limit-1].ID
		out.Matches = out.Matches[:f.Limit]
	}
	return out, nil
}
func (r *PostgresRepository) Join(c context.Context, player, mid, key string, now time.Time) (Participation, error) {
	tx, e := r.pool.Begin(c)
	if e != nil {
		return Participation{}, e
	}
	defer tx.Rollback(context.Background())
	if e = lockIdempotency(c, tx, player, "join", key); e != nil {
		return Participation{}, e
	}
	if p, x := r.idempotent(c, tx, player, "join", key, mid); x == nil {
		return p, nil
	} else if !errors.Is(x, pgx.ErrNoRows) {
		return Participation{}, x
	}
	m, e := r.get(c, tx, mid, true, true)
	if e != nil {
		return Participation{}, e
	}
	if !m.StartAt.After(now) || m.Status == StatusFull {
		return Participation{}, ErrNotOpen
	}
	var existing Participation
	e = scanPart(tx.QueryRow(c, `SELECT id,match_id,player_id,status,decision_reason,created_at,updated_at FROM matches.participations WHERE match_id=$1 AND player_id=$2 AND status IN('REQUESTED','AWAITING_PAYMENT','JOINED')`, mid, player), &existing)
	if e == nil {
		return Participation{}, ErrAlreadyParticipating
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return Participation{}, e
	}
	status := ParticipationRequested
	if m.JoinMode == JoinInstant {
		if m.DepositMinor > 0 {
			status = ParticipationAwaitingPayment
		} else {
			status = ParticipationJoined
		}
		if e = r.reserve(c, tx, player, m, now, ""); e != nil {
			return Participation{}, e
		}
	}
	pid, _ := id.New()
	p := Participation{ID: pid, MatchID: mid, PlayerID: player, Status: status, CreatedAt: now, UpdatedAt: now}
	_, e = tx.Exec(c, `INSERT INTO matches.participations(id,match_id,player_id,status,start_at,end_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$7)`, pid, mid, player, status, m.StartAt, m.EndAt, now)
	if e != nil {
		return Participation{}, mapConflict(e)
	}
	if status == ParticipationAwaitingPayment {
		hold, err := r.createHold(c, tx, p, m, now)
		if err != nil {
			return Participation{}, err
		}
		p.Hold = &hold
	}
	if status == ParticipationJoined || status == ParticipationAwaitingPayment {
		if e = r.refresh(c, tx, mid, m.Capacity, now); e != nil {
			return Participation{}, e
		}
		if m.FeeMinor > 0 {
			if e = enqueuePaymentRequired(c, tx, p, m, now); e != nil {
				return Participation{}, e
			}
		}
		if status == ParticipationJoined {
			if e = enqueueJoined(c, tx, p, m, now); e != nil {
				return Participation{}, e
			}
		}
	}
	_, e = tx.Exec(c, `INSERT INTO matches.idempotency(actor_id,action,idempotency_key,fingerprint,participation_id,created_at) VALUES($1,'join',$2,$3,$4,$5)`, player, key, mid, pid, now)
	if e != nil {
		return Participation{}, e
	}
	if e = tx.Commit(c); e != nil {
		return Participation{}, e
	}
	return p, nil
}
func (r *PostgresRepository) Decide(c context.Context, host, mid, pid, key string, target ParticipationStatus, reason string, now time.Time) (Participation, error) {
	if key == "" {
		return Participation{}, ErrInvalid
	}
	tx, e := r.pool.Begin(c)
	if e != nil {
		return Participation{}, e
	}
	defer tx.Rollback(context.Background())
	action := "decide:" + string(target)
	fingerprint := mid + ":" + pid
	if e = lockIdempotency(c, tx, host, action, key); e != nil {
		return Participation{}, e
	}
	if p, x := r.idempotent(c, tx, host, action, key, fingerprint); x == nil {
		return p, nil
	} else if !errors.Is(x, pgx.ErrNoRows) {
		return Participation{}, x
	}
	m, e := r.get(c, tx, mid, false, true)
	if e != nil {
		return Participation{}, e
	}
	if m.HostID != host {
		return Participation{}, ErrForbidden
	}
	var p Participation
	e = scanPart(tx.QueryRow(c, `SELECT id,match_id,player_id,status,decision_reason,created_at,updated_at FROM matches.participations WHERE id=$1 AND match_id=$2 FOR UPDATE`, pid, mid), &p)
	if errors.Is(e, pgx.ErrNoRows) {
		return Participation{}, ErrNotFound
	}
	if e != nil {
		return Participation{}, e
	}
	if target == ParticipationJoined {
		if p.Status != ParticipationRequested {
			return Participation{}, ErrInvalid
		}
		if e = r.reserve(c, tx, p.PlayerID, m, now, ""); e != nil {
			return Participation{}, e
		}
		if m.DepositMinor > 0 {
			target = ParticipationAwaitingPayment
		}
	} else if target == ParticipationRejected && p.Status != ParticipationRequested {
		return Participation{}, ErrInvalid
	} else if target == ParticipationRemoved && p.Status != ParticipationJoined && p.Status != ParticipationAwaitingPayment {
		return Participation{}, ErrInvalid
	}
	p.Status = target
	p.DecisionReason = reason
	p.UpdatedAt = now
	_, e = tx.Exec(c, `UPDATE matches.participations SET status=$2,decision_reason=$3,updated_at=$4 WHERE id=$1`, pid, target, reason, now)
	if e != nil {
		return Participation{}, e
	}
	if target == ParticipationAwaitingPayment {
		hold, err := r.createHold(c, tx, p, m, now)
		if err != nil {
			return Participation{}, err
		}
		p.Hold = &hold
	}
	if target == ParticipationJoined || target == ParticipationAwaitingPayment {
		if m.FeeMinor > 0 {
			if e = enqueuePaymentRequired(c, tx, p, m, now); e != nil {
				return Participation{}, e
			}
		}
		if target == ParticipationJoined {
			if e = enqueueJoined(c, tx, p, m, now); e != nil {
				return Participation{}, e
			}
		}
	}
	if target == ParticipationRemoved {
		if _, e = tx.Exec(c, `UPDATE matches.holds SET status='RELEASED',released_at=$2,release_reason='HOST_REMOVAL',updated_at=$2 WHERE participation_id=$1 AND status='HELD'`, pid, now); e != nil {
			return Participation{}, e
		}
		if e = enqueueCancellation(c, tx, m, p, host, "HOST_REMOVAL", reason, "FULL", now); e != nil {
			return Participation{}, e
		}
	}
	if e = r.refresh(c, tx, mid, m.Capacity, now); e != nil {
		return Participation{}, e
	}
	_, e = tx.Exec(c, `INSERT INTO matches.idempotency(actor_id,action,idempotency_key,fingerprint,participation_id,created_at) VALUES($1,$2,$3,$4,$5,$6)`, host, action, key, fingerprint, pid, now)
	if e != nil {
		return Participation{}, e
	}
	if e = tx.Commit(c); e != nil {
		return Participation{}, e
	}
	return p, nil
}

func (r *PostgresRepository) createHold(ctx context.Context, tx pgx.Tx, p Participation, m Match, now time.Time) (Hold, error) {
	expiresAt := now.Add(30 * time.Minute)
	if expiresAt.After(m.StartAt) {
		expiresAt = m.StartAt
	}
	holdID, err := id.New()
	if err != nil {
		return Hold{}, err
	}
	hold := Hold{ID: holdID, ParticipationID: p.ID, Status: HoldHeld, ExpiresAt: expiresAt}
	_, err = tx.Exec(ctx, `INSERT INTO matches.holds(id,participation_id,match_id,player_id,status,expires_at,created_at,updated_at) VALUES($1,$2,$3,$4,'HELD',$5,$6,$6)`, hold.ID, p.ID, p.MatchID, p.PlayerID, expiresAt, now)
	return hold, err
}

func enqueuePaymentRequired(ctx context.Context, tx pgx.Tx, p Participation, m Match, now time.Time) error {
	eventID := "payment-required:" + p.ID
	holdExpires := m.StartAt
	if p.Hold != nil {
		holdExpires = p.Hold.ExpiresAt
	}
	event := PaymentRequiredEvent{EventID: eventID, ParticipationID: p.ID, MatchID: m.ID, PayerID: p.PlayerID, PayeeID: m.HostID, FeeMinor: m.FeeMinor, DepositMinor: m.DepositMinor, Currency: m.Currency, PaymentRecipient: m.PaymentRecipient, PaymentInstructions: m.PaymentInstructions, PolicyVersion: m.CancellationPolicyVersion, InstructionVersion: m.PaymentInstructionVersion, HoldExpiresAt: holdExpires, MatchStartAt: m.StartAt, OccurredAt: now}
	_, err := outbox.Enqueue(ctx, tx, "matches.payment-required", eventID, event, now)
	return err
}

func enqueueCancellation(ctx context.Context, tx pgx.Tx, m Match, p Participation, actor, cause, reason, refund string, now time.Time) error {
	eventID, err := id.New()
	if err != nil {
		return err
	}
	event := CancellationEvent{EventID: eventID, MatchID: m.ID, ParticipationID: p.ID, PlayerID: p.PlayerID, HostID: m.HostID, Cause: cause, Reason: reason, RefundOutcome: refund, PolicyVersion: m.CancellationPolicyVersion, OccurredAt: now}
	if _, err = outbox.Enqueue(ctx, tx, "matches.cancelled", "matches-cancelled:"+eventID, event, now); err != nil {
		return err
	}
	auditID, err := id.New()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO matches.cancellation_audit(id,match_id,participation_id,actor_id,cause,reason,refund_outcome,policy_version,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, auditID, m.ID, p.ID, actor, cause, reason, refund, m.CancellationPolicyVersion, now)
	return err
}

func enqueueJoined(ctx context.Context, tx pgx.Tx, p Participation, m Match, now time.Time) error {
	_, err := outbox.Enqueue(ctx, tx, "matches.joined", "matches-joined:"+p.ID, map[string]any{"eventId": "matches-joined:" + p.ID, "recipientId": p.PlayerID, "hostId": m.HostID, "matchId": m.ID, "participationId": p.ID, "startAt": m.StartAt, "occurredAt": now}, now)
	return err
}

func (r *PostgresRepository) loadHold(ctx context.Context, q rowQuerier, p *Participation) error {
	var hold Hold
	err := q.QueryRow(ctx, `SELECT id,participation_id,status,expires_at,transfer_reported_at FROM matches.holds WHERE participation_id=$1`, p.ID).Scan(&hold.ID, &hold.ParticipationID, &hold.Status, &hold.ExpiresAt, &hold.TransferReportedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err == nil {
		p.Hold = &hold
	}
	return err
}

func (r *PostgresRepository) ExtendHold(ctx context.Context, player, participationID string, now time.Time) (Participation, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Participation{}, err
	}
	defer tx.Rollback(context.Background())
	var p Participation
	var hold Hold
	var m Match
	err = tx.QueryRow(ctx, `SELECT p.id,p.match_id,p.player_id,p.status,p.decision_reason,p.created_at,p.updated_at,h.id,h.participation_id,h.status,h.expires_at,h.transfer_reported_at,m.start_at,m.end_at,m.capacity,m.status FROM matches.participations p JOIN matches.holds h ON h.participation_id=p.id JOIN matches.matches m ON m.id=p.match_id WHERE p.id=$1 FOR UPDATE OF m,p,h`, participationID).Scan(&p.ID, &p.MatchID, &p.PlayerID, &p.Status, &p.DecisionReason, &p.CreatedAt, &p.UpdatedAt, &hold.ID, &hold.ParticipationID, &hold.Status, &hold.ExpiresAt, &hold.TransferReportedAt, &m.StartAt, &m.EndAt, &m.Capacity, &m.Status)
	m.ID = p.MatchID
	if errors.Is(err, pgx.ErrNoRows) {
		return Participation{}, ErrNotFound
	}
	if err != nil {
		return Participation{}, err
	}
	if p.PlayerID != player {
		return Participation{}, ErrForbidden
	}
	revivableExpiryRace := p.Status == ParticipationExpired && hold.Status == HoldExpired && hold.ExpiresAt.After(now) && (m.Status == StatusOpen || m.Status == StatusFull)
	if revivableExpiryRace {
		if reserveErr := r.reserve(ctx, tx, p.PlayerID, m, now, p.ID); reserveErr != nil {
			return Participation{}, ErrHoldExpired
		}
		p.Status, hold.Status = ParticipationAwaitingPayment, HoldHeld
		if _, err = tx.Exec(ctx, `UPDATE matches.participations SET status='AWAITING_PAYMENT',updated_at=$2 WHERE id=$1`, p.ID, now); err != nil {
			return Participation{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE matches.holds SET status='HELD',released_at=NULL,release_reason='',updated_at=$2 WHERE id=$1`, hold.ID, now); err != nil {
			return Participation{}, err
		}
	}
	if p.Status != ParticipationAwaitingPayment || hold.Status != HoldHeld || !hold.ExpiresAt.After(now) {
		if p.Status == ParticipationAwaitingPayment && hold.Status == HoldHeld {
			if _, err = tx.Exec(ctx, `UPDATE matches.holds SET status='EXPIRED',released_at=$2,release_reason='REPORT_DEADLINE',updated_at=$2 WHERE id=$1`, hold.ID, now); err != nil {
				return Participation{}, err
			}
			if _, err = tx.Exec(ctx, `UPDATE matches.participations SET status='EXPIRED',updated_at=$2 WHERE id=$1`, p.ID, now); err != nil {
				return Participation{}, err
			}
			if err = r.refresh(ctx, tx, m.ID, m.Capacity, now); err != nil {
				return Participation{}, err
			}
			if err = tx.Commit(ctx); err != nil {
				return Participation{}, err
			}
		}
		return Participation{}, ErrHoldExpired
	}
	firstReportedAt := now
	if hold.TransferReportedAt != nil {
		firstReportedAt = hold.TransferReportedAt.UTC()
	}
	extended := firstReportedAt.Add(2 * time.Hour)
	if extended.After(m.StartAt) {
		extended = m.StartAt
	}
	_, err = tx.Exec(ctx, `UPDATE matches.holds SET expires_at=$2,transfer_reported_at=COALESCE(transfer_reported_at,$3),updated_at=$3 WHERE id=$1`, hold.ID, extended, now)
	if err != nil {
		return Participation{}, err
	}
	hold.ExpiresAt = extended
	if hold.TransferReportedAt == nil {
		reported := now
		hold.TransferReportedAt = &reported
	}
	p.Hold = &hold
	if err = r.refresh(ctx, tx, m.ID, m.Capacity, now); err != nil {
		return Participation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Participation{}, err
	}
	return p, nil
}

func (r *PostgresRepository) ConfirmPayment(ctx context.Context, host, participationID string, now time.Time) (Participation, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Participation{}, err
	}
	defer tx.Rollback(context.Background())
	var p Participation
	err = tx.QueryRow(ctx, `SELECT match_id FROM matches.participations WHERE id=$1`, participationID).Scan(&p.MatchID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Participation{}, ErrNotFound
	}
	if err != nil {
		return Participation{}, err
	}
	m, err := r.get(ctx, tx, p.MatchID, false, true)
	if err != nil {
		return Participation{}, err
	}
	if !m.StartAt.After(now) {
		return Participation{}, ErrNotOpen
	}
	if m.HostID != host {
		return Participation{}, ErrForbidden
	}
	err = scanPart(tx.QueryRow(ctx, `SELECT id,match_id,player_id,status,decision_reason,created_at,updated_at FROM matches.participations WHERE id=$1 FOR UPDATE`, participationID), &p)
	if err != nil {
		return Participation{}, err
	}
	if p.Status == ParticipationJoined {
		if err = tx.Commit(ctx); err != nil {
			return Participation{}, err
		}
		return p, nil
	}
	var hold Hold
	err = tx.QueryRow(ctx, `SELECT id,participation_id,status,expires_at,transfer_reported_at FROM matches.holds WHERE participation_id=$1 FOR UPDATE`, p.ID).Scan(&hold.ID, &hold.ParticipationID, &hold.Status, &hold.ExpiresAt, &hold.TransferReportedAt)
	if err != nil {
		return Participation{}, ErrHoldExpired
	}
	revivableExpiryRace := p.Status == ParticipationExpired && hold.Status == HoldExpired && hold.ExpiresAt.After(now)
	valid := (p.Status == ParticipationAwaitingPayment && hold.Status == HoldHeld && hold.ExpiresAt.After(now)) || revivableExpiryRace
	if !valid || !m.StartAt.After(now) || (m.Status != StatusOpen && m.Status != StatusFull) {
		if p.Status == ParticipationAwaitingPayment && hold.Status == HoldHeld {
			if _, err = tx.Exec(ctx, `UPDATE matches.holds SET status='EXPIRED',released_at=$2,release_reason='CONFIRMATION_DEADLINE',updated_at=$2 WHERE id=$1`, hold.ID, now); err != nil {
				return Participation{}, err
			}
			if _, err = tx.Exec(ctx, `UPDATE matches.participations SET status='EXPIRED',updated_at=$2 WHERE id=$1`, p.ID, now); err != nil {
				return Participation{}, err
			}
			if err = r.refresh(ctx, tx, m.ID, m.Capacity, now); err != nil {
				return Participation{}, err
			}
			if err = tx.Commit(ctx); err != nil {
				return Participation{}, err
			}
		}
		return Participation{}, ErrHoldExpired
	}
	if err = r.reserve(ctx, tx, p.PlayerID, m, now, p.ID); err != nil {
		return Participation{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE matches.holds SET status='CONSUMED',consumed_at=$2,updated_at=$2 WHERE id=$1`, hold.ID, now); err != nil {
		return Participation{}, err
	}
	p.Status = ParticipationJoined
	p.UpdatedAt = now
	if _, err = tx.Exec(ctx, `UPDATE matches.participations SET status='JOINED',updated_at=$2 WHERE id=$1`, p.ID, now); err != nil {
		return Participation{}, err
	}
	if err = r.refresh(ctx, tx, m.ID, m.Capacity, now); err != nil {
		return Participation{}, err
	}
	if err = enqueueJoined(ctx, tx, p, m, now); err != nil {
		return Participation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Participation{}, err
	}
	return p, nil
}

func (r *PostgresRepository) ExpireDueHolds(ctx context.Context, now time.Time, limit int) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(context.Background())
	rows, err := tx.Query(ctx, `SELECT h.id,h.participation_id,h.match_id FROM matches.holds h WHERE h.status='HELD' AND h.expires_at<=$1 ORDER BY h.expires_at FOR UPDATE SKIP LOCKED LIMIT $2`, now, limit)
	if err != nil {
		return 0, err
	}
	type due struct{ holdID, participationID, matchID string }
	var items []due
	for rows.Next() {
		var item due
		if err = rows.Scan(&item.holdID, &item.participationID, &item.matchID); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	affected := make(map[string]struct{})
	for _, item := range items {
		if _, err = tx.Exec(ctx, `UPDATE matches.holds SET status='EXPIRED',released_at=$2,release_reason='DEADLINE',updated_at=$2 WHERE id=$1`, item.holdID, now); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, `UPDATE matches.participations SET status='EXPIRED',updated_at=$2 WHERE id=$1 AND status='AWAITING_PAYMENT'`, item.participationID, now); err != nil {
			return 0, err
		}
		affected[item.matchID] = struct{}{}
	}
	for matchID := range affected {
		var capacity int
		if err = tx.QueryRow(ctx, `SELECT capacity FROM matches.matches WHERE id=$1 FOR UPDATE`, matchID).Scan(&capacity); err != nil {
			return 0, err
		}
		if err = r.refresh(ctx, tx, matchID, capacity, now); err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(items), nil
}

func (r *PostgresRepository) CancelParticipation(ctx context.Context, player, participationID, key, reason string, now time.Time) (Participation, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Participation{}, err
	}
	defer tx.Rollback(context.Background())
	const action = "cancel-participation"
	if err = lockIdempotency(ctx, tx, player, action, key); err != nil {
		return Participation{}, err
	}
	var storedFingerprint, resourceID string
	err = tx.QueryRow(ctx, `SELECT fingerprint,resource_id FROM matches.command_results WHERE actor_id=$1 AND action=$2 AND idempotency_key=$3`, player, action, key).Scan(&storedFingerprint, &resourceID)
	if err == nil {
		if storedFingerprint != participationID {
			return Participation{}, ErrIdempotencyConflict
		}
		var p Participation
		if err = scanPart(tx.QueryRow(ctx, `SELECT id,match_id,player_id,status,decision_reason,created_at,updated_at FROM matches.participations WHERE id=$1`, resourceID), &p); err != nil {
			return Participation{}, err
		}
		return p, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Participation{}, err
	}
	var p Participation
	if err = tx.QueryRow(ctx, `SELECT match_id FROM matches.participations WHERE id=$1`, participationID).Scan(&p.MatchID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Participation{}, ErrNotFound
		}
		return Participation{}, err
	}
	m, err := r.get(ctx, tx, p.MatchID, false, true)
	if err != nil {
		return Participation{}, err
	}
	if !m.StartAt.After(now) {
		return Participation{}, ErrNotOpen
	}
	if err = scanPart(tx.QueryRow(ctx, `SELECT id,match_id,player_id,status,decision_reason,created_at,updated_at FROM matches.participations WHERE id=$1 FOR UPDATE`, participationID), &p); err != nil {
		return Participation{}, err
	}
	if p.PlayerID != player {
		return Participation{}, ErrForbidden
	}
	if p.Status != ParticipationRequested && p.Status != ParticipationAwaitingPayment && p.Status != ParticipationJoined {
		return Participation{}, ErrInvalid
	}
	previous := p.Status
	p.Status = ParticipationCancelled
	p.DecisionReason = reason
	p.UpdatedAt = now
	if _, err = tx.Exec(ctx, `UPDATE matches.participations SET status='CANCELLED',decision_reason=$2,updated_at=$3 WHERE id=$1`, p.ID, reason, now); err != nil {
		return Participation{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE matches.holds SET status='RELEASED',released_at=$2,release_reason='PLAYER_WITHDRAWAL',updated_at=$2 WHERE participation_id=$1 AND status='HELD'`, p.ID, now); err != nil {
		return Participation{}, err
	}
	refund := "NONE"
	if (previous == ParticipationJoined || previous == ParticipationAwaitingPayment) && !now.After(m.StartAt.Add(-6*time.Hour)) {
		refund = "FULL"
	}
	if previous == ParticipationJoined || previous == ParticipationAwaitingPayment {
		if err = enqueueCancellation(ctx, tx, m, p, player, "PLAYER_WITHDRAWAL", reason, refund, now); err != nil {
			return Participation{}, err
		}
	}
	if err = r.refresh(ctx, tx, m.ID, m.Capacity, now); err != nil {
		return Participation{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO matches.command_results(actor_id,action,idempotency_key,fingerprint,resource_type,resource_id,created_at) VALUES($1,$2,$3,$4,'participation',$5,$6)`, player, action, key, participationID, participationID, now)
	if err != nil {
		return Participation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Participation{}, err
	}
	return p, nil
}

func (r *PostgresRepository) CancelMatch(ctx context.Context, host, matchID, key, reason string, now time.Time) (Match, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Match{}, err
	}
	defer tx.Rollback(context.Background())
	const action = "cancel-match"
	if err = lockIdempotency(ctx, tx, host, action, key); err != nil {
		return Match{}, err
	}
	var storedFingerprint, resourceID string
	err = tx.QueryRow(ctx, `SELECT fingerprint,resource_id FROM matches.command_results WHERE actor_id=$1 AND action=$2 AND idempotency_key=$3`, host, action, key).Scan(&storedFingerprint, &resourceID)
	if err == nil {
		if storedFingerprint != matchID {
			return Match{}, ErrIdempotencyConflict
		}
		m, getErr := r.get(ctx, tx, resourceID, false, false)
		if getErr != nil {
			return Match{}, getErr
		}
		return m, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Match{}, err
	}
	m, err := r.get(ctx, tx, matchID, false, true)
	if err != nil {
		return Match{}, err
	}
	if m.HostID != host || (m.Status != StatusDraft && m.Status != StatusOpen && m.Status != StatusFull) {
		return Match{}, ErrForbidden
	}
	if m.Status != StatusDraft && !m.StartAt.After(now) {
		return Match{}, ErrNotOpen
	}
	rows, err := tx.Query(ctx, `SELECT id,match_id,player_id,status,decision_reason,created_at,updated_at FROM matches.participations WHERE match_id=$1 AND status IN('REQUESTED','AWAITING_PAYMENT','JOINED') FOR UPDATE`, matchID)
	if err != nil {
		return Match{}, err
	}
	var participants []Participation
	for rows.Next() {
		var p Participation
		if err = rows.Scan(&p.ID, &p.MatchID, &p.PlayerID, &p.Status, &p.DecisionReason, &p.CreatedAt, &p.UpdatedAt); err != nil {
			rows.Close()
			return Match{}, err
		}
		participants = append(participants, p)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return Match{}, err
	}
	rows.Close()
	if _, err = tx.Exec(ctx, `UPDATE matches.participations SET status='CANCELLED',decision_reason=$2,updated_at=$3 WHERE match_id=$1 AND status IN('REQUESTED','AWAITING_PAYMENT','JOINED')`, matchID, reason, now); err != nil {
		return Match{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE matches.holds SET status='RELEASED',released_at=$2,release_reason='HOST_MATCH_CANCELLATION',updated_at=$2 WHERE match_id=$1 AND status='HELD'`, matchID, now); err != nil {
		return Match{}, err
	}
	for _, p := range participants {
		if p.Status == ParticipationAwaitingPayment || p.Status == ParticipationJoined {
			if err = enqueueCancellation(ctx, tx, m, p, host, "HOST_MATCH_CANCELLATION", reason, "FULL", now); err != nil {
				return Match{}, err
			}
		}
	}
	m.Status = StatusCancelled
	m.UpdatedAt = now
	if _, err = tx.Exec(ctx, `UPDATE matches.matches SET status='CANCELLED',cancelled_at=$2,cancelled_by=$3,cancellation_reason=$4,updated_at=$2 WHERE id=$1`, matchID, now, host, reason); err != nil {
		return Match{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO matches.command_results(actor_id,action,idempotency_key,fingerprint,resource_type,resource_id,created_at) VALUES($1,$2,$3,$4,'match',$5,$6)`, host, action, key, matchID, matchID, now)
	if err != nil {
		return Match{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Match{}, err
	}
	return m, nil
}

func (r *PostgresRepository) UpdatePublished(ctx context.Context, host, matchID, key, description string, capacity int, now time.Time) (Match, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Match{}, err
	}
	defer tx.Rollback(context.Background())
	const action = "update-published"
	if err = lockIdempotency(ctx, tx, host, action, key); err != nil {
		return Match{}, err
	}
	fingerprint := fmt.Sprintf("%s:%d:%s", matchID, capacity, description)
	var storedFingerprint, resourceID string
	err = tx.QueryRow(ctx, `SELECT fingerprint,resource_id FROM matches.command_results WHERE actor_id=$1 AND action=$2 AND idempotency_key=$3`, host, action, key).Scan(&storedFingerprint, &resourceID)
	if err == nil {
		if storedFingerprint != fingerprint {
			return Match{}, ErrIdempotencyConflict
		}
		m, getErr := r.get(ctx, tx, resourceID, false, false)
		if getErr != nil {
			return Match{}, getErr
		}
		return m, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Match{}, err
	}
	m, err := r.get(ctx, tx, matchID, false, true)
	if err != nil {
		return Match{}, err
	}
	if m.HostID != host || (m.Status != StatusOpen && m.Status != StatusFull) {
		return Match{}, ErrForbidden
	}
	var occupied int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM matches.participations p LEFT JOIN matches.holds h ON h.participation_id=p.id WHERE p.match_id=$1 AND (p.status='JOINED' OR (p.status='AWAITING_PAYMENT' AND h.status='HELD' AND h.expires_at>$2))`, matchID, now).Scan(&occupied); err != nil {
		return Match{}, err
	}
	if capacity < occupied {
		return Match{}, ErrCapacityBelowOccupied
	}
	m.Description, m.Capacity, m.UpdatedAt = description, capacity, now
	if _, err = tx.Exec(ctx, `UPDATE matches.matches SET description=$2,capacity=$3,updated_at=$4 WHERE id=$1`, matchID, description, capacity, now); err != nil {
		return Match{}, err
	}
	if err = r.refresh(ctx, tx, matchID, capacity, now); err != nil {
		return Match{}, err
	}
	if capacity <= occupied {
		m.Status = StatusFull
	} else {
		m.Status = StatusOpen
	}
	if _, err = tx.Exec(ctx, `INSERT INTO matches.command_results(actor_id,action,idempotency_key,fingerprint,resource_type,resource_id,created_at) VALUES($1,$2,$3,$4,'match',$5,$6)`, host, action, key, fingerprint, matchID, now); err != nil {
		return Match{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Match{}, err
	}
	return m, nil
}

func lockIdempotency(ctx context.Context, tx pgx.Tx, actor, action, key string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,hashtextextended($2,hashtextextended($3,0))))`, actor, action, key)
	return err
}
func (r *PostgresRepository) reserve(c context.Context, tx pgx.Tx, player string, m Match, now time.Time, excludeParticipation string) error {
	if _, e := tx.Exec(c, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, player); e != nil {
		return e
	}
	var conflict bool
	e := tx.QueryRow(c, `SELECT EXISTS(
        SELECT 1 FROM matches.participations p
        LEFT JOIN matches.holds h ON h.participation_id=p.id
        WHERE p.player_id=$1 AND p.id IS DISTINCT FROM NULLIF($4,'')::uuid
          AND p.start_at < $3 AND p.end_at > $2
          AND (p.status='JOINED' OR (p.status='AWAITING_PAYMENT' AND h.status='HELD' AND h.expires_at>$5))
    )`, player, m.StartAt, m.EndAt, excludeParticipation, now).Scan(&conflict)
	if e != nil {
		return e
	}
	if conflict {
		return ErrScheduleConflict
	}
	var occupied int
	if e = tx.QueryRow(c, `SELECT count(*) FROM matches.participations p LEFT JOIN matches.holds h ON h.participation_id=p.id WHERE p.match_id=$1 AND p.id IS DISTINCT FROM NULLIF($2,'')::uuid AND (p.status='JOINED' OR (p.status='AWAITING_PAYMENT' AND h.status='HELD' AND h.expires_at>$3))`, m.ID, excludeParticipation, now).Scan(&occupied); e != nil {
		return e
	}
	if occupied >= m.Capacity {
		return ErrFull
	}
	return nil
}
func (r *PostgresRepository) refresh(c context.Context, tx pgx.Tx, mid string, cap int, now time.Time) error {
	var n int
	if e := tx.QueryRow(c, `SELECT count(*) FROM matches.participations p LEFT JOIN matches.holds h ON h.participation_id=p.id WHERE p.match_id=$1 AND (p.status='JOINED' OR (p.status='AWAITING_PAYMENT' AND h.status='HELD' AND h.expires_at>$2))`, mid, now).Scan(&n); e != nil {
		return e
	}
	s := StatusOpen
	if n >= cap {
		s = StatusFull
	}
	_, e := tx.Exec(c, `UPDATE matches.matches SET status=$2 WHERE id=$1 AND status IN('OPEN','FULL')`, mid, s)
	return e
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (r *PostgresRepository) get(c context.Context, q rowQuerier, mid string, public, lock bool) (Match, error) {
	sql := `SELECT m.id,m.host_id,m.title,m.description,m.venue_id,m.venue_name,m.venue_address,m.venue_area,m.venue_time_zone,m.court,m.start_at,m.end_at,m.format,m.style,m.rules,m.min_level,m.max_level,m.capacity,m.fee_minor,m.currency,m.join_mode,m.host_plays,m.court_attested,m.court_attested_at,m.status,m.created_at,m.updated_at,m.deposit_minor,m.payment_recipient,m.payment_instructions,m.payment_instruction_version,m.cancellation_policy_version,(SELECT count(*) FROM matches.participations p LEFT JOIN matches.holds h ON h.participation_id=p.id WHERE p.match_id=m.id AND (p.status='JOINED' OR (p.status='AWAITING_PAYMENT' AND h.status='HELD' AND h.expires_at>now()))) FROM matches.matches m WHERE m.id=$1`
	if public {
		sql += ` AND m.status IN('OPEN','FULL')`
	}
	if lock {
		sql += ` FOR UPDATE OF m`
	}
	var m Match
	e := q.QueryRow(c, sql, mid).Scan(&m.ID, &m.HostID, &m.Title, &m.Description, &m.Venue.ID, &m.Venue.Name, &m.Venue.Address, &m.Venue.Area, &m.Venue.TimeZone, &m.Venue.Court, &m.StartAt, &m.EndAt, &m.Format, &m.Style, &m.Rules, &m.MinLevel, &m.MaxLevel, &m.Capacity, &m.FeeMinor, &m.Currency, &m.JoinMode, &m.HostPlays, &m.CourtAttested, &m.CourtAttestedAt, &m.Status, &m.CreatedAt, &m.UpdatedAt, &m.DepositMinor, &m.PaymentRecipient, &m.PaymentInstructions, &m.PaymentInstructionVersion, &m.CancellationPolicyVersion, &m.Occupied)
	if errors.Is(e, pgx.ErrNoRows) {
		if public {
			return Match{}, ErrNotOpen
		}
		return Match{}, ErrNotFound
	}
	return m, e
}
func (r *PostgresRepository) idempotent(c context.Context, tx pgx.Tx, a, action, key, fingerprint string) (Participation, error) {
	var p Participation
	var stored string
	e := tx.QueryRow(c, `SELECT fingerprint FROM matches.idempotency WHERE actor_id=$1 AND action=$2 AND idempotency_key=$3`, a, action, key).Scan(&stored)
	if e != nil {
		return Participation{}, e
	}
	if stored != fingerprint {
		return Participation{}, ErrIdempotencyConflict
	}
	e = scanPart(tx.QueryRow(c, `SELECT p.id,p.match_id,p.player_id,p.status,p.decision_reason,p.created_at,p.updated_at FROM matches.idempotency i JOIN matches.participations p ON p.id=i.participation_id WHERE i.actor_id=$1 AND i.action=$2 AND i.idempotency_key=$3`, a, action, key), &p)
	if e == nil {
		e = r.loadHold(c, tx, &p)
	}
	return p, e
}
func scanPart(row pgx.Row, p *Participation) error {
	return row.Scan(&p.ID, &p.MatchID, &p.PlayerID, &p.Status, &p.DecisionReason, &p.CreatedAt, &p.UpdatedAt)
}
func mapConflict(e error) error {
	var pe *pgconn.PgError
	if errors.As(e, &pe) && pe.Code == "23505" {
		return ErrAlreadyParticipating
	}
	return e
}
