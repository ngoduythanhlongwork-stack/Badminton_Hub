ALTER TABLE players.profiles
    ADD COLUMN reliability_score numeric(5,2),
    ADD COLUMN reliability_sample_size integer NOT NULL DEFAULT 0 CHECK (reliability_sample_size >= 0),
    ADD COLUMN skill_feedback_count integer NOT NULL DEFAULT 0 CHECK (skill_feedback_count >= 0),
    ADD COLUMN level_review_suggested boolean NOT NULL DEFAULT false;

CREATE TABLE players.trust_signals (
    id uuid PRIMARY KEY,
    account_id uuid NOT NULL,
    source_type text NOT NULL CHECK (source_type IN ('ATTENDANCE','LATE_CANCEL','SKILL_FEEDBACK')),
    source_id uuid NOT NULL,
    source_revision integer NOT NULL CHECK (source_revision > 0),
    match_id uuid NOT NULL,
    reliability_value numeric(3,2),
    skill_direction text CHECK (skill_direction IN ('AS_EXPECTED','STRONGER_THAN_PROFILE','LOWER_THAN_PROFILE')),
    effective boolean NOT NULL DEFAULT true,
    occurred_at timestamptz NOT NULL,
    recorded_at timestamptz NOT NULL,
    UNIQUE(source_type,source_id,source_revision)
);
CREATE UNIQUE INDEX one_effective_trust_signal
    ON players.trust_signals(source_type,source_id) WHERE effective;
CREATE INDEX trust_signals_account_recent
    ON players.trust_signals(account_id,occurred_at DESC) WHERE effective;
