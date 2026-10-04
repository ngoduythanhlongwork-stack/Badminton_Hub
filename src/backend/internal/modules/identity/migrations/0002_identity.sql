CREATE SCHEMA IF NOT EXISTS identity;

CREATE TABLE identity.accounts (
    id text PRIMARY KEY,
    email text NOT NULL,
    canonical_email text NOT NULL UNIQUE,
    state text NOT NULL CHECK (state IN ('UNVERIFIED','ACTIVE','RESTRICTED','SUSPENDED')),
    email_verified_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK ((state = 'UNVERIFIED') = (email_verified_at IS NULL))
);

CREATE TABLE identity.credentials (
    account_id text PRIMARY KEY REFERENCES identity.accounts(id),
    password_hash text NOT NULL,
    changed_at timestamptz NOT NULL
);

CREATE TABLE identity.action_tokens (
    id text PRIMARY KEY,
    account_id text NOT NULL REFERENCES identity.accounts(id),
    purpose text NOT NULL CHECK (purpose IN ('VERIFY_EMAIL','RESET_PASSWORD')),
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_at timestamptz NOT NULL
);
CREATE INDEX identity_action_tokens_account_purpose_idx
    ON identity.action_tokens(account_id, purpose, created_at DESC);

CREATE TABLE identity.resend_attempts (
    account_id text NOT NULL REFERENCES identity.accounts(id),
    source_key text NOT NULL,
    created_at timestamptz NOT NULL
);
CREATE INDEX identity_resend_attempts_window_idx
    ON identity.resend_attempts(account_id, source_key, created_at DESC);

CREATE TABLE identity.login_failures (
    canonical_email text NOT NULL,
    source_key text NOT NULL,
    created_at timestamptz NOT NULL
);
CREATE INDEX identity_login_failures_window_idx
    ON identity.login_failures(canonical_email, source_key, created_at DESC);

CREATE TABLE identity.session_families (
    id text PRIMARY KEY,
    account_id text NOT NULL REFERENCES identity.accounts(id),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    revoke_reason text,
    created_at timestamptz NOT NULL
);
CREATE INDEX identity_session_families_account_idx ON identity.session_families(account_id);

CREATE TABLE identity.refresh_tokens (
    id text PRIMARY KEY,
    family_id text NOT NULL REFERENCES identity.session_families(id),
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL
);

CREATE TABLE identity.access_tokens (
    id text PRIMARY KEY,
    family_id text NOT NULL REFERENCES identity.session_families(id),
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL
);

CREATE TABLE identity.permission_assignments (
    id text PRIMARY KEY,
    account_id text NOT NULL REFERENCES identity.accounts(id),
    permission text NOT NULL,
    scope_type text NOT NULL,
    scope_id text NOT NULL DEFAULT '',
    granted_by text REFERENCES identity.accounts(id),
    rationale text NOT NULL,
    granted_at timestamptz NOT NULL,
    revoked_by text REFERENCES identity.accounts(id),
    revoked_at timestamptz,
    revoke_rationale text
);
CREATE UNIQUE INDEX identity_permission_active_unique
    ON identity.permission_assignments(account_id, permission, scope_type, scope_id)
    WHERE revoked_at IS NULL;

CREATE TABLE identity.organizer_applications (
    id text PRIMARY KEY,
    account_id text NOT NULL REFERENCES identity.accounts(id),
    reason text NOT NULL,
    contact text NOT NULL,
    status text NOT NULL CHECK (status IN ('PENDING','APPROVED','REJECTED','REVOKED')),
    submitted_at timestamptz NOT NULL,
    reviewed_by text REFERENCES identity.accounts(id),
    reviewed_at timestamptz,
    review_rationale text
);
CREATE UNIQUE INDEX identity_organizer_one_pending
    ON identity.organizer_applications(account_id) WHERE status='PENDING';

CREATE TABLE identity.security_audit (
    id text PRIMARY KEY,
    account_id text REFERENCES identity.accounts(id),
    actor_id text REFERENCES identity.accounts(id),
    action text NOT NULL,
    object_type text NOT NULL,
    object_id text NOT NULL,
    rationale text,
    occurred_at timestamptz NOT NULL
);
