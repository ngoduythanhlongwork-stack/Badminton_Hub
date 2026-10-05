//go:build integration

package matches

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"badmintonhub/internal/modules/venues"
	platformmigrations "badmintonhub/internal/platform/migrations"
	"badmintonhub/internal/platform/testdb"
)

func integrationRepo(t *testing.T) *PostgresRepository {
	t.Helper()
	p := testdb.Open(t)
	runner, e := platformmigrations.NewRunner(p, venues.Catalog(), Catalog())
	if e != nil {
		t.Fatal(e)
	}
	if e = runner.Up(context.Background()); e != nil {
		t.Fatal(e)
	}
	return NewPostgresRepository(p)
}
func seededOpen(t *testing.T, r *PostgresRepository, id, host string, start, end time.Time, capacity int) Match {
	t.Helper()
	m := Match{ID: id, HostID: host, Title: "Kèo", Description: "Mô tả", Venue: VenueSnapshot{ID: "10000000-0000-4000-8000-000000000001", Name: "Sân", Address: "Q1", Area: "Q1", TimeZone: "Asia/Ho_Chi_Minh"}, StartAt: start, EndAt: end, Format: "DOUBLES", Style: "SOCIAL", Rules: "Fair play", MinLevel: 1, MaxLevel: 6, Capacity: capacity, Currency: "VND", PaymentInstructionVersion: 1, CancellationPolicyVersion: "MVP-2026-10-D15", JoinMode: JoinInstant, CourtAttested: true, CourtAttestedAt: time.Now().UTC(), Status: StatusDraft, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if _, e := r.Create(context.Background(), m); e != nil {
		t.Fatal(e)
	}
	out, e := r.Publish(context.Background(), host, id, time.Now().UTC())
	if e != nil {
		t.Fatal(e)
	}
	return out
}

func seededPaid(t *testing.T, r *PostgresRepository, id, host string, start time.Time, capacity int) Match {
	t.Helper()
	m := Match{ID: id, HostID: host, Title: "Kèo có cọc", Description: "Mô tả", Venue: VenueSnapshot{ID: "10000000-0000-4000-8000-000000000001", Name: "Sân", Address: "Q1", Area: "Q1", TimeZone: "Asia/Ho_Chi_Minh"}, StartAt: start, EndAt: start.Add(time.Hour), Format: "DOUBLES", Style: "SOCIAL", Rules: "Fair play", MinLevel: 1, MaxLevel: 6, Capacity: capacity, FeeMinor: 100_000, DepositMinor: 30_000, Currency: "VND", PaymentRecipient: "Host", PaymentInstructions: "Chuyển khoản theo mã lượt", PaymentInstructionVersion: 1, CancellationPolicyVersion: "MVP-2026-10-D15", JoinMode: JoinInstant, CourtAttested: true, CourtAttestedAt: time.Now().UTC(), Status: StatusDraft, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if _, err := r.Create(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	out, err := r.Publish(context.Background(), host, id, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func TestConcurrentLastSlotIsNeverOverAllocated(t *testing.T) {
	r := integrationRepo(t)
	start := time.Now().UTC().Add(24 * time.Hour)
	mid := "20000000-0000-4000-8000-000000000001"
	seededOpen(t, r, mid, "30000000-0000-4000-8000-000000000001", start, start.Add(time.Hour), 1)
	players := []string{"40000000-0000-4000-8000-000000000001", "40000000-0000-4000-8000-000000000002"}
	ready := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range players {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-ready
			_, errs[i] = r.Join(context.Background(), players[i], mid, "race", time.Now().UTC())
		}(i)
	}
	close(ready)
	wg.Wait()
	success, full := 0, 0
	for _, e := range errs {
		if e == nil {
			success++
		} else if errors.Is(e, ErrFull) || errors.Is(e, ErrNotOpen) {
			full++
		} else {
			t.Fatalf("unexpected race error: %v", e)
		}
	}
	if success != 1 || full != 1 {
		t.Fatalf("success=%d full=%d", success, full)
	}
}
func TestScheduleOverlapRejectsButAdjacentAllows(t *testing.T) {
	r := integrationRepo(t)
	start := time.Now().UTC().Add(24 * time.Hour)
	player := "50000000-0000-4000-8000-000000000001"
	host := "60000000-0000-4000-8000-000000000001"
	m1 := "70000000-0000-4000-8000-000000000001"
	m2 := "70000000-0000-4000-8000-000000000002"
	m3 := "70000000-0000-4000-8000-000000000003"
	seededOpen(t, r, m1, host, start, start.Add(time.Hour), 2)
	seededOpen(t, r, m2, host, start.Add(time.Hour), start.Add(2*time.Hour), 2)
	seededOpen(t, r, m3, host, start.Add(30*time.Minute), start.Add(90*time.Minute), 2)
	if _, e := r.Join(context.Background(), player, m1, "one", time.Now().UTC()); e != nil {
		t.Fatal(e)
	}
	if _, e := r.Join(context.Background(), player, m2, "two", time.Now().UTC()); e != nil {
		t.Fatalf("adjacent rejected: %v", e)
	}
	if _, e := r.Join(context.Background(), player, m3, "three", time.Now().UTC()); !errors.Is(e, ErrScheduleConflict) {
		t.Fatalf("overlap result: %v", e)
	}
}

func TestConcurrentSameIdempotencyKeyReturnsOneParticipation(t *testing.T) {
	r := integrationRepo(t)
	start := time.Now().UTC().Add(24 * time.Hour)
	matchID := "80000000-0000-4000-8000-000000000001"
	playerID := "81000000-0000-4000-8000-000000000001"
	seededOpen(t, r, matchID, "82000000-0000-4000-8000-000000000001", start, start.Add(time.Hour), 2)
	ready := make(chan struct{})
	results := make([]Participation, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for index := range results {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-ready
			results[index], errs[index] = r.Join(context.Background(), playerID, matchID, "same-command", time.Now().UTC())
		}(index)
	}
	close(ready)
	wg.Wait()
	if errs[0] != nil || errs[1] != nil || results[0].ID == "" || results[0].ID != results[1].ID {
		t.Fatalf("results=%+v errors=%v", results, errs)
	}
}

func TestPaidHoldOccupiesSlotAndExpiryReportRaceRevivesIt(t *testing.T) {
	r := integrationRepo(t)
	t0 := time.Now().UTC()
	matchID := "90000000-0000-4000-8000-000000000001"
	hostID := "91000000-0000-4000-8000-000000000001"
	playerID := "92000000-0000-4000-8000-000000000001"
	seededPaid(t, r, matchID, hostID, t0.Add(4*time.Hour), 1)
	participation, err := r.Join(context.Background(), playerID, matchID, "paid-join", t0)
	if err != nil {
		t.Fatal(err)
	}
	if participation.Status != ParticipationAwaitingPayment || participation.Hold == nil || !participation.Hold.ExpiresAt.Equal(t0.Add(30*time.Minute)) {
		t.Fatalf("participation=%+v", participation)
	}
	if _, err = r.Join(context.Background(), "93000000-0000-4000-8000-000000000001", matchID, "last-slot", t0); !errors.Is(err, ErrNotOpen) && !errors.Is(err, ErrFull) {
		t.Fatalf("second join error=%v", err)
	}
	if count, expireErr := r.ExpireDueHolds(context.Background(), t0.Add(31*time.Minute), 10); expireErr != nil || count != 1 {
		t.Fatalf("expired=%d error=%v", count, expireErr)
	}
	revived, err := r.ExtendHold(context.Background(), playerID, participation.ID, t0.Add(29*time.Minute))
	if err != nil {
		t.Fatalf("valid report lost expiry race: %v", err)
	}
	if revived.Status != ParticipationAwaitingPayment || revived.Hold == nil || !revived.Hold.ExpiresAt.Equal(t0.Add(149*time.Minute)) {
		t.Fatalf("revived=%+v", revived)
	}
	again, err := r.ExtendHold(context.Background(), playerID, participation.ID, t0.Add(60*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if delta := again.Hold.ExpiresAt.Sub(t0.Add(149 * time.Minute)); delta < -time.Microsecond || delta > time.Microsecond {
		t.Fatalf("repeated report extended deadline to %s", again.Hold.ExpiresAt)
	}
}

func TestLateAcknowledgementNeverRestoresParticipation(t *testing.T) {
	r := integrationRepo(t)
	t0 := time.Now().UTC()
	matchID := "94000000-0000-4000-8000-000000000001"
	hostID := "95000000-0000-4000-8000-000000000001"
	seededPaid(t, r, matchID, hostID, t0.Add(4*time.Hour), 1)
	participation, err := r.Join(context.Background(), "96000000-0000-4000-8000-000000000001", matchID, "paid-late", t0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.ConfirmPayment(context.Background(), hostID, participation.ID, t0.Add(31*time.Minute)); !errors.Is(err, ErrHoldExpired) {
		t.Fatalf("late acknowledgement error=%v", err)
	}
	stored, err := r.Participation(context.Background(), participation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != ParticipationExpired {
		t.Fatalf("status=%s", stored.Status)
	}
}

func TestHostCancellationReleasesJoinedParticipation(t *testing.T) {
	r := integrationRepo(t)
	t0 := time.Now().UTC()
	matchID := "97000000-0000-4000-8000-000000000001"
	hostID := "98000000-0000-4000-8000-000000000001"
	seededPaid(t, r, matchID, hostID, t0.Add(8*time.Hour), 1)
	participation, err := r.Join(context.Background(), "99000000-0000-4000-8000-000000000001", matchID, "cancel-join", t0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.ExtendHold(context.Background(), participation.PlayerID, participation.ID, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err = r.ConfirmPayment(context.Background(), hostID, participation.ID, t0.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err = r.CancelMatch(context.Background(), hostID, matchID, "cancel-match", "Sân đóng cửa", t0.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	stored, err := r.Participation(context.Background(), participation.ID)
	if err != nil || stored.Status != ParticipationCancelled {
		t.Fatalf("stored=%+v error=%v", stored, err)
	}
}

func TestPublishedCapacityCannotDropBelowOccupied(t *testing.T) {
	r := integrationRepo(t)
	t0 := time.Now().UTC()
	matchID := "a1000000-0000-4000-8000-000000000001"
	hostID := "a2000000-0000-4000-8000-000000000001"
	seededOpen(t, r, matchID, hostID, t0.Add(8*time.Hour), t0.Add(9*time.Hour), 2)
	if _, err := r.Join(context.Background(), "a3000000-0000-4000-8000-000000000001", matchID, "capacity-join", t0); err != nil {
		t.Fatal(err)
	}
	if _, err := r.UpdatePublished(context.Background(), hostID, matchID, "capacity-low", "Mô tả mới", 0, t0.Add(time.Minute)); !errors.Is(err, ErrCapacityBelowOccupied) {
		t.Fatalf("capacity error=%v", err)
	}
	updated, err := r.UpdatePublished(context.Background(), hostID, matchID, "capacity-up", "Mô tả mới", 3, t0.Add(time.Minute))
	if err != nil || updated.Capacity != 3 || updated.Description != "Mô tả mới" {
		t.Fatalf("updated=%+v error=%v", updated, err)
	}
}
