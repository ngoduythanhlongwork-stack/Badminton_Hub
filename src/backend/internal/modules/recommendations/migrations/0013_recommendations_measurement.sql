CREATE SCHEMA IF NOT EXISTS recommendations;
CREATE TABLE recommendations.scoring_configs (
 version text PRIMARY KEY, status text NOT NULL CHECK(status IN('DRAFT','APPROVED','ACTIVE','RETIRED')),
 weights jsonb NOT NULL, approved_at timestamptz, activated_at timestamptz
);
INSERT INTO recommendations.scoring_configs(version,status,weights,approved_at,activated_at)
VALUES('D23-V1','ACTIVE','{"skill":35,"distance":25,"time":20,"preference":15,"hostReliability":5}',now(),now());
CREATE TABLE recommendations.exposures (
 id uuid PRIMARY KEY, player_id uuid NOT NULL, scoring_version text NOT NULL REFERENCES recommendations.scoring_configs(version),
 candidate_count integer NOT NULL, result_match_ids uuid[] NOT NULL, reason_codes jsonb NOT NULL, exposed_at timestamptz NOT NULL
);
CREATE INDEX recommendation_exposure_retention ON recommendations.exposures(exposed_at);
CREATE TABLE recommendations.analytics_events (
 id uuid PRIMARY KEY, event_key text NOT NULL UNIQUE, event_type text NOT NULL, source_id uuid,
 source_revision integer, actor_id uuid, match_id uuid, payload jsonb NOT NULL DEFAULT '{}',
 metric_version text NOT NULL DEFAULT 'D24-V1', occurred_at timestamptz NOT NULL, recorded_at timestamptz NOT NULL
);
CREATE TABLE recommendations.attendance_outcomes (
 participation_id uuid PRIMARY KEY, match_id uuid NOT NULL, revision integer NOT NULL,
 outcome text NOT NULL CHECK(outcome IN('PRESENT','NO_SHOW','UNKNOWN')), occurred_at timestamptz NOT NULL
);
