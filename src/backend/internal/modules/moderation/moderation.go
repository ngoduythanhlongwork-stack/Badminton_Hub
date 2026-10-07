package moderation

import (
	"badmintonhub/internal/platform/clock"
	"badmintonhub/internal/platform/id"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"time"
)

var (
	ErrInvalid   = errors.New("invalid moderation command")
	ErrForbidden = errors.New("moderation action forbidden")
	ErrNotFound  = errors.New("moderation case not found")
	ErrConflict  = errors.New("moderation conflict")
)

type Case struct {
	ID            string    `json:"id"`
	SubjectType   string    `json:"subjectType"`
	SubjectID     string    `json:"subjectId"`
	ReporterID    string    `json:"reporterId"`
	TargetActorID string    `json:"targetActorId,omitempty"`
	Reason        string    `json:"reason"`
	Description   string    `json:"description"`
	EvidenceRef   string    `json:"evidenceRef,omitempty"`
	Status        string    `json:"status"`
	LinkedCaseID  string    `json:"linkedCaseId,omitempty"`
	AssigneeID    string    `json:"assigneeId,omitempty"`
	Decision      string    `json:"decision,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}
type Service struct {
	pool  *pgxpool.Pool
	clock clock.Clock
}

func NewService(pool *pgxpool.Pool, c clock.Clock) (*Service, error) {
	if pool == nil {
		return nil, ErrInvalid
	}
	if c == nil {
		c = clock.System{}
	}
	return &Service{pool: pool, clock: c}, nil
}
func (s *Service) Block(ctx context.Context, actor, target string) error {
	if actor == "" || target == "" || actor == target {
		return ErrInvalid
	}
	now := s.clock.Now().UTC()
	_, e := s.pool.Exec(ctx, `INSERT INTO moderation.blocks(blocker_id,blocked_id,active,created_at,updated_at) VALUES($1,$2,true,$3,$3) ON CONFLICT(blocker_id,blocked_id) DO UPDATE SET active=true,updated_at=$3`, actor, target, now)
	return e
}
func (s *Service) Unblock(ctx context.Context, actor, target string) error {
	tag, e := s.pool.Exec(ctx, `UPDATE moderation.blocks SET active=false,updated_at=$3 WHERE blocker_id=$1 AND blocked_id=$2`, actor, target, s.clock.Now().UTC())
	if e == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return e
}
func (s *Service) Blocked(ctx context.Context, a, b string) (bool, error) {
	var blocked bool
	e := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM moderation.blocks WHERE active AND ((blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1)))`, a, b).Scan(&blocked)
	return blocked, e
}
func (s *Service) Report(ctx context.Context, c Case) (Case, error) {
	c.SubjectType = strings.ToUpper(strings.TrimSpace(c.SubjectType))
	c.Reason = strings.ToUpper(strings.TrimSpace(c.Reason))
	c.Description = strings.TrimSpace(c.Description)
	if c.ReporterID == "" || c.SubjectID == "" || c.Description == "" || !oneOf(c.SubjectType, "USER", "MATCH", "MESSAGE", "PAYMENT", "ATTENDANCE", "REVIEW") || !oneOf(c.Reason, "SPAM", "FRAUD", "HARASSMENT", "FAKE_SKILL", "NO_SHOW", "OTHER") {
		return Case{}, ErrInvalid
	}
	c.ID, _ = id.New()
	c.Status = "SUBMITTED"
	c.CreatedAt = s.clock.Now().UTC()
	c.UpdatedAt = c.CreatedAt
	_ = s.pool.QueryRow(ctx, `SELECT id::text FROM moderation.cases WHERE subject_type=$1 AND subject_id=$2 AND status NOT IN('RESOLVED') ORDER BY created_at LIMIT 1`, c.SubjectType, c.SubjectID).Scan(&c.LinkedCaseID)
	_, e := s.pool.Exec(ctx, `INSERT INTO moderation.cases(id,subject_type,subject_id,reporter_id,target_actor_id,reason,description,evidence_ref,status,linked_case_id,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11)`, c.ID, c.SubjectType, c.SubjectID, c.ReporterID, null(c.TargetActorID), c.Reason, c.Description, null(c.EvidenceRef), c.Status, null(c.LinkedCaseID), c.CreatedAt)
	return c, e
}

func (s *Service) AuthorizeOwnerAction(ctx context.Context, caseID, admin, subjectType, subjectID string) error {
	c, e := caseByID(ctx, s.pool, caseID, false)
	if e != nil {
		return e
	}
	if c.AssigneeID != admin || c.Status != "DECIDED" || c.SubjectType != subjectType || c.SubjectID != subjectID {
		return ErrForbidden
	}
	return nil
}
func (s *Service) ResolveOwnerAction(ctx context.Context, caseID, admin, result string) error {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(context.Background())
	c, e := caseByID(ctx, tx, caseID, true)
	if e != nil {
		return e
	}
	if c.AssigneeID != admin || c.Status != "DECIDED" {
		return ErrForbidden
	}
	before := c.Status
	c.Status = "RESOLVED"
	c.UpdatedAt = s.clock.Now().UTC()
	if _, e = tx.Exec(ctx, `UPDATE moderation.cases SET status='RESOLVED',resolved_at=$2,updated_at=$2 WHERE id=$1`, caseID, c.UpdatedAt); e != nil {
		return e
	}
	if e = audit(ctx, tx, c, admin, "OWNER_ACTION", before, c.Status, result); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (s *Service) Assign(ctx context.Context, caseID, admin string) (Case, error) {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return Case{}, e
	}
	defer tx.Rollback(context.Background())
	c, e := caseByID(ctx, tx, caseID, true)
	if e != nil {
		return Case{}, e
	}
	if admin == c.ReporterID || admin == c.TargetActorID {
		return Case{}, ErrConflict
	}
	before := c.Status
	c.Status = "IN_REVIEW"
	c.AssigneeID = admin
	c.UpdatedAt = s.clock.Now().UTC()
	if _, e = tx.Exec(ctx, `UPDATE moderation.cases SET assignee_id=$2,status=$3,updated_at=$4 WHERE id=$1`, caseID, admin, c.Status, c.UpdatedAt); e != nil {
		return Case{}, e
	}
	if e = audit(ctx, tx, c, admin, "ASSIGN", before, c.Status, "case assigned"); e != nil {
		return Case{}, e
	}
	e = tx.Commit(ctx)
	return c, e
}
func (s *Service) Decide(ctx context.Context, caseID, admin, decision string, resolved bool) (Case, error) {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return Case{}, e
	}
	defer tx.Rollback(context.Background())
	c, e := caseByID(ctx, tx, caseID, true)
	if e != nil {
		return Case{}, e
	}
	if c.AssigneeID != admin || admin == c.ReporterID || admin == c.TargetActorID {
		return Case{}, ErrForbidden
	}
	decision = strings.TrimSpace(decision)
	if decision == "" {
		return Case{}, ErrInvalid
	}
	before := c.Status
	c.Decision = decision
	if resolved {
		c.Status = "RESOLVED"
	} else {
		c.Status = "DECIDED"
	}
	c.UpdatedAt = s.clock.Now().UTC()
	if resolved {
		_, e = tx.Exec(ctx, `UPDATE moderation.cases SET status='RESOLVED',decision=$2,updated_at=$3,resolved_at=$3 WHERE id=$1`, caseID, decision, c.UpdatedAt)
	} else {
		_, e = tx.Exec(ctx, `UPDATE moderation.cases SET status='DECIDED',decision=$2,updated_at=$3,resolved_at=NULL WHERE id=$1`, caseID, decision, c.UpdatedAt)
	}
	if e != nil {
		return Case{}, e
	}
	if e = audit(ctx, tx, c, admin, "DECIDE", before, c.Status, decision); e != nil {
		return Case{}, e
	}
	e = tx.Commit(ctx)
	return c, e
}

func (s *Service) Appeal(ctx context.Context, caseID, actor, reason string) (Case, error) {
	reason = strings.TrimSpace(reason)
	if caseID == "" || actor == "" || reason == "" {
		return Case{}, ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Case{}, err
	}
	defer tx.Rollback(context.Background())
	c, err := caseByID(ctx, tx, caseID, true)
	if err != nil {
		return Case{}, err
	}
	now := s.clock.Now().UTC()
	if actor != c.ReporterID && actor != c.TargetActorID {
		return Case{}, ErrForbidden
	}
	var appealed bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM moderation.case_appeals WHERE case_id=$1)`, caseID).Scan(&appealed); err != nil {
		return Case{}, err
	}
	if appealed {
		return Case{}, ErrConflict
	}
	if c.Status != "DECIDED" && c.Status != "RESOLVED" {
		return Case{}, ErrForbidden
	}
	if now.After(c.UpdatedAt.Add(7 * 24 * time.Hour)) {
		return Case{}, ErrConflict
	}
	appealID, _ := id.New()
	if _, err = tx.Exec(ctx, `INSERT INTO moderation.case_appeals(id,case_id,appellant_id,reason,created_at) VALUES($1,$2,$3,$4,$5)`, appealID, caseID, actor, reason, now); err != nil {
		var pgErr interface{ SQLState() string }
		if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
			return Case{}, ErrConflict
		}
		return Case{}, err
	}
	before := c.Status
	c.Status = "REOPENED"
	c.UpdatedAt = now
	if _, err = tx.Exec(ctx, `UPDATE moderation.cases SET status='REOPENED',assignee_id=NULL,resolved_at=NULL,updated_at=$2 WHERE id=$1`, caseID, now); err != nil {
		return Case{}, err
	}
	if err = audit(ctx, tx, c, actor, "APPEAL", before, c.Status, reason); err != nil {
		return Case{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Case{}, err
	}
	c.AssigneeID = ""
	return c, nil
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func caseByID(ctx context.Context, q rowQuerier, caseID string, lock bool) (Case, error) {
	sql := `SELECT id,subject_type,subject_id,reporter_id,coalesce(target_actor_id::text,''),reason,description,coalesce(evidence_ref,''),status,coalesce(linked_case_id::text,''),coalesce(assignee_id::text,''),decision,created_at,updated_at FROM moderation.cases WHERE id=$1`
	if lock {
		sql += " FOR UPDATE"
	}
	var c Case
	e := q.QueryRow(ctx, sql, caseID).Scan(&c.ID, &c.SubjectType, &c.SubjectID, &c.ReporterID, &c.TargetActorID, &c.Reason, &c.Description, &c.EvidenceRef, &c.Status, &c.LinkedCaseID, &c.AssigneeID, &c.Decision, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(e, pgx.ErrNoRows) {
		e = ErrNotFound
	}
	return c, e
}
func audit(ctx context.Context, tx pgx.Tx, c Case, actor, action, before, after, reason string) error {
	auditID, _ := id.New()
	_, e := tx.Exec(ctx, `INSERT INTO moderation.case_audit(id,case_id,actor_id,action,before_state,after_state,reason,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, auditID, c.ID, actor, action, before, after, reason, c.UpdatedAt)
	return e
}
func oneOf(v string, values ...string) bool {
	for _, x := range values {
		if v == x {
			return true
		}
	}
	return false
}
func null(v string) any {
	if v == "" {
		return nil
	}
	return v
}
