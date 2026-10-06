CREATE SCHEMA IF NOT EXISTS moderation;
CREATE TABLE moderation.blocks (
 blocker_id uuid NOT NULL, blocked_id uuid NOT NULL, active boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 PRIMARY KEY(blocker_id,blocked_id), CHECK(blocker_id<>blocked_id)
);
CREATE TABLE moderation.cases (
 id uuid PRIMARY KEY, subject_type text NOT NULL CHECK(subject_type IN('USER','MATCH','MESSAGE','PAYMENT','ATTENDANCE')),
 subject_id uuid NOT NULL, reporter_id uuid NOT NULL, target_actor_id uuid,
 reason text NOT NULL CHECK(reason IN('SPAM','FRAUD','HARASSMENT','FAKE_SKILL','NO_SHOW','OTHER')),
 description text NOT NULL, evidence_ref text, status text NOT NULL DEFAULT 'SUBMITTED'
 CHECK(status IN('SUBMITTED','TRIAGED','IN_REVIEW','WAITING_INFORMATION','DECIDED','RESOLVED','REOPENED')),
 linked_case_id uuid REFERENCES moderation.cases(id), assignee_id uuid, decision text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL, resolved_at timestamptz
);
CREATE INDEX moderation_case_queue ON moderation.cases(status,created_at);
CREATE TABLE moderation.case_audit (
 id uuid PRIMARY KEY, case_id uuid NOT NULL REFERENCES moderation.cases(id), actor_id uuid NOT NULL,
 action text NOT NULL, before_state text NOT NULL, after_state text NOT NULL, reason text NOT NULL, created_at timestamptz NOT NULL
);
