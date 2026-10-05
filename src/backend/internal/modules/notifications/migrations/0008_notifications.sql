CREATE SCHEMA IF NOT EXISTS notifications;

CREATE TABLE notifications.notifications (
    id uuid PRIMARY KEY,
    source_event_id text NOT NULL,
    source_version integer NOT NULL CHECK(source_version > 0),
    recipient_id uuid NOT NULL,
    channel text NOT NULL CHECK(channel IN ('IN_APP','EMAIL')),
    purpose text NOT NULL,
    template_version integer NOT NULL CHECK(template_version > 0),
    title text NOT NULL,
    body text NOT NULL,
    action_path text NOT NULL DEFAULT '',
    scheduled_at timestamptz NOT NULL,
    valid_until timestamptz,
    delivery_status text NOT NULL CHECK(delivery_status IN ('PENDING','SENT','FAILED','RETRY_PENDING','ABANDONED','CANCELLED')),
    attempt_count integer NOT NULL DEFAULT 0 CHECK(attempt_count >= 0),
    last_error text NOT NULL DEFAULT '',
    sent_at timestamptz,
    read_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE(source_event_id,source_version,recipient_id,channel,purpose)
);
CREATE INDEX notifications_recipient_idx ON notifications.notifications(recipient_id,created_at DESC);
CREATE INDEX notifications_due_idx ON notifications.notifications(scheduled_at) WHERE delivery_status IN ('PENDING','RETRY_PENDING');

CREATE TABLE notifications.preferences (
    account_id uuid PRIMARY KEY,
    email_reminders boolean NOT NULL DEFAULT true,
    updated_at timestamptz NOT NULL
);
