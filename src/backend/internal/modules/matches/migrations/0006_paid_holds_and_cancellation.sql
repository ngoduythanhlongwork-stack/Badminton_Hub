ALTER TABLE matches.matches DROP CONSTRAINT matches_fee_minor_check;
ALTER TABLE matches.matches
    ADD CONSTRAINT matches_fee_minor_check CHECK (fee_minor >= 0),
    ADD COLUMN deposit_minor bigint NOT NULL DEFAULT 0,
    ADD COLUMN payment_recipient text NOT NULL DEFAULT '',
    ADD COLUMN payment_instructions text NOT NULL DEFAULT '',
    ADD COLUMN payment_instruction_version integer NOT NULL DEFAULT 1,
    ADD COLUMN cancellation_policy_version text NOT NULL DEFAULT 'MVP-2026-10-D15',
    ADD COLUMN cancelled_at timestamptz,
    ADD COLUMN cancelled_by uuid,
    ADD COLUMN cancellation_reason text NOT NULL DEFAULT '',
    ADD CONSTRAINT matches_deposit_check CHECK (
        deposit_minor >= 0 AND deposit_minor <= fee_minor
        AND (deposit_minor = 0 OR (currency = 'VND' AND payment_recipient <> '' AND payment_instructions <> ''))
    );

ALTER TABLE matches.matches DROP CONSTRAINT matches_status_check;
ALTER TABLE matches.matches ADD CONSTRAINT matches_status_check
    CHECK (status IN ('DRAFT','OPEN','FULL','CANCELLED'));

ALTER TABLE matches.participations DROP CONSTRAINT participations_status_check;
ALTER TABLE matches.participations ADD CONSTRAINT participations_status_check
    CHECK (status IN ('REQUESTED','AWAITING_PAYMENT','JOINED','REJECTED','REMOVED','CANCELLED','EXPIRED'));

DROP INDEX matches.one_active_participation_per_match;
CREATE UNIQUE INDEX one_active_participation_per_match
    ON matches.participations(match_id,player_id)
    WHERE status IN ('REQUESTED','AWAITING_PAYMENT','JOINED');
DROP INDEX matches.active_player_schedule;
CREATE INDEX active_player_schedule
    ON matches.participations(player_id,start_at,end_at)
    WHERE status IN ('AWAITING_PAYMENT','JOINED');

CREATE TABLE matches.holds (
    id uuid PRIMARY KEY,
    participation_id uuid NOT NULL UNIQUE REFERENCES matches.participations(id),
    match_id uuid NOT NULL REFERENCES matches.matches(id),
    player_id uuid NOT NULL,
    status text NOT NULL CHECK (status IN ('HELD','CONSUMED','EXPIRED','RELEASED')),
    expires_at timestamptz NOT NULL,
    transfer_reported_at timestamptz,
    consumed_at timestamptz,
    released_at timestamptz,
    release_reason text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE INDEX matches_live_holds_by_match ON matches.holds(match_id,expires_at) WHERE status='HELD';
CREATE INDEX matches_live_holds_by_player ON matches.holds(player_id,expires_at) WHERE status='HELD';

CREATE TABLE matches.command_results (
    actor_id uuid NOT NULL,
    action text NOT NULL,
    idempotency_key text NOT NULL,
    fingerprint text NOT NULL,
    resource_type text NOT NULL,
    resource_id uuid NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY(actor_id,action,idempotency_key)
);

CREATE TABLE matches.cancellation_audit (
    id uuid PRIMARY KEY,
    match_id uuid NOT NULL REFERENCES matches.matches(id),
    participation_id uuid REFERENCES matches.participations(id),
    actor_id uuid NOT NULL,
    cause text NOT NULL CHECK(cause IN ('PLAYER_WITHDRAWAL','HOST_REMOVAL','HOST_MATCH_CANCELLATION')),
    reason text NOT NULL,
    refund_outcome text NOT NULL CHECK(refund_outcome IN ('FULL','NONE')),
    policy_version text NOT NULL,
    occurred_at timestamptz NOT NULL
);
