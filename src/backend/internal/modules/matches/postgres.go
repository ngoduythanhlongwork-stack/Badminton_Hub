package matches

import (
	"badmintonhub/internal/platform/id"
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

const insertMatch = `INSERT INTO matches.matches(id,host_id,title,description,venue_id,venue_name,venue_address,venue_area,venue_time_zone,court,start_at,end_at,format,style,rules,min_level,max_level,capacity,fee_minor,currency,join_mode,host_plays,court_attested,court_attested_at,status,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27)`

func (r *PostgresRepository) Create(c context.Context, m Match) (Match, error) {
	_, e := r.pool.Exec(c, insertMatch, m.ID, m.HostID, m.Title, m.Description, m.Venue.ID, m.Venue.Name, m.Venue.Address, m.Venue.Area, m.Venue.TimeZone, m.Venue.Court, m.StartAt, m.EndAt, m.Format, m.Style, m.Rules, m.MinLevel, m.MaxLevel, m.Capacity, m.FeeMinor, m.Currency, m.JoinMode, m.HostPlays, m.CourtAttested, m.CourtAttestedAt, m.Status, m.CreatedAt, m.UpdatedAt)
	return m, e
}
func (r *PostgresRepository) UpdateDraft(c context.Context, m Match) (Match, error) {
	tag, e := r.pool.Exec(c, `UPDATE matches.matches SET title=$3,description=$4,venue_id=$5,venue_name=$6,venue_address=$7,venue_area=$8,venue_time_zone=$9,court=$10,start_at=$11,end_at=$12,format=$13,style=$14,rules=$15,min_level=$16,max_level=$17,capacity=$18,fee_minor=$19,currency=$20,join_mode=$21,host_plays=$22,court_attested=$23,court_attested_at=$24,updated_at=$25 WHERE id=$1 AND host_id=$2 AND status='DRAFT'`, m.ID, m.HostID, m.Title, m.Description, m.Venue.ID, m.Venue.Name, m.Venue.Address, m.Venue.Area, m.Venue.TimeZone, m.Venue.Court, m.StartAt, m.EndAt, m.Format, m.Style, m.Rules, m.MinLevel, m.MaxLevel, m.Capacity, m.FeeMinor, m.Currency, m.JoinMode, m.HostPlays, m.CourtAttested, m.CourtAttestedAt, m.UpdatedAt)
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
		if e = r.reserve(c, tx, host, m); e != nil {
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
	e = scanPart(tx.QueryRow(c, `SELECT id,match_id,player_id,status,decision_reason,created_at,updated_at FROM matches.participations WHERE match_id=$1 AND player_id=$2 AND status IN('REQUESTED','JOINED')`, mid, player), &existing)
	if e == nil {
		return Participation{}, ErrAlreadyParticipating
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return Participation{}, e
	}
	status := ParticipationRequested
	if m.JoinMode == JoinInstant {
		status = ParticipationJoined
		if e = r.reserve(c, tx, player, m); e != nil {
			return Participation{}, e
		}
	}
	pid, _ := id.New()
	p := Participation{ID: pid, MatchID: mid, PlayerID: player, Status: status, CreatedAt: now, UpdatedAt: now}
	_, e = tx.Exec(c, `INSERT INTO matches.participations(id,match_id,player_id,status,start_at,end_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$7)`, pid, mid, player, status, m.StartAt, m.EndAt, now)
	if e != nil {
		return Participation{}, mapConflict(e)
	}
	if status == ParticipationJoined {
		if e = r.refresh(c, tx, mid, m.Capacity); e != nil {
			return Participation{}, e
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
		if e = r.reserve(c, tx, p.PlayerID, m); e != nil {
			return Participation{}, e
		}
	} else if target == ParticipationRejected && p.Status != ParticipationRequested {
		return Participation{}, ErrInvalid
	} else if target == ParticipationRemoved && p.Status != ParticipationJoined {
		return Participation{}, ErrInvalid
	}
	p.Status = target
	p.DecisionReason = reason
	p.UpdatedAt = now
	_, e = tx.Exec(c, `UPDATE matches.participations SET status=$2,decision_reason=$3,updated_at=$4 WHERE id=$1`, pid, target, reason, now)
	if e != nil {
		return Participation{}, e
	}
	if e = r.refresh(c, tx, mid, m.Capacity); e != nil {
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

func lockIdempotency(ctx context.Context, tx pgx.Tx, actor, action, key string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,hashtextextended($2,hashtextextended($3,0))))`, actor, action, key)
	return err
}
func (r *PostgresRepository) reserve(c context.Context, tx pgx.Tx, player string, m Match) error {
	if _, e := tx.Exec(c, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, player); e != nil {
		return e
	}
	var conflict bool
	e := tx.QueryRow(c, `SELECT EXISTS(SELECT 1 FROM matches.participations WHERE player_id=$1 AND status='JOINED' AND start_at < $3 AND end_at > $2)`, player, m.StartAt, m.EndAt).Scan(&conflict)
	if e != nil {
		return e
	}
	if conflict {
		return ErrScheduleConflict
	}
	var occupied int
	if e = tx.QueryRow(c, `SELECT count(*) FROM matches.participations WHERE match_id=$1 AND status='JOINED'`, m.ID).Scan(&occupied); e != nil {
		return e
	}
	if occupied >= m.Capacity {
		return ErrFull
	}
	return nil
}
func (r *PostgresRepository) refresh(c context.Context, tx pgx.Tx, mid string, cap int) error {
	var n int
	if e := tx.QueryRow(c, `SELECT count(*) FROM matches.participations WHERE match_id=$1 AND status='JOINED'`, mid).Scan(&n); e != nil {
		return e
	}
	s := StatusOpen
	if n >= cap {
		s = StatusFull
	}
	_, e := tx.Exec(c, `UPDATE matches.matches SET status=$2 WHERE id=$1`, mid, s)
	return e
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (r *PostgresRepository) get(c context.Context, q rowQuerier, mid string, public, lock bool) (Match, error) {
	sql := `SELECT m.id,m.host_id,m.title,m.description,m.venue_id,m.venue_name,m.venue_address,m.venue_area,m.venue_time_zone,m.court,m.start_at,m.end_at,m.format,m.style,m.rules,m.min_level,m.max_level,m.capacity,m.fee_minor,m.currency,m.join_mode,m.host_plays,m.court_attested,m.court_attested_at,m.status,m.created_at,m.updated_at,(SELECT count(*) FROM matches.participations p WHERE p.match_id=m.id AND p.status='JOINED') FROM matches.matches m WHERE m.id=$1`
	if public {
		sql += ` AND m.status IN('OPEN','FULL')`
	}
	if lock {
		sql += ` FOR UPDATE OF m`
	}
	var m Match
	e := q.QueryRow(c, sql, mid).Scan(&m.ID, &m.HostID, &m.Title, &m.Description, &m.Venue.ID, &m.Venue.Name, &m.Venue.Address, &m.Venue.Area, &m.Venue.TimeZone, &m.Venue.Court, &m.StartAt, &m.EndAt, &m.Format, &m.Style, &m.Rules, &m.MinLevel, &m.MaxLevel, &m.Capacity, &m.FeeMinor, &m.Currency, &m.JoinMode, &m.HostPlays, &m.CourtAttested, &m.CourtAttestedAt, &m.Status, &m.CreatedAt, &m.UpdatedAt, &m.Occupied)
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
