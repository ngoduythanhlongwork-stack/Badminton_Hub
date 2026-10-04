package postgres

import (
	"context"
	"errors"
	"time"

	"badmintonhub/internal/modules/identity/internal/application"
	"badmintonhub/internal/platform/id"
	"github.com/jackc/pgx/v5"
)

func (s *Store) HasPermission(ctx context.Context, q application.PermissionQuery, _ time.Time) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM identity.permission_assignments WHERE account_id=$1 AND permission=$2 AND scope_type=$3 AND (scope_id=$4 OR scope_id='') AND revoked_at IS NULL)`, q.AccountID, q.Permission, q.ScopeType, q.ScopeID).Scan(&exists)
	return exists, err
}

func (s *Store) GrantPermission(ctx context.Context, g application.PermissionGrant) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	_, err = tx.Exec(ctx, `INSERT INTO identity.permission_assignments(id,account_id,permission,scope_type,scope_id,granted_by,rationale,granted_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, g.ID, g.AccountID, g.Permission, g.ScopeType, g.ScopeID, g.GrantedBy, g.Rationale, g.GrantedAt)
	if unique(err) {
		return application.ErrConflict
	}
	if err != nil {
		return err
	}
	if err = audit(ctx, tx, g.AccountID, g.GrantedBy, "PERMISSION_GRANTED", "permission_assignment", g.ID, g.Rationale, g.GrantedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) RevokePermission(ctx context.Context, q application.PermissionQuery, actor, rationale string, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var assignmentID string
	err = tx.QueryRow(ctx, `UPDATE identity.permission_assignments SET revoked_by=$1,revoked_at=$2,revoke_rationale=$3 WHERE account_id=$4 AND permission=$5 AND scope_type=$6 AND (scope_id=$7 OR scope_id='') AND revoked_at IS NULL RETURNING id`, actor, now, rationale, q.AccountID, q.Permission, q.ScopeType, q.ScopeID).Scan(&assignmentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrConflict
	}
	if err != nil {
		return err
	}
	if q.Permission == application.PermissionOrganizerPublish {
		_, err = tx.Exec(ctx, `UPDATE identity.organizer_applications SET status='REVOKED',reviewed_by=$1,reviewed_at=$2,review_rationale=$3 WHERE id=(SELECT id FROM identity.organizer_applications WHERE account_id=$4 AND status='APPROVED' ORDER BY reviewed_at DESC LIMIT 1)`, actor, now, rationale, q.AccountID)
		if err != nil {
			return err
		}
	}
	if err = audit(ctx, tx, q.AccountID, actor, "PERMISSION_REVOKED", "permission_assignment", assignmentID, rationale, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) CreateOrganizerApplication(ctx context.Context, a application.OrganizerApplication) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO identity.organizer_applications(id,account_id,reason,contact,status,submitted_at) VALUES($1,$2,$3,$4,$5,$6)`, a.ID, a.AccountID, a.Reason, a.Contact, a.Status, a.SubmittedAt)
	if unique(err) {
		return application.ErrConflict
	}
	return err
}

func (s *Store) OrganizerApplication(ctx context.Context, applicationID string) (application.OrganizerApplication, error) {
	var a application.OrganizerApplication
	err := s.pool.QueryRow(ctx, `SELECT id,account_id,reason,contact,status,submitted_at,COALESCE(reviewed_by,''),reviewed_at,COALESCE(review_rationale,'') FROM identity.organizer_applications WHERE id=$1`, applicationID).Scan(&a.ID, &a.AccountID, &a.Reason, &a.Contact, &a.Status, &a.SubmittedAt, &a.ReviewedBy, &a.ReviewedAt, &a.ReviewRationale)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, application.ErrInvalidInput
	}
	return a, err
}

func (s *Store) ReviewOrganizerApplication(ctx context.Context, a application.OrganizerApplication, g *application.PermissionGrant) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	tag, err := tx.Exec(ctx, `UPDATE identity.organizer_applications SET status=$1,reviewed_by=$2,reviewed_at=$3,review_rationale=$4 WHERE id=$5 AND status='PENDING'`, a.Status, a.ReviewedBy, a.ReviewedAt, a.ReviewRationale, a.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return application.ErrConflict
	}
	if g != nil {
		_, err = tx.Exec(ctx, `INSERT INTO identity.permission_assignments(id,account_id,permission,scope_type,scope_id,granted_by,rationale,granted_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, g.ID, g.AccountID, g.Permission, g.ScopeType, g.ScopeID, g.GrantedBy, g.Rationale, g.GrantedAt)
		if unique(err) {
			err = nil
		}
		if err != nil {
			return err
		}
	}
	action := "ORGANIZER_" + a.Status
	if err = audit(ctx, tx, a.AccountID, a.ReviewedBy, action, "organizer_application", a.ID, a.ReviewRationale, *a.ReviewedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) SetAccountState(ctx context.Context, accountID string, state application.AccountState, actor, rationale string, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	tag, err := tx.Exec(ctx, `UPDATE identity.accounts SET state=$1,updated_at=$2 WHERE id=$3`, state, now, accountID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return application.ErrInvalidInput
	}
	if state == application.AccountSuspended {
		if _, err = tx.Exec(ctx, `UPDATE identity.session_families SET revoked_at=$1,revoke_reason='ACCOUNT_SUSPENDED' WHERE account_id=$2 AND revoked_at IS NULL`, now, accountID); err != nil {
			return err
		}
	}
	if err = audit(ctx, tx, accountID, actor, "ACCOUNT_STATE_CHANGED", "account", accountID, rationale, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) BootstrapAdmin(ctx context.Context, accountID, rationale string, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(73194512002)`); err != nil {
		return err
	}
	var state application.AccountState
	if err = tx.QueryRow(ctx, `SELECT state FROM identity.accounts WHERE id=$1 FOR UPDATE`, accountID).Scan(&state); errors.Is(err, pgx.ErrNoRows) {
		return application.ErrInvalidInput
	} else if err != nil {
		return err
	}
	if state != application.AccountActive {
		return application.ErrForbidden
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM identity.permission_assignments`).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return application.ErrConflict
	}
	for _, permission := range []application.Permission{application.PermissionAssignmentsGrant, application.PermissionOrganizerReview, application.PermissionAccountManage, application.PermissionVenueManage} {
		assignmentID, idErr := id.New()
		if idErr != nil {
			return idErr
		}
		if _, err = tx.Exec(ctx, `INSERT INTO identity.permission_assignments(id,account_id,permission,scope_type,scope_id,granted_by,rationale,granted_at) VALUES($1,$2,$3,'GLOBAL','',NULL,$4,$5)`, assignmentID, accountID, permission, rationale, now); err != nil {
			return err
		}
	}
	auditID, idErr := id.New()
	if idErr != nil {
		return idErr
	}
	if _, err = tx.Exec(ctx, `INSERT INTO identity.security_audit(id,account_id,actor_id,action,object_type,object_id,rationale,occurred_at) VALUES($1,$2,NULL,'ADMIN_BOOTSTRAPPED','account',$2,$3,$4)`, auditID, accountID, rationale, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func audit(ctx context.Context, tx pgx.Tx, accountID, actor, action, objectType, objectID, rationale string, now time.Time) error {
	auditID, err := id.New()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO identity.security_audit(id,account_id,actor_id,action,object_type,object_id,rationale,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, auditID, accountID, actor, action, objectType, objectID, rationale, now)
	return err
}
