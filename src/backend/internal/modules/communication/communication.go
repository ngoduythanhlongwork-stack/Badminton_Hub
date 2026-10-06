package communication

import (
	"badmintonhub/internal/platform/clock"
	"badmintonhub/internal/platform/id"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrForbidden = errors.New("room action forbidden")
	ErrInvalid   = errors.New("invalid room message")
	ErrConflict  = errors.New("room command conflict")
)

type Access struct {
	CanRead, CanSend bool
	ReadThrough      *time.Time
}
type AccessProvider interface {
	RoomAccess(context.Context, string, string) (Access, error)
}
type Message struct {
	ID        string    `json:"id"`
	MatchID   string    `json:"matchId"`
	SenderID  string    `json:"senderId"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}
type Service struct {
	pool   *pgxpool.Pool
	access AccessProvider
	clock  clock.Clock
}

func NewService(pool *pgxpool.Pool, access AccessProvider, c clock.Clock) (*Service, error) {
	if pool == nil || access == nil {
		return nil, ErrInvalid
	}
	if c == nil {
		c = clock.System{}
	}
	return &Service{pool: pool, access: access, clock: c}, nil
}
func (s *Service) Send(ctx context.Context, actor, matchID, key, body string) (Message, error) {
	body = strings.TrimSpace(body)
	if key == "" || len([]rune(body)) < 1 || len([]rune(body)) > 2000 {
		return Message{}, ErrInvalid
	}
	a, err := s.access.RoomAccess(ctx, actor, matchID)
	if err != nil {
		return Message{}, err
	}
	if !a.CanSend {
		return Message{}, ErrForbidden
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Message{}, err
	}
	defer tx.Rollback(context.Background())
	sum := fmt.Sprintf("%x", sha256.Sum256([]byte(body)))
	var stored, idValue string
	err = tx.QueryRow(ctx, `SELECT fingerprint,message_id::text FROM communication.idempotency WHERE actor_id=$1 AND match_id=$2 AND idempotency_key=$3`, actor, matchID, key).Scan(&stored, &idValue)
	if err == nil {
		if stored != sum {
			return Message{}, ErrConflict
		}
		return s.byID(ctx, tx, idValue)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Message{}, err
	}
	message := Message{MatchID: matchID, SenderID: actor, Body: body, CreatedAt: s.clock.Now().UTC()}
	message.ID, _ = id.New()
	if _, err = tx.Exec(ctx, `INSERT INTO communication.messages(id,match_id,sender_id,body,created_at) VALUES($1,$2,$3,$4,$5)`, message.ID, matchID, actor, body, message.CreatedAt); err != nil {
		return Message{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO communication.idempotency(actor_id,match_id,idempotency_key,fingerprint,message_id,created_at) VALUES($1,$2,$3,$4,$5,$6)`, actor, matchID, key, sum, message.ID, message.CreatedAt); err != nil {
		return Message{}, ErrConflict
	}
	if err = tx.Commit(ctx); err != nil {
		return Message{}, err
	}
	return message, nil
}
func (s *Service) List(ctx context.Context, actor, matchID string, limit int) ([]Message, error) {
	a, err := s.access.RoomAccess(ctx, actor, matchID)
	if err != nil {
		return nil, err
	}
	if !a.CanRead {
		return nil, ErrForbidden
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	cutoff := s.clock.Now().UTC()
	if a.ReadThrough != nil {
		cutoff = *a.ReadThrough
	}
	rows, err := s.pool.Query(ctx, `SELECT id,match_id,sender_id,body,created_at FROM communication.messages WHERE match_id=$1 AND created_at<=$2 AND moderation_state='VISIBLE' ORDER BY created_at,id LIMIT $3`, matchID, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Message
	for rows.Next() {
		var m Message
		if err = rows.Scan(&m.ID, &m.MatchID, &m.SenderID, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (s *Service) byID(ctx context.Context, q rowQuerier, messageID string) (Message, error) {
	var m Message
	err := q.QueryRow(ctx, `SELECT id,match_id,sender_id,body,created_at FROM communication.messages WHERE id=$1`, messageID).Scan(&m.ID, &m.MatchID, &m.SenderID, &m.Body, &m.CreatedAt)
	return m, err
}
