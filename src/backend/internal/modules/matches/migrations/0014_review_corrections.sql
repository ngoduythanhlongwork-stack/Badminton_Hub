ALTER TABLE matches.reviews
    ADD COLUMN corrected_at timestamptz,
    ADD COLUMN corrected_by uuid;

CREATE TABLE matches.review_revisions (
    id uuid PRIMARY KEY,
    review_id uuid NOT NULL REFERENCES matches.reviews(id),
    match_id uuid NOT NULL,
    revision integer NOT NULL CHECK(revision > 1),
    previous_match_quality integer NOT NULL,
    previous_host_rating integer NOT NULL,
    previous_tags text[] NOT NULL,
    previous_skill_feedback text NOT NULL,
    new_match_quality integer NOT NULL CHECK(new_match_quality BETWEEN 1 AND 5),
    new_host_rating integer NOT NULL CHECK(new_host_rating BETWEEN 1 AND 5),
    new_tags text[] NOT NULL,
    new_skill_feedback text NOT NULL CHECK(new_skill_feedback IN('AS_EXPECTED','STRONGER_THAN_PROFILE','LOWER_THAN_PROFILE')),
    actor_id uuid NOT NULL,
    case_id uuid NOT NULL,
    reason text NOT NULL,
    occurred_at timestamptz NOT NULL,
    UNIQUE(review_id,revision)
);
