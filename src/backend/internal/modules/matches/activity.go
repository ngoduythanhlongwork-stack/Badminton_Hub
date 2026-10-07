package matches

import (
	"context"
	"time"
)

func (r *PostgresRepository) ListActivity(ctx context.Context, accountID, view string, now time.Time, limit int) ([]ActivityItem, error) {
	base := `SELECT m.id,m.title,m.venue_name,m.venue_area,m.venue_time_zone,m.format,m.style,m.start_at,m.end_at,m.status,coalesce(p.id::text,''),coalesce(p.status,'') FROM matches.matches m LEFT JOIN matches.participations p ON p.match_id=m.id AND p.player_id=$1 `
	var query string
	switch view {
	case "upcoming":
		query = base + `WHERE p.id IS NOT NULL AND p.status IN('REQUESTED','AWAITING_PAYMENT','JOINED','CHECKED_IN') AND m.end_at>$2 ORDER BY m.start_at,m.id LIMIT $3`
	case "history":
		query = base + `WHERE p.id IS NOT NULL AND (m.end_at<=$2 OR m.status IN('COMPLETED','CANCELLED') OR p.status IN('COMPLETED','NO_SHOW','UNKNOWN','CANCELLED','REMOVED','REJECTED','EXPIRED')) ORDER BY m.end_at DESC,m.id LIMIT $3`
	case "hosted":
		query = base + `WHERE m.host_id=$1 ORDER BY CASE WHEN m.end_at>$2 THEN 0 ELSE 1 END,m.start_at,m.id LIMIT $3`
	default:
		return nil, ErrInvalid
	}
	rows, err := r.pool.Query(ctx, query, accountID, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ActivityItem{}
	for rows.Next() {
		var item ActivityItem
		var participationStatus string
		if err = rows.Scan(&item.MatchID, &item.Title, &item.VenueName, &item.Area, &item.TimeZone, &item.Format, &item.Style, &item.StartAt, &item.EndAt, &item.MatchStatus, &item.ParticipationID, &participationStatus); err != nil {
			return nil, err
		}
		if view == "hosted" {
			item.Role = "HOST"
		} else {
			item.Role = "PLAYER"
		}
		item.ParticipationState = ParticipationStatus(participationStatus)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *PostgresRepository) ListParticipants(ctx context.Context, hostID, matchID string) ([]ParticipantSummary, error) {
	var allowed bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM matches.matches WHERE id=$1 AND host_id=$2)`, matchID, hostID).Scan(&allowed); err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrForbidden
	}
	rows, err := r.pool.Query(ctx, `SELECT p.id,p.match_id,p.player_id,p.status,p.decision_reason,p.attendance_revision,p.created_at,p.updated_at,h.status,h.expires_at FROM matches.participations p LEFT JOIN matches.holds h ON h.participation_id=p.id WHERE p.match_id=$1 ORDER BY p.created_at,p.id`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ParticipantSummary{}
	for rows.Next() {
		var item ParticipantSummary
		var holdStatus *string
		if err = rows.Scan(&item.ID, &item.MatchID, &item.PlayerID, &item.Status, &item.DecisionReason, &item.AttendanceRevision, &item.CreatedAt, &item.UpdatedAt, &holdStatus, &item.HoldExpiresAt); err != nil {
			return nil, err
		}
		if holdStatus != nil {
			item.HoldStatus = HoldStatus(*holdStatus)
		}
		result = append(result, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		var exists bool
		if e := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM matches.matches WHERE id=$1)`, matchID).Scan(&exists); e != nil {
			return nil, e
		}
		if !exists {
			return nil, ErrNotFound
		}
	}
	return result, nil
}

var _ ActivityRepository = (*PostgresRepository)(nil)
