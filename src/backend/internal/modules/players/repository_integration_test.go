//go:build integration

package players

import (
	"context"
	"errors"
	"testing"
	"time"

	"badmintonhub/internal/platform/migrations"
	"badmintonhub/internal/platform/testdb"
)

func TestPostgresRepositoryPersistsDraftAndProtectsCompletedDOB(t *testing.T) {
	pool := testdb.Open(t)
	runner, err := migrations.NewRunner(pool, Migrations())
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	repository := NewPostgresRepository(pool)
	service, err := NewService(repository, fixedClock{time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	name, area := "An", "Da Nang"
	dob := CalendarDate{Year: 2000, Month: time.February, Day: 29}
	experience, level := Experience3PlusYears, SkillAdvanced
	draft := Draft{DisplayName: &name, DateOfBirth: &dob, Experience: &experience, SkillLevel: &level,
		PreferredFormats: []GameFormat{FormatDoubles, FormatMixed}, PlayStyles: []PlayStyle{StyleTraining},
		UsualPeriods: []UsualPeriod{PeriodEvening}, RegularArea: &area}
	if _, err := service.SaveDraft(context.Background(), "11111111-1111-4111-8111-111111111111", draft); err != nil {
		t.Fatal(err)
	}
	completed, err := service.CompleteOnboarding(context.Background(), "11111111-1111-4111-8111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if completed.Version != 2 {
		t.Fatalf("version=%d", completed.Version)
	}
	_, err = pool.Exec(context.Background(), `UPDATE players.profiles SET date_of_birth='1999-01-01' WHERE account_id='11111111-1111-4111-8111-111111111111'`)
	if err == nil {
		t.Fatal("database must reject completed DOB change")
	}
}

func TestPostgresRepositoryRejectsStaleWrite(t *testing.T) {
	pool := testdb.Open(t)
	runner, _ := migrations.NewRunner(pool, Migrations())
	if err := runner.Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	repository := NewPostgresRepository(pool)
	p := newProfile("22222222-2222-4222-8222-222222222222")
	p.Status = OnboardingInProgress
	p.UpdatedAt = time.Now().UTC()
	saved, err := repository.Save(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	copy := saved
	saved.UpdatedAt = saved.UpdatedAt.Add(time.Second)
	if _, err := repository.Save(context.Background(), saved); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Save(context.Background(), copy); !errors.Is(err, ErrConflict) {
		t.Fatalf("error=%v", err)
	}
}
