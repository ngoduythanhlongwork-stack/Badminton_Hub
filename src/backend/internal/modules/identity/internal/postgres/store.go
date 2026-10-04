package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"badmintonhub/internal/modules/identity/internal/application"
	"badmintonhub/internal/platform/outbox"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) (*Store, error) {
	if pool == nil {
		return nil, errors.New("identity postgres pool is required")
	}
	return &Store{pool}, nil
}

func (s *Store) CreateAccount(ctx context.Context, account application.Account, passwordHash string, token application.TokenRecord, message application.EmailObligation) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	_, err = tx.Exec(ctx, `INSERT INTO identity.accounts(id,email,canonical_email,state,created_at,updated_at) VALUES($1,$2,$2,$3,$4,$4)`, account.ID, account.Email, account.State, token.CreatedAt)
	if err != nil {
		if unique(err) {
			return application.ErrEmailExists
		}
		return fmt.Errorf("insert identity account: %w", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO identity.credentials(account_id,password_hash,changed_at) VALUES($1,$2,$3)`, account.ID, passwordHash, token.CreatedAt); err != nil {
		return err
	}
	if err = insertToken(ctx, tx, token); err != nil {
		return err
	}
	if _, err = outbox.Enqueue(ctx, tx, "identity.email", "identity-email:"+token.ID, message, token.CreatedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) AccountByCanonicalEmail(ctx context.Context, email string) (application.Account, string, error) {
	var a application.Account
	var hash string
	err := s.pool.QueryRow(ctx, `SELECT a.id,a.email,a.state,a.email_verified_at,c.password_hash FROM identity.accounts a JOIN identity.credentials c ON c.account_id=a.id WHERE a.canonical_email=$1`, email).Scan(&a.ID, &a.Email, &a.State, &a.EmailVerifiedAt, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, "", application.ErrInvalidCredentials
	}
	if err != nil {
		return a, "", err
	}
	return a, hash, nil
}

func (s *Store) AccountByID(ctx context.Context, accountID string) (application.Account, error) {
	return accountByID(ctx, s.pool, accountID)
}

type querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func accountByID(ctx context.Context, q querier, accountID string) (application.Account, error) {
	var a application.Account
	err := q.QueryRow(ctx, `SELECT id,email,state,email_verified_at FROM identity.accounts WHERE id=$1`, accountID).Scan(&a.ID, &a.Email, &a.State, &a.EmailVerifiedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, application.ErrInvalidCredentials
	}
	return a, err
}

func insertToken(ctx context.Context, tx pgx.Tx, token application.TokenRecord) error {
	_, err := tx.Exec(ctx, `INSERT INTO identity.action_tokens(id,account_id,purpose,token_hash,expires_at,created_at) VALUES($1,$2,$3,$4,$5,$6)`, token.ID, token.AccountID, token.Purpose, token.Hash, token.ExpiresAt, token.CreatedAt)
	return err
}

func (s *Store) ReplaceActionToken(ctx context.Context, token application.TokenRecord, message application.EmailObligation) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `UPDATE identity.action_tokens SET used_at=$1 WHERE account_id=$2 AND purpose=$3 AND used_at IS NULL`, token.CreatedAt, token.AccountID, token.Purpose); err != nil {
		return err
	}
	if err = insertToken(ctx, tx, token); err != nil {
		return err
	}
	if _, err = outbox.Enqueue(ctx, tx, "identity.email", "identity-email:"+token.ID, message, token.CreatedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) ConsumeVerification(ctx context.Context, hash []byte, now time.Time) (application.Account, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return application.Account{}, false, err
	}
	defer tx.Rollback(context.Background())
	var accountID string
	var expires time.Time
	var used *time.Time
	err = tx.QueryRow(ctx, `SELECT account_id,expires_at,used_at FROM identity.action_tokens WHERE token_hash=$1 AND purpose='VERIFY_EMAIL' FOR UPDATE`, hash).Scan(&accountID, &expires, &used)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !expires.After(now)) {
		return application.Account{}, false, application.ErrInvalidToken
	}
	if err != nil {
		return application.Account{}, false, err
	}
	if used == nil {
		if _, err = tx.Exec(ctx, `UPDATE identity.action_tokens SET used_at=$1 WHERE token_hash=$2`, now, hash); err != nil {
			return application.Account{}, false, err
		}
		if _, err = tx.Exec(ctx, `UPDATE identity.accounts SET state='ACTIVE',email_verified_at=$1,updated_at=$1 WHERE id=$2 AND state='UNVERIFIED'`, now, accountID); err != nil {
			return application.Account{}, false, err
		}
	}
	a, err := accountByID(ctx, tx, accountID)
	if err != nil {
		return a, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return a, false, err
	}
	return a, used == nil, nil
}

func (s *Store) ConsumeReset(ctx context.Context, hash []byte, passwordHash string, now time.Time) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(context.Background())
	var accountID string
	var expires time.Time
	var used *time.Time
	err = tx.QueryRow(ctx, `SELECT account_id,expires_at,used_at FROM identity.action_tokens WHERE token_hash=$1 AND purpose='RESET_PASSWORD' FOR UPDATE`, hash).Scan(&accountID, &expires, &used)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (used != nil || !expires.After(now))) {
		return "", application.ErrInvalidToken
	}
	if err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `UPDATE identity.action_tokens SET used_at=$1 WHERE token_hash=$2`, now, hash); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `UPDATE identity.credentials SET password_hash=$1,changed_at=$2 WHERE account_id=$3`, passwordHash, now, accountID); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `UPDATE identity.session_families SET revoked_at=$1,revoke_reason='PASSWORD_RESET' WHERE account_id=$2 AND revoked_at IS NULL`, now, accountID); err != nil {
		return "", err
	}
	return accountID, tx.Commit(ctx)
}

func (s *Store) CheckAndRecordResend(ctx context.Context, accountID, source string, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `SELECT id FROM identity.accounts WHERE id=$1 FOR UPDATE`, accountID); err != nil {
		return err
	}
	var count int
	var latest *time.Time
	if err = tx.QueryRow(ctx, `SELECT count(*),max(created_at) FROM identity.resend_attempts WHERE account_id=$1 AND source_key=$2 AND created_at>$3`, accountID, source, now.Add(-24*time.Hour)).Scan(&count, &latest); err != nil {
		return err
	}
	if count >= 5 || (latest != nil && latest.After(now.Add(-time.Minute))) {
		return application.ErrRateLimited
	}
	if _, err = tx.Exec(ctx, `INSERT INTO identity.resend_attempts(account_id,source_key,created_at) VALUES($1,$2,$3)`, accountID, source, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) LoginFailureCount(ctx context.Context, email, source string, since time.Time) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM identity.login_failures WHERE canonical_email=$1 AND source_key=$2 AND created_at>=$3`, email, source, since).Scan(&n)
	return n, err
}
func (s *Store) RecordLoginFailure(ctx context.Context, email, source string, now time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO identity.login_failures(canonical_email,source_key,created_at) VALUES($1,$2,$3)`, email, source, now)
	return err
}
func (s *Store) ClearLoginFailures(ctx context.Context, email, source string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM identity.login_failures WHERE canonical_email=$1 AND source_key=$2`, email, source)
	return err
}

func (s *Store) CompleteLoginAttempt(ctx context.Context, email, source string, now time.Time, succeeded bool) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,hashtextextended($2,0)))`, email, source); err != nil {
		return false, err
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM identity.login_failures WHERE canonical_email=$1 AND source_key=$2 AND created_at>=$3`, email, source, now.Add(-15*time.Minute)).Scan(&count); err != nil {
		return false, err
	}
	if count >= 10 {
		if err = tx.Commit(ctx); err != nil {
			return false, err
		}
		return false, nil
	}
	if succeeded {
		_, err = tx.Exec(ctx, `DELETE FROM identity.login_failures WHERE canonical_email=$1 AND source_key=$2`, email, source)
	} else {
		_, err = tx.Exec(ctx, `INSERT INTO identity.login_failures(canonical_email,source_key,created_at) VALUES($1,$2,$3)`, email, source, now)
	}
	if err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) CreateSession(ctx context.Context, r application.SessionRecord) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `INSERT INTO identity.session_families(id,account_id,expires_at,created_at) VALUES($1,$2,$3,$4)`, r.FamilyID, r.AccountID, r.RefreshExpiresAt, r.CreatedAt); err != nil {
		return err
	}
	if err = insertSessionTokens(ctx, tx, r); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func insertSessionTokens(ctx context.Context, tx pgx.Tx, r application.SessionRecord) error {
	if _, err := tx.Exec(ctx, `INSERT INTO identity.access_tokens(id,family_id,token_hash,expires_at,created_at) VALUES($1,$2,$3,$4,$5)`, r.AccessID, r.FamilyID, r.AccessHash, r.AccessExpiresAt, r.CreatedAt); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO identity.refresh_tokens(id,family_id,token_hash,expires_at,created_at) VALUES($1,$2,$3,$4,$5)`, r.RefreshID, r.FamilyID, r.RefreshHash, r.RefreshExpiresAt, r.CreatedAt)
	return err
}

func (s *Store) RotateRefresh(ctx context.Context, hash []byte, r application.SessionRecord, now time.Time) (application.Account, time.Time, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return application.Account{}, time.Time{}, err
	}
	defer tx.Rollback(context.Background())
	var familyID, accountID string
	var tokenExpires, familyExpires time.Time
	var consumed, revoked *time.Time
	err = tx.QueryRow(ctx, `SELECT rt.family_id,sf.account_id,rt.expires_at,sf.expires_at,rt.consumed_at,sf.revoked_at FROM identity.refresh_tokens rt JOIN identity.session_families sf ON sf.id=rt.family_id WHERE rt.token_hash=$1 FOR UPDATE OF rt,sf`, hash).Scan(&familyID, &accountID, &tokenExpires, &familyExpires, &consumed, &revoked)
	if errors.Is(err, pgx.ErrNoRows) {
		return application.Account{}, time.Time{}, application.ErrInvalidToken
	}
	if err != nil {
		return application.Account{}, time.Time{}, err
	}
	if consumed != nil {
		if revoked == nil {
			if _, err = tx.Exec(ctx, `UPDATE identity.session_families SET revoked_at=$1,revoke_reason='REFRESH_REUSE' WHERE id=$2`, now, familyID); err != nil {
				return application.Account{}, time.Time{}, err
			}
		}
		if err = tx.Commit(ctx); err != nil {
			return application.Account{}, time.Time{}, err
		}
		return application.Account{}, time.Time{}, application.ErrInvalidToken
	}
	if revoked != nil || !tokenExpires.After(now) || !familyExpires.After(now) {
		return application.Account{}, time.Time{}, application.ErrInvalidToken
	}
	if _, err = tx.Exec(ctx, `UPDATE identity.refresh_tokens SET consumed_at=$1 WHERE token_hash=$2`, now, hash); err != nil {
		return application.Account{}, time.Time{}, err
	}
	r.FamilyID = familyID
	r.AccountID = accountID
	r.RefreshExpiresAt = familyExpires
	if r.AccessExpiresAt.After(familyExpires) {
		r.AccessExpiresAt = familyExpires
	}
	if err = insertSessionTokens(ctx, tx, r); err != nil {
		return application.Account{}, time.Time{}, err
	}
	a, err := accountByID(ctx, tx, accountID)
	if err != nil {
		return a, time.Time{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return a, time.Time{}, err
	}
	return a, familyExpires, nil
}

func (s *Store) RevokeFamilyByAccess(ctx context.Context, hash []byte, now time.Time, reason string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE identity.session_families sf SET revoked_at=$1,revoke_reason=$2 FROM identity.access_tokens at WHERE at.family_id=sf.id AND at.token_hash=$3 AND sf.revoked_at IS NULL`, now, reason, hash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return application.ErrInvalidToken
	}
	return nil
}
func (s *Store) AuthenticateAccess(ctx context.Context, hash []byte, now time.Time) (application.Account, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT sf.account_id FROM identity.access_tokens at JOIN identity.session_families sf ON sf.id=at.family_id JOIN identity.accounts a ON a.id=sf.account_id WHERE at.token_hash=$1 AND at.expires_at>$2 AND at.revoked_at IS NULL AND sf.expires_at>$2 AND sf.revoked_at IS NULL AND a.state IN ('ACTIVE','RESTRICTED')`, hash, now).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return application.Account{}, application.ErrInvalidToken
	}
	if err != nil {
		return application.Account{}, err
	}
	return accountByID(ctx, s.pool, id)
}
func (s *Store) RevokeAccountSessions(ctx context.Context, accountID string, now time.Time, reason string) error {
	_, err := s.pool.Exec(ctx, `UPDATE identity.session_families SET revoked_at=$1,revoke_reason=$2 WHERE account_id=$3 AND revoked_at IS NULL`, now, reason, accountID)
	return err
}

func unique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

var _ application.Store = (*Store)(nil)
