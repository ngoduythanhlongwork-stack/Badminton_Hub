CREATE SCHEMA IF NOT EXISTS communication;
CREATE TABLE communication.messages (
 id uuid PRIMARY KEY, match_id uuid NOT NULL, sender_id uuid NOT NULL,
 body text NOT NULL CHECK(char_length(body) BETWEEN 1 AND 2000),
 moderation_state text NOT NULL DEFAULT 'VISIBLE' CHECK(moderation_state IN('VISIBLE','HIDDEN')),
 created_at timestamptz NOT NULL,
 UNIQUE(sender_id,match_id,id)
);
CREATE INDEX room_messages ON communication.messages(match_id,created_at,id);
CREATE TABLE communication.idempotency (
 actor_id uuid NOT NULL, match_id uuid NOT NULL, idempotency_key text NOT NULL,
 fingerprint text NOT NULL, message_id uuid NOT NULL REFERENCES communication.messages(id), created_at timestamptz NOT NULL,
 PRIMARY KEY(actor_id,match_id,idempotency_key)
);
