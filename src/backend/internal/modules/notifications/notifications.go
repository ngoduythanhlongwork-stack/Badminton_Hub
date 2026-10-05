package notifications

import (
	"context"
	"errors"
	"sync"
	"time"

	"badmintonhub/internal/platform/clock"
	"badmintonhub/internal/platform/id"
	"badmintonhub/internal/platform/outbox"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalid   = errors.New("notifications: invalid input")
	ErrForbidden = errors.New("notifications: forbidden")
	ErrNotFound  = errors.New("notifications: not found")
)

type Notification struct {
	ID, SourceEventID, RecipientID, Channel, Purpose   string
	SourceVersion, TemplateVersion, AttemptCount       int
	Title, Body, ActionPath, DeliveryStatus, LastError string
	ScheduledAt                                        time.Time
	ValidUntil, SentAt, ReadAt                         *time.Time
	CreatedAt, UpdatedAt                               time.Time
}

type AccountDirectory interface {
	EmailForAccount(context.Context, string) (string, error)
}

type EmailMessage struct {
	Recipient, Subject, Body, ActionPath, IdempotencyKey string
}

type EmailSender interface {
	Send(context.Context, EmailMessage) error
}

type Service struct {
	pool      *pgxpool.Pool
	directory AccountDirectory
	sender    EmailSender
	clock     clock.Clock
}

func NewService(pool *pgxpool.Pool, directory AccountDirectory, sender EmailSender, c clock.Clock) (*Service, error) {
	if pool == nil || directory == nil || sender == nil {
		return nil, errors.New("notification pool, directory, and sender are required")
	}
	if c == nil {
		c = clock.System{}
	}
	return &Service{pool: pool, directory: directory, sender: sender, clock: c}, nil
}

func (s *Service) List(ctx context.Context, recipient string, limit int) ([]Notification, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `SELECT id,source_event_id,source_version,recipient_id,channel,purpose,template_version,title,body,action_path,scheduled_at,valid_until,delivery_status,attempt_count,last_error,sent_at,read_at,created_at,updated_at FROM notifications.notifications WHERE recipient_id=$1 AND channel='IN_APP' ORDER BY created_at DESC LIMIT $2`, recipient, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Notification, 0)
	for rows.Next() {
		var item Notification
		if err = scan(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) MarkRead(ctx context.Context, recipient, notificationID string) (Notification, error) {
	now := s.clock.Now().UTC()
	var item Notification
	err := s.pool.QueryRow(ctx, `UPDATE notifications.notifications SET read_at=COALESCE(read_at,$3),updated_at=$3 WHERE id=$1 AND recipient_id=$2 AND channel='IN_APP' RETURNING id,source_event_id,source_version,recipient_id,channel,purpose,template_version,title,body,action_path,scheduled_at,valid_until,delivery_status,attempt_count,last_error,sent_at,read_at,created_at,updated_at`, notificationID, recipient, now).Scan(notificationTargets(&item)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Notification{}, ErrNotFound
	}
	return item, err
}

func (s *Service) SetEmailReminders(ctx context.Context, accountID string, enabled bool) error {
	if accountID == "" {
		return ErrInvalid
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO notifications.preferences(account_id,email_reminders,updated_at) VALUES($1,$2,$3) ON CONFLICT(account_id) DO UPDATE SET email_reminders=EXCLUDED.email_reminders,updated_at=EXCLUDED.updated_at`, accountID, enabled, s.clock.Now().UTC())
	return err
}

func (s *Service) emailRemindersEnabled(ctx context.Context, accountID string) (bool, error) {
	var enabled bool
	err := s.pool.QueryRow(ctx, `SELECT email_reminders FROM notifications.preferences WHERE account_id=$1`, accountID).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	return enabled, err
}

func (s *Service) create(ctx context.Context, sourceEvent string, version int, recipient, purpose, title, body, action string, scheduled time.Time, validUntil *time.Time, email bool) error {
	if sourceEvent == "" || recipient == "" || purpose == "" || title == "" || body == "" || version <= 0 {
		return ErrInvalid
	}
	now := s.clock.Now().UTC()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	channels := []string{"IN_APP"}
	if email {
		channels = append(channels, "EMAIL")
	}
	for _, channel := range channels {
		notificationID, newErr := id.New()
		if newErr != nil {
			return newErr
		}
		status := "SENT"
		var sentAt *time.Time
		if channel == "EMAIL" {
			status = "PENDING"
		} else {
			sentAt = &now
		}
		var storedID string
		newErr = tx.QueryRow(ctx, `INSERT INTO notifications.notifications(id,source_event_id,source_version,recipient_id,channel,purpose,template_version,title,body,action_path,scheduled_at,valid_until,delivery_status,sent_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,1,$7,$8,$9,$10,$11,$12,$13,$14,$14) ON CONFLICT(source_event_id,source_version,recipient_id,channel,purpose) DO UPDATE SET source_event_id=EXCLUDED.source_event_id RETURNING id`, notificationID, sourceEvent, version, recipient, channel, purpose, title, body, action, scheduled.UTC(), validUntil, status, sentAt, now).Scan(&storedID)
		if newErr != nil {
			return newErr
		}
		if channel == "EMAIL" && !scheduled.After(now) {
			if _, newErr = outbox.Enqueue(ctx, tx, "notifications.deliver-email", "notification-email:"+storedID, map[string]string{"notificationId": storedID}, now); newErr != nil {
				return newErr
			}
		}
	}
	return tx.Commit(ctx)
}

func (s *Service) CancelReminders(ctx context.Context, participationID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE notifications.notifications SET delivery_status='CANCELLED',updated_at=$2 WHERE purpose LIKE $1 AND delivery_status IN('PENDING','RETRY_PENDING')`, "MATCH_REMINDER:"+participationID+":%", s.clock.Now().UTC())
	return err
}

func (s *Service) DispatchDue(ctx context.Context, limit int) (int, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	now := s.clock.Now().UTC()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(context.Background())
	rows, err := tx.Query(ctx, `SELECT id FROM notifications.notifications WHERE channel='EMAIL' AND delivery_status IN('PENDING','RETRY_PENDING') AND scheduled_at<=$1 AND (valid_until IS NULL OR valid_until>$1) ORDER BY scheduled_at FOR UPDATE SKIP LOCKED LIMIT $2`, now, limit)
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var notificationID string
		if err = rows.Scan(&notificationID); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, notificationID)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	for _, notificationID := range ids {
		if _, err = outbox.Enqueue(ctx, tx, "notifications.deliver-email", "notification-email:"+notificationID, map[string]string{"notificationId": notificationID}, now); err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(ids), nil
}

func scan(row pgx.Row, item *Notification) error { return row.Scan(notificationTargets(item)...) }

func notificationTargets(item *Notification) []any {
	return []any{&item.ID, &item.SourceEventID, &item.SourceVersion, &item.RecipientID, &item.Channel, &item.Purpose, &item.TemplateVersion, &item.Title, &item.Body, &item.ActionPath, &item.ScheduledAt, &item.ValidUntil, &item.DeliveryStatus, &item.AttemptCount, &item.LastError, &item.SentAt, &item.ReadAt, &item.CreatedAt, &item.UpdatedAt}
}

type CaptureEmailSender struct {
	mu        sync.Mutex
	messages  []EmailMessage
	delivered map[string]struct{}
}

func (c *CaptureEmailSender) Send(_ context.Context, message EmailMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.delivered == nil {
		c.delivered = make(map[string]struct{})
	}
	if _, exists := c.delivered[message.IdempotencyKey]; exists {
		return nil
	}
	c.delivered[message.IdempotencyKey] = struct{}{}
	c.messages = append(c.messages, message)
	return nil
}

func (c *CaptureEmailSender) Messages() []EmailMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]EmailMessage, len(c.messages))
	copy(result, c.messages)
	return result
}
