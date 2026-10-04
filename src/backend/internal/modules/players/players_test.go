package players

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type memoryRepository struct{ profile *OwnerProfile }

func (r *memoryRepository) Get(_ context.Context, accountID string) (OwnerProfile, error) {
	if r.profile == nil || r.profile.AccountID != accountID {
		return OwnerProfile{}, ErrNotFound
	}
	return *r.profile, nil
}
func (r *memoryRepository) Save(_ context.Context, profile OwnerProfile) (OwnerProfile, error) {
	profile.Version++
	r.profile = &profile
	return profile, nil
}

func TestOnboardingDraftResumeCompleteAndPrivacy(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	repository := &memoryRepository{}
	service, err := NewService(repository, fixedClock{now}, nil)
	if err != nil {
		t.Fatal(err)
	}
	name, area := " Nguyen An ", "Quan 1"
	dob := CalendarDate{Year: 2000, Month: time.January, Day: 2}
	draft := Draft{DisplayName: &name, DateOfBirth: &dob, RegularArea: &area}
	profile, err := service.SaveDraft(context.Background(), "account-1", draft)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Status != OnboardingInProgress || *profile.DisplayName != "Nguyen An" {
		t.Fatalf("draft=%+v", profile)
	}
	resumed, err := service.GetOwnerProfile(context.Background(), "account-1")
	if err != nil || resumed.DateOfBirth == nil {
		t.Fatalf("resume=%+v err=%v", resumed, err)
	}
	if _, err := service.CompleteOnboarding(context.Background(), "account-1"); !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("missing fields error=%v", err)
	}
	experience, level := Experience1To3Years, SkillIntermediate
	draft.Experience, draft.SkillLevel = &experience, &level
	draft.PreferredFormats = []GameFormat{FormatDoubles}
	draft.PlayStyles = []PlayStyle{StyleSocial}
	draft.UsualPeriods = []UsualPeriod{PeriodEvening}
	if _, err := service.SaveDraft(context.Background(), "account-1", draft); err != nil {
		t.Fatal(err)
	}
	completed, err := service.CompleteOnboarding(context.Background(), "account-1")
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != OnboardingComplete || completed.SkillConfidence != SkillConfidenceNew || completed.Reliability != ReliabilityNew {
		t.Fatalf("completed=%+v", completed)
	}
	public, err := service.GetPublicProfile(context.Background(), "account-1")
	if err != nil {
		t.Fatal(err)
	}
	if public.DisplayName != "Nguyen An" || public.MatchCount != 0 || public.Reliability != ReliabilityNew {
		t.Fatalf("public=%+v", public)
	}
}

func TestAdultEligibilityUsesVietnamCalendarAndLeapDayRule(t *testing.T) {
	location, err := time.LoadLocation(eligibilityTimeZone)
	if err != nil {
		t.Fatal(err)
	}
	dob := CalendarDate{Year: 2008, Month: time.February, Day: 29}
	if IsAdult(dob, time.Date(2026, time.February, 28, 16, 59, 0, 0, time.UTC), location) {
		t.Fatal("Feb 29 birth must not be eligible on Feb 28 in Vietnam")
	}
	if !IsAdult(dob, time.Date(2026, time.February, 28, 17, 0, 0, 0, time.UTC), location) {
		t.Fatal("Feb 29 birth must be eligible March 1 in non-leap year")
	}
	regular := CalendarDate{Year: 2008, Month: time.October, Day: 5}
	if IsAdult(regular, time.Date(2026, time.October, 4, 16, 0, 0, 0, time.UTC), location) {
		t.Fatal("must use Vietnam date, not the next UTC day")
	}
}

func TestUnderageCannotCompleteAndCompletedDOBIsImmutable(t *testing.T) {
	now := time.Date(2026, time.October, 4, 1, 0, 0, 0, time.UTC)
	repository := &memoryRepository{}
	service, _ := NewService(repository, fixedClock{now}, nil)
	name, area := "An", "Ha Noi"
	dob := CalendarDate{Year: 2010, Month: time.January, Day: 1}
	experience, level := Experience3To12Months, SkillBeginner
	draft := Draft{DisplayName: &name, DateOfBirth: &dob, Experience: &experience, SkillLevel: &level, PreferredFormats: []GameFormat{FormatSingles}, PlayStyles: []PlayStyle{StyleCasual}, UsualPeriods: []UsualPeriod{PeriodMorning}, RegularArea: &area}
	if _, err := service.SaveDraft(context.Background(), "account-2", draft); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteOnboarding(context.Background(), "account-2"); !errors.Is(err, ErrUnderage) {
		t.Fatalf("error=%v", err)
	}
	dob = CalendarDate{Year: 2000, Month: time.January, Day: 1}
	draft.DateOfBirth = &dob
	if _, err := service.SaveDraft(context.Background(), "account-2", draft); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteOnboarding(context.Background(), "account-2"); err != nil {
		t.Fatal(err)
	}
	changed := CalendarDate{Year: 1999, Month: time.January, Day: 1}
	draft.DateOfBirth = &changed
	if _, err := service.SaveDraft(context.Background(), "account-2", draft); !errors.Is(err, ErrDateOfBirthFixed) {
		t.Fatalf("error=%v", err)
	}
}

func TestIncompleteProfileHasNoPublicProjection(t *testing.T) {
	repository := &memoryRepository{}
	service, _ := NewService(repository, fixedClock{time.Now()}, nil)
	name := "An"
	if _, err := service.SaveDraft(context.Background(), "account-3", Draft{DisplayName: &name}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetPublicProfile(context.Background(), "account-3"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error=%v", err)
	}
}
