CREATE SCHEMA players;
CREATE TABLE players.profiles (
 account_id uuid PRIMARY KEY,
 onboarding_status text NOT NULL DEFAULT 'IN_PROGRESS' CHECK (onboarding_status IN ('IN_PROGRESS','COMPLETE')),
 display_name text CHECK (display_name IS NULL OR char_length(btrim(display_name)) BETWEEN 1 AND 100),
 avatar_url text,
 date_of_birth date,
 gender text CHECK (gender IS NULL OR gender IN ('FEMALE','MALE','NON_BINARY','UNDISCLOSED')),
 experience text CHECK (experience IS NULL OR experience IN ('UNDER_3_MONTHS','3_TO_12_MONTHS','1_TO_3_YEARS','3_PLUS_YEARS')),
 skill_level text CHECK (skill_level IS NULL OR skill_level IN ('BEGINNER','BEGINNER_PLUS','INTERMEDIATE','INTERMEDIATE_PLUS','ADVANCED','COMPETITIVE')),
 preferred_formats text[] NOT NULL DEFAULT '{}' CHECK (preferred_formats <@ ARRAY['SINGLES','DOUBLES','MIXED']::text[]),
 play_styles text[] NOT NULL DEFAULT '{}' CHECK (play_styles <@ ARRAY['CASUAL','SOCIAL','TRAINING','COMPETITIVE']::text[]),
 usual_periods text[] NOT NULL DEFAULT '{}' CHECK (usual_periods <@ ARRAY['MORNING','AFTERNOON','EVENING']::text[]),
 regular_area text CHECK (regular_area IS NULL OR char_length(btrim(regular_area)) BETWEEN 1 AND 120),
 skill_confidence text NOT NULL DEFAULT 'NEW' CHECK (skill_confidence IN ('NEW','DEVELOPING','ESTABLISHED')),
 reliability_label text NOT NULL DEFAULT 'NEW' CHECK (reliability_label IN ('NEW','RELIABLE','GOOD','NEEDS_IMPROVEMENT')),
 match_count integer NOT NULL DEFAULT 0 CHECK (match_count >= 0),
 completed_at timestamptz,
 updated_at timestamptz NOT NULL,
 version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
 CHECK (onboarding_status <> 'COMPLETE' OR (display_name IS NOT NULL AND date_of_birth IS NOT NULL AND
  experience IS NOT NULL AND skill_level IS NOT NULL AND cardinality(preferred_formats) > 0 AND
  cardinality(play_styles) > 0 AND cardinality(usual_periods) > 0 AND regular_area IS NOT NULL AND completed_at IS NOT NULL))
);
CREATE FUNCTION players.protect_completed_profile() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF OLD.onboarding_status = 'COMPLETE' AND NEW.onboarding_status <> 'COMPLETE' THEN RAISE EXCEPTION 'completed onboarding cannot be reopened'; END IF;
 IF OLD.onboarding_status = 'COMPLETE' AND NEW.date_of_birth IS DISTINCT FROM OLD.date_of_birth THEN RAISE EXCEPTION 'date of birth cannot be changed after onboarding'; END IF;
 RETURN NEW;
END;
$$;
CREATE TRIGGER protect_completed_profile BEFORE UPDATE ON players.profiles
FOR EACH ROW EXECUTE FUNCTION players.protect_completed_profile();
