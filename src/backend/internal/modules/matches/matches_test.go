package matches

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeEligibility struct {
	level   int
	allowed bool
}

func (f fakeEligibility) CanPublishMatch(context.Context, string) (bool, error) {
	return f.allowed, nil
}
func (f fakeEligibility) PlayerEligibility(context.Context, string) (PlayerEligibility, error) {
	return PlayerEligibility{Eligible: f.allowed, Level: f.level}, nil
}

type fakeVenue struct{}

func (fakeVenue) PublishedVenue(context.Context, string, string) (VenueSnapshot, error) {
	return VenueSnapshot{ID: "v", Name: "A", TimeZone: "Asia/Ho_Chi_Minh"}, nil
}

type fakeRepo struct {
	match     Match
	joined    bool
	published bool
}

func (f *fakeRepo) Create(_ context.Context, m Match) (Match, error)      { return m, nil }
func (f *fakeRepo) UpdateDraft(_ context.Context, m Match) (Match, error) { return m, nil }
func (f *fakeRepo) OwnedDraft(context.Context, string, string) (Match, error) {
	return f.match, nil
}
func (f *fakeRepo) Publish(context.Context, string, string, time.Time) (Match, error) {
	f.published = true
	return Match{}, nil
}
func (f *fakeRepo) PublicList(context.Context, Filters) (Page, error)   { return Page{}, nil }
func (f *fakeRepo) PublicDetail(context.Context, string) (Match, error) { return f.match, nil }
func (f *fakeRepo) Participation(context.Context, string) (Participation, error) {
	return Participation{MatchID: "m", PlayerID: "p", Status: ParticipationRequested}, nil
}
func (f *fakeRepo) Join(context.Context, string, string, string, time.Time) (Participation, error) {
	f.joined = true
	return Participation{Status: ParticipationJoined}, nil
}
func (f *fakeRepo) Decide(context.Context, string, string, string, string, ParticipationStatus, string, time.Time) (Participation, error) {
	return Participation{}, nil
}
func TestInstantSkillMismatchDenied(t *testing.T) {
	r := &fakeRepo{match: Match{MinLevel: 3, MaxLevel: 4, JoinMode: JoinInstant}}
	s, e := NewService(r, fakeEligibility{allowed: true}, fakeEligibility{allowed: true, level: 2}, fakeVenue{}, nil)
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.Join(context.Background(), "p", "m", "key")
	if !errors.Is(e, ErrForbidden) || r.joined {
		t.Fatalf("got %v joined=%v", e, r.joined)
	}
}
func TestApprovalSkillMismatchCanRequest(t *testing.T) {
	r := &fakeRepo{match: Match{MinLevel: 3, MaxLevel: 4, JoinMode: JoinApproval}}
	s, _ := NewService(r, fakeEligibility{allowed: true}, fakeEligibility{allowed: true, level: 2}, fakeVenue{}, nil)
	if _, e := s.Join(context.Background(), "p", "m", "key"); e != nil {
		t.Fatal(e)
	}
	if !r.joined {
		t.Fatal("join request not delegated")
	}
}
func TestValidationFreeOnlyAndTimezone(t *testing.T) {
	now := time.Now().UTC()
	m := Match{Title: "A", Description: "D", Format: "DOUBLES", Style: "SOCIAL", Rules: "R", Venue: VenueSnapshot{ID: "v", TimeZone: "Asia/Ho_Chi_Minh"}, StartAt: now.Add(time.Hour), EndAt: now.Add(2 * time.Hour), MinLevel: 1, MaxLevel: 6, Capacity: 2, Currency: "VND", JoinMode: JoinInstant, CourtAttested: true}
	if e := validate(m, now); e != nil {
		t.Fatal(e)
	}
	m.FeeMinor = 1
	if !errors.Is(validate(m, now), ErrPaidUnsupported) {
		t.Fatal("paid match accepted")
	}
}
func TestNewServiceFailsClosed(t *testing.T) {
	if _, e := NewService(nil, nil, nil, nil, nil); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}

func TestPublishRechecksDraftTimeAndVenue(t *testing.T) {
	now := time.Now().UTC()
	repository := &fakeRepo{match: Match{HostID: "host", Title: "A", Description: "D", Format: "DOUBLES", Style: "SOCIAL", Rules: "R", Venue: VenueSnapshot{ID: "v", TimeZone: "Asia/Ho_Chi_Minh"}, StartAt: now.Add(-time.Minute), EndAt: now.Add(time.Hour), MinLevel: 1, MaxLevel: 6, Capacity: 2, Currency: "VND", JoinMode: JoinInstant, CourtAttested: true, Status: StatusDraft}}
	service, err := NewService(repository, fakeEligibility{allowed: true}, fakeEligibility{allowed: true, level: 2}, fakeVenue{}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Publish(context.Background(), "host", "match"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("error=%v", err)
	}
	if repository.published {
		t.Fatal("past draft was published")
	}
}
