//go:build integration

package main

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"badmintonhub/internal/modules/communication"
	"badmintonhub/internal/modules/identity"
	"badmintonhub/internal/modules/matches"
	"badmintonhub/internal/modules/moderation"
	"badmintonhub/internal/modules/notifications"
	"badmintonhub/internal/modules/payments"
	"badmintonhub/internal/modules/players"
	"badmintonhub/internal/modules/recommendations"
	"badmintonhub/internal/modules/venues"
	"badmintonhub/internal/platform/migrations"
	"badmintonhub/internal/platform/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

type d24Profiles struct{}

func (d24Profiles) RecommendationProfile(_ context.Context, accountID string) (recommendations.Profile, error) {
	return recommendations.Profile{AccountID: accountID, SkillLevel: 3, Formats: []string{"DOUBLES"}, Styles: []string{"SOCIAL"}, Periods: []string{"EVENING"}, Area: "Quan 1", Reliability: "GOOD"}, nil
}
func (d24Profiles) HostReliabilities(_ context.Context, accountIDs []string) (map[string]string, error) {
	result := make(map[string]string, len(accountIDs))
	for _, accountID := range accountIDs {
		result[accountID] = "GOOD"
	}
	return result, nil
}

type d24Candidates struct {
	repo      *matches.PostgresRepository
	mu        sync.Mutex
	durations []time.Duration
}

func (c *d24Candidates) RecommendationCandidates(ctx context.Context, accountID string, limit int) ([]recommendations.Candidate, error) {
	started := time.Now()
	defer func() {
		c.mu.Lock()
		c.durations = append(c.durations, time.Since(started))
		c.mu.Unlock()
	}()
	values, err := c.repo.RecommendationCandidates(ctx, accountID, time.Now().UTC(), limit)
	if err != nil {
		return nil, err
	}
	result := make([]recommendations.Candidate, len(values))
	for i, value := range values {
		result[i] = recommendations.Candidate{ID: value.ID, HostID: value.HostID, Area: value.Area, TimeZone: value.TimeZone, Format: value.Format, Style: value.Style, JoinMode: string(value.JoinMode), StartAt: value.StartAt, EndAt: value.EndAt, MinLevel: value.MinLevel, MaxLevel: value.MaxLevel}
	}
	return result, nil
}

func TestD24RecommendationPilotLoad(t *testing.T) {
	bootstrapPool := testdb.Open(t)
	poolConfig := bootstrapPool.Config()
	poolConfig.MaxConns = 10 // same as the production connection factory
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	ctx := context.Background()
	runner, err := migrations.NewRunner(pool, identity.Migrations(), players.Migrations(), venues.Migrations(), matches.Migrations(), payments.Migrations(), notifications.Migrations(), communication.Migrations(), moderation.Migrations(), recommendations.Migrations())
	if err != nil {
		t.Fatal(err)
	}
	if err = runner.Up(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err = pool.Exec(ctx, `INSERT INTO identity.accounts(id,email,canonical_email,state,email_verified_at,created_at,updated_at)
SELECT md5('account-'||n)::uuid::text,'pilot-'||n||'@example.test','pilot-'||n||'@example.test','ACTIVE',$1,$1,$1 FROM generate_series(1,500) n`, now); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO matches.matches(id,host_id,title,description,venue_id,venue_name,venue_address,venue_area,venue_time_zone,court,start_at,end_at,format,style,rules,min_level,max_level,capacity,fee_minor,currency,join_mode,host_plays,court_attested,court_attested_at,status,created_at,updated_at,completed_at)
SELECT md5('historic-'||n)::uuid,md5('account-'||(1+(n%500)))::uuid,'Historic','D24',md5('venue-'||(n%20))::uuid,'San','Q1','Quan 1','Asia/Ho_Chi_Minh','Court 1',$1::timestamptz-interval '1 day'*(1+(n%365)),$1::timestamptz-interval '1 day'*(1+(n%365))+interval '2 hours','DOUBLES','SOCIAL','Fair play',1,6,4,0,'VND','INSTANT',true,true,$1,'COMPLETED',$1,$1,$1 FROM generate_series(1,10000) n`, now); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO matches.matches(id,host_id,title,description,venue_id,venue_name,venue_address,venue_area,venue_time_zone,court,start_at,end_at,format,style,rules,min_level,max_level,capacity,fee_minor,currency,join_mode,host_plays,court_attested,court_attested_at,status,created_at,updated_at)
SELECT md5('active-'||n)::uuid,md5('account-'||(1+(n%500)))::uuid,'Active','D24',md5('venue-'||(n%20))::uuid,'San','Q1','Quan 1','Asia/Ho_Chi_Minh','Court 1',$1::timestamptz+interval '1 day'+interval '1 minute'*n,$1::timestamptz+interval '1 day'+interval '1 minute'*n+interval '2 hours','DOUBLES','SOCIAL','Fair play',1,6,4,0,'VND','INSTANT',true,true,$1,'OPEN',$1,$1 FROM generate_series(1,1000) n`, now); err != nil {
		t.Fatal(err)
	}
	candidateProvider := &d24Candidates{repo: matches.NewPostgresRepository(pool)}
	service, err := recommendations.NewService(pool, d24Profiles{}, candidateProvider, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	const concurrency = 50
	var warmup sync.WaitGroup
	warmupStart := make(chan struct{})
	warmupErrors := make([]error, concurrency)
	for i := 0; i < concurrency; i++ {
		warmup.Add(1)
		go func(index int) {
			defer warmup.Done()
			<-warmupStart
			_, warmupErrors[index] = service.Recommend(ctx, md5UUID(index+1))
		}(i)
	}
	close(warmupStart)
	warmup.Wait()
	for _, warmupErr := range warmupErrors {
		if warmupErr != nil {
			t.Fatal(warmupErr)
		}
	}
	durations := make([]time.Duration, concurrency)
	errs := make([]error, concurrency)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			began := time.Now()
			_, errs[index] = service.Recommend(ctx, md5UUID(index+1))
			durations[index] = time.Since(began)
		}(i)
	}
	close(start)
	wg.Wait()
	for _, runErr := range errs {
		if runErr != nil {
			t.Fatal(runErr)
		}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p95 := durations[(95*len(durations)+99)/100-1]
	candidateProvider.mu.Lock()
	candidateDurations := append([]time.Duration(nil), candidateProvider.durations[len(candidateProvider.durations)-concurrency:]...)
	candidateProvider.mu.Unlock()
	sort.Slice(candidateDurations, func(i, j int) bool { return candidateDurations[i] < candidateDurations[j] })
	candidateP95 := candidateDurations[(95*len(candidateDurations)+99)/100-1]
	t.Logf("D24 environment=local-docker accounts=500 historical_matches=10000 active_candidates=1000 concurrency=50 warmup_concurrent_requests=50 candidate_p95=%s end_to_end_p95=%s", candidateP95, p95)
	if p95 > time.Second {
		t.Fatalf("recommendation p95=%s exceeds 1s", p95)
	}
}

func md5UUID(index int) string {
	const values = "0123456789abcdef"
	b := []byte("00000000-0000-4000-8000-000000000000")
	b[len(b)-1] = values[index%16]
	b[len(b)-2] = values[(index/16)%16]
	return string(b)
}
