package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"badmintonhub/internal/platform/outbox"
	"github.com/jackc/pgx/v5"
)

type event struct {
	EventID, Kind, RecipientID, HostID, MatchID, ParticipationID string
	StartAt, OccurredAt                                          time.Time
}

func (s *Service) PaymentEventHandler() outbox.Handler {
	return func(ctx context.Context, message outbox.Message) error {
		var payload event
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			return errors.New("decode payment notification event")
		}
		title, body := paymentCopy(payload.Kind)
		if title == "" {
			return ErrInvalid
		}
		return s.create(ctx, payload.EventID, 1, payload.RecipientID, payload.Kind, title, body, "/payments", payload.OccurredAt, nil, true)
	}
}

func paymentCopy(kind string) (string, string) {
	switch kind {
	case "TRANSFER_REPORTED":
		return "Có báo chuyển khoản mới", "Người chơi đã báo chuyển khoản. Vui lòng kiểm tra giao dịch trước khi xác nhận."
	case "RECEIPT_ACKNOWLEDGED":
		return "Host đã ghi nhận tiền", "Host đã ghi nhận khoản thực nhận. Trạng thái tham gia được xác nhận riêng."
	case "TRANSFER_NOT_FOUND":
		return "Host chưa tìm thấy giao dịch", "Host chưa tìm thấy khoản chuyển và đã để lại lý do đối soát."
	case "REFUND_DUE":
		return "Có khoản cần hoàn", "Một nghĩa vụ hoàn tiền đã được tạo và cần xử lý trong 48 giờ."
	case "REFUND_REPORTED_SENT":
		return "Host đã báo gửi hoàn tiền", "Host đã báo gửi khoản hoàn. Vui lòng kiểm tra và xác nhận hoặc tranh chấp."
	default:
		return "", ""
	}
}

func (s *Service) JoinedHandler() outbox.Handler {
	return func(ctx context.Context, message outbox.Message) error {
		var payload event
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			return errors.New("decode joined event")
		}
		if err := s.create(ctx, payload.EventID, 1, payload.RecipientID, "MATCH_JOINED", "Đã xác nhận tham gia", "Suất của bạn đã được xác nhận.", "/matches/"+payload.MatchID, payload.OccurredAt, &payload.StartAt, true); err != nil {
			return err
		}
		emailReminders, err := s.emailRemindersEnabled(ctx, payload.RecipientID)
		if err != nil {
			return err
		}
		for _, offset := range []struct {
			name     string
			duration time.Duration
		}{{"24H", 24 * time.Hour}, {"2H", 2 * time.Hour}} {
			scheduled := payload.StartAt.Add(-offset.duration)
			if scheduled.After(payload.OccurredAt) {
				purpose := "MATCH_REMINDER:" + payload.ParticipationID + ":" + offset.name
				if err := s.create(ctx, payload.EventID+":reminder:"+offset.name, 1, payload.RecipientID, purpose, "Sắp đến giờ chơi", "Kèo cầu lông sắp bắt đầu. Hãy kiểm tra lại giờ và sân trong ứng dụng.", "/matches/"+payload.MatchID, scheduled, &payload.StartAt, emailReminders); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

func (s *Service) CancellationHandler() outbox.Handler {
	type cancellation struct {
		EventID, MatchID, ParticipationID, PlayerID, HostID, Cause string
		OccurredAt                                                 time.Time `json:"occurredAt"`
	}
	return func(ctx context.Context, message outbox.Message) error {
		var payload cancellation
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			return errors.New("decode cancellation notification event")
		}
		if err := s.CancelReminders(ctx, payload.ParticipationID); err != nil {
			return err
		}
		recipient := payload.PlayerID
		if payload.Cause == "PLAYER_WITHDRAWAL" {
			recipient = payload.HostID
		}
		return s.create(ctx, payload.EventID, 1, recipient, "MATCH_CANCELLED", "Thay đổi lượt tham gia", "Lượt tham gia đã được hủy. Trạng thái hoàn tiền, nếu có, được theo dõi riêng.", "/matches/"+payload.MatchID, payload.OccurredAt, nil, true)
	}
}

func (s *Service) EmailDeliveryHandler() outbox.Handler {
	type payload struct {
		NotificationID string `json:"notificationId"`
	}
	return func(ctx context.Context, message outbox.Message) error {
		var body payload
		if err := json.Unmarshal(message.Payload, &body); err != nil {
			return errors.New("decode notification email obligation")
		}
		var item Notification
		err := scan(s.pool.QueryRow(ctx, `SELECT id,source_event_id,source_version,recipient_id,channel,purpose,template_version,title,body,action_path,scheduled_at,valid_until,delivery_status,attempt_count,last_error,sent_at,read_at,created_at,updated_at FROM notifications.notifications WHERE id=$1`, body.NotificationID), &item)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		now := s.clock.Now().UTC()
		if item.DeliveryStatus == "SENT" || item.DeliveryStatus == "CANCELLED" || item.DeliveryStatus == "ABANDONED" || (item.ValidUntil != nil && !item.ValidUntil.After(now)) {
			return nil
		}
		email, err := s.directory.EmailForAccount(ctx, item.RecipientID)
		if err != nil {
			return s.recordDeliveryFailure(ctx, item.ID, message.Attempt, err)
		}
		err = s.sender.Send(ctx, EmailMessage{Recipient: email, Subject: item.Title, Body: item.Body, ActionPath: item.ActionPath, IdempotencyKey: item.ID})
		if err != nil {
			return s.recordDeliveryFailure(ctx, item.ID, message.Attempt, err)
		}
		_, err = s.pool.Exec(ctx, `UPDATE notifications.notifications SET delivery_status='SENT',attempt_count=$2,last_error='',sent_at=$3,updated_at=$3 WHERE id=$1`, item.ID, message.Attempt, now)
		return err
	}
}

func (s *Service) recordDeliveryFailure(ctx context.Context, notificationID string, attempt int, deliveryErr error) error {
	now := s.clock.Now().UTC()
	lastError := deliveryErr.Error()
	if len(lastError) > 300 {
		lastError = lastError[:300]
	}
	status := "RETRY_PENDING"
	if attempt >= 3 {
		status = "ABANDONED"
	}
	_, err := s.pool.Exec(ctx, `UPDATE notifications.notifications SET delivery_status=$2,attempt_count=$3,last_error=$4,updated_at=$5 WHERE id=$1`, notificationID, status, attempt, lastError, now)
	if err != nil {
		return err
	}
	if status == "ABANDONED" {
		return nil
	}
	return fmt.Errorf("email delivery failed: %w", deliveryErr)
}
