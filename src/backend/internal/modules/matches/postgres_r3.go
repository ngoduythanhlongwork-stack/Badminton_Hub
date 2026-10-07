package matches

import (
	"badmintonhub/internal/platform/id"
	"badmintonhub/internal/platform/outbox"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (r *PostgresRepository) RecordAttendance(ctx context.Context, host, participationID, key string, target ParticipationStatus, reason string, now time.Time) (Participation, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Participation{}, err
	}
	defer tx.Rollback(context.Background())
	part, m, err := attendanceContext(ctx, tx, participationID)
	if err != nil {
		return Participation{}, err
	}
	if m.HostID != host {
		return Participation{}, ErrForbidden
	}
	if now.Before(m.StartAt.Add(-30*time.Minute)) || now.After(m.EndAt.Add(24*time.Hour)) {
		return Participation{}, ErrAttendanceWindow
	}
	fingerprint := participationID + ":" + string(target)
	if existing, ok, err := r.r3ParticipationResult(ctx, tx, host, "attendance", key, fingerprint); err != nil {
		return Participation{}, err
	} else if ok {
		return existing, tx.Commit(ctx)
	}
	if part.Status != ParticipationJoined && part.Status != ParticipationCheckedIn && part.Status != ParticipationNoShow {
		return Participation{}, ErrReviewNotAllowed
	}
	part.Status, part.AttendanceRevision, part.UpdatedAt = target, part.AttendanceRevision+1, now
	if _, err = tx.Exec(ctx, `UPDATE matches.participations SET status=$2,attendance_revision=$3,attendance_recorded_at=$4,attendance_recorded_by=$5,updated_at=$4 WHERE id=$1`, part.ID, part.Status, part.AttendanceRevision, now, host); err != nil {
		return Participation{}, err
	}
	if err = insertAttendanceRevision(ctx, tx, part, host, "HOST", "", reason, now); err != nil {
		return Participation{}, err
	}
	if err = r.enqueueAttendanceSignal(ctx, tx, part, now); err != nil {
		return Participation{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO matches.command_results(actor_id,action,idempotency_key,fingerprint,resource_type,resource_id,created_at) VALUES($1,'attendance',$2,$3,'participation',$4,$5)`, host, key, fingerprint, part.ID, now); err != nil {
		return Participation{}, mapR3Idempotency(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return Participation{}, err
	}
	return part, nil
}

func (r *PostgresRepository) CompleteDueMatches(ctx context.Context, now time.Time, limit int) (int, error) {
	if _, err := r.pool.Exec(ctx, `UPDATE matches.matches SET status='IN_PROGRESS',updated_at=$1 WHERE status IN('OPEN','FULL') AND start_at<=$1`, now); err != nil {
		return 0, err
	}
	rows, err := r.pool.Query(ctx, `SELECT id FROM matches.matches WHERE status IN('OPEN','FULL','IN_PROGRESS') AND end_at + interval '24 hours' <= $1 ORDER BY end_at FOR UPDATE SKIP LOCKED LIMIT $2`, now, limit)
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, matchID := range ids {
		tx, beginErr := r.pool.Begin(ctx)
		if beginErr != nil {
			return 0, beginErr
		}
		var end time.Time
		var status Status
		if err = tx.QueryRow(ctx, `SELECT end_at,status FROM matches.matches WHERE id=$1 FOR UPDATE`, matchID).Scan(&end, &status); err != nil {
			tx.Rollback(context.Background())
			return 0, err
		}
		if status == StatusCompleted || status == StatusCancelled || end.Add(24*time.Hour).After(now) {
			tx.Rollback(context.Background())
			continue
		}
		if _, err = tx.Exec(ctx, `UPDATE matches.matches SET status='COMPLETED',completed_at=$2,updated_at=$2 WHERE id=$1`, matchID, now); err != nil {
			tx.Rollback(context.Background())
			return 0, err
		}
		if _, err = tx.Exec(ctx, `UPDATE matches.participations SET status='COMPLETED',updated_at=$2 WHERE match_id=$1 AND status='CHECKED_IN'`, matchID, now); err != nil {
			tx.Rollback(context.Background())
			return 0, err
		}
		unknownRows, queryErr := tx.Query(ctx, `UPDATE matches.participations SET status='UNKNOWN',attendance_revision=attendance_revision+1,attendance_recorded_at=$2,updated_at=$2 WHERE match_id=$1 AND status='JOINED' RETURNING id,player_id,attendance_revision,created_at`, matchID, now)
		if queryErr != nil {
			tx.Rollback(context.Background())
			return 0, queryErr
		}
		var unknown []Participation
		for unknownRows.Next() {
			p := Participation{MatchID: matchID, Status: ParticipationUnknown, UpdatedAt: now}
			if err = unknownRows.Scan(&p.ID, &p.PlayerID, &p.AttendanceRevision, &p.CreatedAt); err != nil {
				unknownRows.Close()
				tx.Rollback(context.Background())
				return 0, err
			}
			unknown = append(unknown, p)
		}
		unknownRows.Close()
		for _, p := range unknown {
			if err = insertAttendanceRevision(ctx, tx, p, p.PlayerID, "SYSTEM", "", "completion cutoff", now); err != nil {
				tx.Rollback(context.Background())
				return 0, err
			}
		}
		if _, err = outbox.Enqueue(ctx, tx, "matches.completed", matchID+":completed", map[string]any{"matchId": matchID, "completedAt": now}, now); err != nil {
			tx.Rollback(context.Background())
			return 0, err
		}
		if err = tx.Commit(ctx); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

func (r *PostgresRepository) SubmitReview(ctx context.Context, review Review, key string, now time.Time) (Review, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Review{}, err
	}
	defer tx.Rollback(context.Background())
	var completedAt time.Time
	if err = tx.QueryRow(ctx, `SELECT completed_at FROM matches.matches WHERE id=$1 AND status='COMPLETED' FOR UPDATE`, review.MatchID).Scan(&completedAt); errors.Is(err, pgx.ErrNoRows) {
		return Review{}, ErrReviewNotAllowed
	} else if err != nil {
		return Review{}, err
	}
	if now.After(completedAt.Add(7 * 24 * time.Hour)) {
		return Review{}, ErrReviewWindow
	}
	var eligible int
	if err = tx.QueryRow(ctx, `SELECT count(DISTINCT player_id) FROM matches.participations WHERE match_id=$1 AND player_id IN($2,$3) AND status IN('CHECKED_IN','COMPLETED')`, review.MatchID, review.ReviewerID, review.TargetPlayerID).Scan(&eligible); err != nil {
		return Review{}, err
	}
	if eligible != 2 {
		return Review{}, ErrReviewNotAllowed
	}
	fingerprint := fmt.Sprintf("%s:%s:%d:%d:%v:%s", review.MatchID, review.TargetPlayerID, review.MatchQuality, review.HostRating, review.Tags, review.SkillFeedback)
	var storedFingerprint, existingID string
	err = tx.QueryRow(ctx, `SELECT fingerprint,resource_id::text FROM matches.command_results WHERE actor_id=$1 AND action='review' AND idempotency_key=$2`, review.ReviewerID, key).Scan(&storedFingerprint, &existingID)
	if err == nil {
		if storedFingerprint != fingerprint {
			return Review{}, ErrIdempotencyConflict
		}
		return r.reviewByID(ctx, tx, existingID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Review{}, err
	}
	review.ID, _ = id.New()
	review.Revision = 1
	review.SubmittedAt = now
	_, err = tx.Exec(ctx, `INSERT INTO matches.reviews(id,match_id,reviewer_id,target_player_id,match_quality,host_rating,tags,skill_feedback,revision,submitted_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,1,$9)`, review.ID, review.MatchID, review.ReviewerID, review.TargetPlayerID, review.MatchQuality, review.HostRating, review.Tags, review.SkillFeedback, now)
	if err != nil {
		return Review{}, mapConflict(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO matches.command_results(actor_id,action,idempotency_key,fingerprint,resource_type,resource_id,created_at) VALUES($1,'review',$2,$3,'review',$4,$5)`, review.ReviewerID, key, fingerprint, review.ID, now); err != nil {
		return Review{}, mapR3Idempotency(err)
	}
	event := TrustSignalEvent{EventID: review.ID, AccountID: review.TargetPlayerID, SourceType: "SKILL_FEEDBACK", SourceID: review.ID, SourceRevision: 1, MatchID: review.MatchID, SkillDirection: review.SkillFeedback, OccurredAt: now}
	if _, err = outbox.Enqueue(ctx, tx, "matches.trust-signal", review.ID+":1", event, now); err != nil {
		return Review{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Review{}, err
	}
	return review, nil
}

func (r *PostgresRepository) CorrectReview(ctx context.Context, admin, reviewID, caseID, key string, replacement Review, reason string, now time.Time) (Review, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Review{}, err
	}
	defer tx.Rollback(context.Background())

	var current Review
	var completedAt time.Time
	err = tx.QueryRow(ctx, `SELECT r.id,r.match_id,r.reviewer_id,r.target_player_id,r.match_quality,r.host_rating,r.tags,r.skill_feedback,r.revision,r.submitted_at,m.completed_at
FROM matches.reviews r JOIN matches.matches m ON m.id=r.match_id WHERE r.id=$1 FOR UPDATE OF r,m`, reviewID).Scan(&current.ID, &current.MatchID, &current.ReviewerID, &current.TargetPlayerID, &current.MatchQuality, &current.HostRating, &current.Tags, &current.SkillFeedback, &current.Revision, &current.SubmittedAt, &completedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Review{}, ErrNotFound
	}
	if err != nil {
		return Review{}, err
	}
	if now.After(completedAt.Add(30 * 24 * time.Hour)) {
		return Review{}, ErrReviewWindow
	}
	fingerprint := fmt.Sprintf("%s:%s:%d:%d:%v:%s:%s", reviewID, caseID, replacement.MatchQuality, replacement.HostRating, replacement.Tags, replacement.SkillFeedback, reason)
	var storedFingerprint, storedID string
	err = tx.QueryRow(ctx, `SELECT fingerprint,resource_id::text FROM matches.command_results WHERE actor_id=$1 AND action='review-correction' AND idempotency_key=$2`, admin, key).Scan(&storedFingerprint, &storedID)
	if err == nil {
		if storedFingerprint != fingerprint {
			return Review{}, ErrIdempotencyConflict
		}
		result, readErr := r.reviewByID(ctx, tx, storedID)
		if readErr != nil {
			return Review{}, readErr
		}
		return result, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Review{}, err
	}

	revisionID, _ := id.New()
	replacement.ID = current.ID
	replacement.MatchID = current.MatchID
	replacement.ReviewerID = current.ReviewerID
	replacement.TargetPlayerID = current.TargetPlayerID
	replacement.Revision = current.Revision + 1
	replacement.SubmittedAt = current.SubmittedAt
	replacement.CorrectedAt = &now
	replacement.CorrectedBy = admin
	_, err = tx.Exec(ctx, `INSERT INTO matches.review_revisions(id,review_id,match_id,revision,previous_match_quality,previous_host_rating,previous_tags,previous_skill_feedback,new_match_quality,new_host_rating,new_tags,new_skill_feedback,actor_id,case_id,reason,occurred_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`, revisionID, reviewID, current.MatchID, replacement.Revision, current.MatchQuality, current.HostRating, current.Tags, current.SkillFeedback, replacement.MatchQuality, replacement.HostRating, replacement.Tags, replacement.SkillFeedback, admin, caseID, reason, now)
	if err != nil {
		return Review{}, err
	}
	_, err = tx.Exec(ctx, `UPDATE matches.reviews SET match_quality=$2,host_rating=$3,tags=$4,skill_feedback=$5,revision=$6,corrected_at=$7,corrected_by=$8 WHERE id=$1`, reviewID, replacement.MatchQuality, replacement.HostRating, replacement.Tags, replacement.SkillFeedback, replacement.Revision, now, admin)
	if err != nil {
		return Review{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO matches.command_results(actor_id,action,idempotency_key,fingerprint,resource_type,resource_id,created_at) VALUES($1,'review-correction',$2,$3,'review',$4,$5)`, admin, key, fingerprint, reviewID, now); err != nil {
		return Review{}, mapR3Idempotency(err)
	}
	eventID, _ := id.New()
	event := TrustSignalEvent{EventID: eventID, AccountID: current.TargetPlayerID, SourceType: "SKILL_FEEDBACK", SourceID: reviewID, SourceRevision: replacement.Revision, MatchID: current.MatchID, SkillDirection: replacement.SkillFeedback, OccurredAt: now}
	if _, err = outbox.Enqueue(ctx, tx, "matches.trust-signal", fmt.Sprintf("%s:%d", reviewID, replacement.Revision), event, now); err != nil {
		return Review{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Review{}, err
	}
	return replacement, nil
}

func (r *PostgresRepository) CorrectAttendance(ctx context.Context, admin, participationID, caseID, key string, target ParticipationStatus, reason string, now time.Time) (Participation, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Participation{}, err
	}
	defer tx.Rollback(context.Background())
	part, m, err := attendanceContext(ctx, tx, participationID)
	if err != nil {
		return Participation{}, err
	}
	if m.CompletedAt == nil || now.After(m.CompletedAt.Add(30*24*time.Hour)) {
		return Participation{}, ErrAttendanceWindow
	}
	fingerprint := participationID + ":" + caseID + ":" + string(target) + ":" + reason
	if existing, ok, resultErr := r.r3ParticipationResult(ctx, tx, admin, "attendance-correction", key, fingerprint); resultErr != nil {
		return Participation{}, resultErr
	} else if ok {
		return existing, tx.Commit(ctx)
	}
	if part.Status != ParticipationCompleted && part.Status != ParticipationCheckedIn && part.Status != ParticipationNoShow && part.Status != ParticipationUnknown {
		return Participation{}, ErrReviewNotAllowed
	}
	part.Status = target
	part.AttendanceRevision++
	part.UpdatedAt = now
	if _, err = tx.Exec(ctx, `UPDATE matches.participations SET status=$2,attendance_revision=$3,attendance_recorded_at=$4,attendance_recorded_by=$5,updated_at=$4 WHERE id=$1`, part.ID, target, part.AttendanceRevision, now, admin); err != nil {
		return Participation{}, err
	}
	if err = insertAttendanceRevision(ctx, tx, part, admin, "ADMIN_CASE", caseID, reason, now); err != nil {
		return Participation{}, err
	}
	if err = r.enqueueAttendanceSignal(ctx, tx, part, now); err != nil {
		return Participation{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO matches.command_results(actor_id,action,idempotency_key,fingerprint,resource_type,resource_id,created_at) VALUES($1,'attendance-correction',$2,$3,'participation',$4,$5)`, admin, key, fingerprint, part.ID, now); err != nil {
		return Participation{}, mapR3Idempotency(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return Participation{}, err
	}
	return part, nil
}

func attendanceContext(ctx context.Context, tx pgx.Tx, participationID string) (Participation, Match, error) {
	var p Participation
	var m Match
	err := tx.QueryRow(ctx, `SELECT p.id,p.match_id,p.player_id,p.status,p.decision_reason,p.created_at,p.updated_at,p.attendance_revision,m.host_id,m.start_at,m.end_at,m.completed_at FROM matches.participations p JOIN matches.matches m ON m.id=p.match_id WHERE p.id=$1 FOR UPDATE OF p,m`, participationID).Scan(&p.ID, &p.MatchID, &p.PlayerID, &p.Status, &p.DecisionReason, &p.CreatedAt, &p.UpdatedAt, &p.AttendanceRevision, &m.HostID, &m.StartAt, &m.EndAt, &m.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return p, m, err
}

func insertAttendanceRevision(ctx context.Context, tx pgx.Tx, p Participation, actor, source, caseID, reason string, now time.Time) error {
	revisionID, _ := id.New()
	var caseValue any
	if caseID != "" {
		caseValue = caseID
	}
	_, err := tx.Exec(ctx, `INSERT INTO matches.attendance_revisions(id,participation_id,match_id,player_id,revision,previous_status,new_status,actor_id,source,case_id,reason,occurred_at) VALUES($1,$2,$3,$4,$5,COALESCE((SELECT new_status FROM matches.attendance_revisions WHERE participation_id=$2 ORDER BY revision DESC LIMIT 1),'JOINED'),$6,$7,$8,$9,$10,$11)`, revisionID, p.ID, p.MatchID, p.PlayerID, p.AttendanceRevision, p.Status, actor, source, caseValue, reason, now)
	return err
}

func (r *PostgresRepository) enqueueAttendanceSignal(ctx context.Context, tx pgx.Tx, p Participation, now time.Time) error {
	var value *float64
	if p.Status == ParticipationCheckedIn || p.Status == ParticipationCompleted {
		v := 1.0
		value = &v
	} else if p.Status == ParticipationNoShow {
		v := 0.0
		value = &v
	}
	eventID, _ := id.New()
	event := TrustSignalEvent{EventID: eventID, AccountID: p.PlayerID, SourceType: "ATTENDANCE", SourceID: p.ID, SourceRevision: p.AttendanceRevision, MatchID: p.MatchID, ReliabilityValue: value, OccurredAt: now}
	_, err := outbox.Enqueue(ctx, tx, "matches.trust-signal", p.ID+fmt.Sprintf(":%d", p.AttendanceRevision), event, now)
	return err
}

func (r *PostgresRepository) r3ParticipationResult(ctx context.Context, tx pgx.Tx, actor, action, key, fingerprint string) (Participation, bool, error) {
	var stored, id string
	err := tx.QueryRow(ctx, `SELECT fingerprint,resource_id::text FROM matches.command_results WHERE actor_id=$1 AND action=$2 AND idempotency_key=$3`, actor, action, key).Scan(&stored, &id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Participation{}, false, nil
	}
	if err != nil {
		return Participation{}, false, err
	}
	if stored != fingerprint {
		return Participation{}, false, ErrIdempotencyConflict
	}
	p, _, err := attendanceContext(ctx, tx, id)
	return p, true, err
}

func (r *PostgresRepository) reviewByID(ctx context.Context, q rowQuerier, id string) (Review, error) {
	var v Review
	err := q.QueryRow(ctx, `SELECT id,match_id,reviewer_id,target_player_id,match_quality,host_rating,tags,skill_feedback,revision,submitted_at,corrected_at,coalesce(corrected_by::text,'') FROM matches.reviews WHERE id=$1`, id).Scan(&v.ID, &v.MatchID, &v.ReviewerID, &v.TargetPlayerID, &v.MatchQuality, &v.HostRating, &v.Tags, &v.SkillFeedback, &v.Revision, &v.SubmittedAt, &v.CorrectedAt, &v.CorrectedBy)
	return v, err
}

func mapR3Idempotency(err error) error {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		return ErrIdempotencyConflict
	}
	return err
}

func (r *PostgresRepository) RoomAccess(ctx context.Context, accountID, matchID string, now time.Time) (RoomAccess, error) {
	var hostID string
	var status Status
	var endAt time.Time
	err := r.pool.QueryRow(ctx, `SELECT host_id,status,end_at FROM matches.matches WHERE id=$1`, matchID).Scan(&hostID, &status, &endAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return RoomAccess{}, ErrNotFound
	}
	if err != nil {
		return RoomAccess{}, err
	}
	readDeadline, sendDeadline := endAt.Add(30*24*time.Hour), endAt.Add(7*24*time.Hour)
	if accountID == hostID {
		return RoomAccess{CanRead: !now.After(readDeadline), CanSend: status != StatusCancelled && !now.After(sendDeadline)}, nil
	}
	var partStatus ParticipationStatus
	var lostAt time.Time
	err = r.pool.QueryRow(ctx, `SELECT status,updated_at FROM matches.participations WHERE match_id=$1 AND player_id=$2 ORDER BY created_at DESC LIMIT 1`, matchID, accountID).Scan(&partStatus, &lostAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return RoomAccess{}, ErrForbidden
	}
	if err != nil {
		return RoomAccess{}, err
	}
	switch partStatus {
	case ParticipationJoined, ParticipationCheckedIn, ParticipationCompleted:
		return RoomAccess{CanRead: !now.After(readDeadline), CanSend: status != StatusCancelled && !now.After(sendDeadline)}, nil
	case ParticipationCancelled, ParticipationRemoved, ParticipationNoShow, ParticipationUnknown:
		if now.After(readDeadline) {
			return RoomAccess{}, nil
		}
		return RoomAccess{CanRead: true, ReadThrough: &lostAt}, nil
	default:
		return RoomAccess{}, nil
	}
}

func (r *PostgresRepository) RecommendationCandidates(ctx context.Context, accountID string, now time.Time, limit int) ([]RecommendationCandidate, error) {
	rows, err := r.pool.Query(ctx, `WITH occupancy AS MATERIALIZED (
  SELECT p.match_id,count(*) occupied
  FROM matches.participations p
  LEFT JOIN matches.holds h ON h.participation_id=p.id
	WHERE p.status IN('JOINED','CHECKED_IN') OR (p.status='AWAITING_PAYMENT' AND h.status='HELD' AND h.expires_at>$2)
  GROUP BY p.match_id
), player_schedule AS MATERIALIZED (
  SELECT start_at,end_at FROM matches.participations
  WHERE player_id=$1 AND status IN('AWAITING_PAYMENT','JOINED','CHECKED_IN')
)
SELECT m.id,m.host_id,m.venue_area,m.venue_time_zone,m.format,m.style,m.start_at,m.end_at,m.min_level,m.max_level,m.join_mode
FROM matches.matches m LEFT JOIN occupancy o ON o.match_id=m.id
WHERE m.status IN('OPEN','FULL') AND m.start_at>$2 AND COALESCE(o.occupied,0)<m.capacity
AND NOT EXISTS(SELECT 1 FROM player_schedule s WHERE s.start_at<m.end_at AND s.end_at>m.start_at)
ORDER BY m.start_at,m.id LIMIT $3`, accountID, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]RecommendationCandidate, 0, limit)
	for rows.Next() {
		var item RecommendationCandidate
		if err = rows.Scan(&item.ID, &item.HostID, &item.Area, &item.TimeZone, &item.Format, &item.Style, &item.StartAt, &item.EndAt, &item.MinLevel, &item.MaxLevel, &item.JoinMode); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
