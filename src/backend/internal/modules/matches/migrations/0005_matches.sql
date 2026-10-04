CREATE SCHEMA IF NOT EXISTS matches;
CREATE TABLE matches.matches (
 id uuid PRIMARY KEY, host_id uuid NOT NULL, title text NOT NULL, description text NOT NULL,
 venue_id uuid NOT NULL, venue_name text NOT NULL, venue_address text NOT NULL, venue_area text NOT NULL, venue_time_zone text NOT NULL, court text NOT NULL DEFAULT '',
 start_at timestamptz NOT NULL, end_at timestamptz NOT NULL, format text NOT NULL, style text NOT NULL, rules text NOT NULL,
 min_level integer NOT NULL, max_level integer NOT NULL, capacity integer NOT NULL CHECK(capacity>0), fee_minor bigint NOT NULL CHECK(fee_minor=0), currency char(3) NOT NULL,
 join_mode text NOT NULL CHECK(join_mode IN('INSTANT','APPROVAL_REQUIRED')), host_plays boolean NOT NULL,
 court_attested boolean NOT NULL, court_attested_at timestamptz NOT NULL,
 status text NOT NULL CHECK(status IN('DRAFT','OPEN','FULL')), created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 CHECK(end_at>start_at), CHECK(min_level<=max_level)
);
CREATE TABLE matches.participations (
 id uuid PRIMARY KEY, match_id uuid NOT NULL REFERENCES matches.matches(id), player_id uuid NOT NULL,
 status text NOT NULL CHECK(status IN('REQUESTED','JOINED','REJECTED','REMOVED')),
 decision_reason text NOT NULL DEFAULT '', start_at timestamptz NOT NULL, end_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX one_active_participation_per_match ON matches.participations(match_id,player_id) WHERE status IN('REQUESTED','JOINED');
CREATE INDEX active_player_schedule ON matches.participations(player_id,start_at,end_at) WHERE status='JOINED';
CREATE TABLE matches.idempotency (
 actor_id uuid NOT NULL, action text NOT NULL, idempotency_key text NOT NULL, fingerprint text NOT NULL, participation_id uuid NOT NULL REFERENCES matches.participations(id), created_at timestamptz NOT NULL,
 PRIMARY KEY(actor_id,action,idempotency_key)
);
CREATE INDEX matches_public_search ON matches.matches(status,start_at,id);
