CREATE SCHEMA IF NOT EXISTS venues;
CREATE TABLE venues.venues (
 id uuid PRIMARY KEY, name text NOT NULL, address text NOT NULL DEFAULT '', area text NOT NULL DEFAULT '', latitude double precision, longitude double precision, time_zone text NOT NULL DEFAULT '',
 opening_hours text[] NOT NULL DEFAULT '{}', amenities text[] NOT NULL DEFAULT '{}', photos text[] NOT NULL DEFAULT '{}', courts text[] NOT NULL DEFAULT '{}',
 price_amount_minor bigint, price_currency char(3), price_unit text, price_source_kind text, price_source_reference text, price_updated_at timestamptz,
 status text NOT NULL CHECK(status IN('DRAFT','PUBLISHED','HIDDEN')), created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 CHECK ((price_amount_minor IS NULL AND price_currency IS NULL AND price_unit IS NULL AND price_source_kind IS NULL AND price_updated_at IS NULL) OR (price_amount_minor >= 0 AND price_currency IS NOT NULL AND price_unit IS NOT NULL AND price_source_kind IS NOT NULL AND price_updated_at IS NOT NULL)),
 CHECK ((latitude IS NULL AND longitude IS NULL) OR (latitude BETWEEN -90 AND 90 AND longitude BETWEEN -180 AND 180))
);
CREATE TABLE venues.manager_assignments (venue_id uuid NOT NULL REFERENCES venues.venues(id), manager_id uuid NOT NULL, assigned_by uuid NOT NULL, assigned_at timestamptz NOT NULL, revoked_at timestamptz, PRIMARY KEY(venue_id,manager_id));
CREATE INDEX venues_public_area_idx ON venues.venues(area,id) WHERE status='PUBLISHED';
