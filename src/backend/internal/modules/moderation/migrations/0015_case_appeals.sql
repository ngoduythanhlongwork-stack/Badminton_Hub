ALTER TABLE moderation.cases DROP CONSTRAINT cases_subject_type_check;
ALTER TABLE moderation.cases ADD CONSTRAINT cases_subject_type_check
    CHECK(subject_type IN('USER','MATCH','MESSAGE','PAYMENT','ATTENDANCE','REVIEW'));

CREATE TABLE moderation.case_appeals (
    id uuid PRIMARY KEY,
    case_id uuid NOT NULL UNIQUE REFERENCES moderation.cases(id),
    appellant_id uuid NOT NULL,
    reason text NOT NULL,
    created_at timestamptz NOT NULL
);
