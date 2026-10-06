ALTER TABLE matches.matches DROP CONSTRAINT matches_status_check;
ALTER TABLE matches.matches ADD CONSTRAINT matches_status_check
    CHECK (status IN ('DRAFT','OPEN','FULL','IN_PROGRESS','COMPLETED','CANCELLED'));
ALTER TABLE matches.matches ADD COLUMN completed_at timestamptz;

ALTER TABLE matches.participations DROP CONSTRAINT participations_status_check;
ALTER TABLE matches.participations ADD CONSTRAINT participations_status_check
    CHECK (status IN ('REQUESTED','AWAITING_PAYMENT','JOINED','CHECKED_IN','COMPLETED','NO_SHOW','UNKNOWN','REJECTED','REMOVED','CANCELLED','EXPIRED'));
ALTER TABLE matches.participations
    ADD COLUMN attendance_revision integer NOT NULL DEFAULT 0 CHECK (attendance_revision >= 0),
    ADD COLUMN attendance_recorded_at timestamptz,
    ADD COLUMN attendance_recorded_by uuid;

DROP INDEX matches.one_active_participation_per_match;
CREATE UNIQUE INDEX one_active_participation_per_match
    ON matches.participations(match_id,player_id)
    WHERE status IN ('REQUESTED','AWAITING_PAYMENT','JOINED','CHECKED_IN','COMPLETED','NO_SHOW','UNKNOWN');
DROP INDEX matches.active_player_schedule;
CREATE INDEX active_player_schedule
    ON matches.participations(player_id,start_at,end_at)
    WHERE status IN ('AWAITING_PAYMENT','JOINED','CHECKED_IN');

CREATE TABLE matches.attendance_revisions (
    id uuid PRIMARY KEY,
    participation_id uuid NOT NULL REFERENCES matches.participations(id),
    match_id uuid NOT NULL REFERENCES matches.matches(id),
    player_id uuid NOT NULL,
    revision integer NOT NULL CHECK (revision > 0),
    previous_status text NOT NULL,
    new_status text NOT NULL CHECK (new_status IN ('CHECKED_IN','NO_SHOW','UNKNOWN')),
    actor_id uuid NOT NULL,
    source text NOT NULL CHECK (source IN ('HOST','SYSTEM','ADMIN_CASE')),
    case_id uuid,
    reason text NOT NULL DEFAULT '',
    occurred_at timestamptz NOT NULL,
    UNIQUE(participation_id,revision),
    CHECK ((source='ADMIN_CASE') = (case_id IS NOT NULL))
);

CREATE TABLE matches.reviews (
    id uuid PRIMARY KEY,
    match_id uuid NOT NULL REFERENCES matches.matches(id),
    reviewer_id uuid NOT NULL,
    target_player_id uuid NOT NULL,
    match_quality integer NOT NULL CHECK (match_quality BETWEEN 1 AND 5),
    host_rating integer NOT NULL CHECK (host_rating BETWEEN 1 AND 5),
    tags text[] NOT NULL DEFAULT '{}',
    skill_feedback text NOT NULL CHECK (skill_feedback IN ('AS_EXPECTED','STRONGER_THAN_PROFILE','LOWER_THAN_PROFILE')),
    revision integer NOT NULL DEFAULT 1 CHECK (revision > 0),
    submitted_at timestamptz NOT NULL,
    UNIQUE(match_id,reviewer_id,target_player_id),
    CHECK (reviewer_id <> target_player_id),
    CHECK (tags <@ ARRAY['FRIENDLY','FAIR','COMPETITIVE','ON_TIME','GOOD_SKILL_MATCH']::text[])
);
CREATE INDEX reviews_target_recent ON matches.reviews(target_player_id,submitted_at DESC);
