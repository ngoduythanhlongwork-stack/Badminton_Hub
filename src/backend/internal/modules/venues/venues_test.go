package venues

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeRepo struct {
	updated   Venue
	updateErr error
}
type fakeAuthorization struct{ scope ManagementScope }

func (f fakeAuthorization) VenueManagementScope(context.Context, string) (ManagementScope, error) {
	return f.scope, nil
}

func (f *fakeRepo) Create(context.Context, Venue, Actor) (Venue, error) { return Venue{}, nil }
func (f *fakeRepo) Update(_ context.Context, v Venue, _ Actor) (Venue, error) {
	f.updated = v
	return v, f.updateErr
}
func (f *fakeRepo) SetStatus(context.Context, string, Status, Actor, time.Time) (Venue, error) {
	return Venue{}, nil
}
func (f *fakeRepo) AssignManager(context.Context, string, string, Actor, time.Time) error { return nil }
func (f *fakeRepo) PublicList(context.Context, Filters) (Page, error)                     { return Page{}, nil }
func (f *fakeRepo) PublicDetail(context.Context, string) (Venue, error)                   { return Venue{}, nil }
func TestValidatePublishRequiresReferenceData(t *testing.T) {
	v := Venue{Name: "Sân A", Address: "A", Area: "Q1", TimeZone: "Asia/Ho_Chi_Minh", OpeningHours: []string{"08:00-22:00"}}
	if !errors.Is(validatePublish(v), ErrInvalid) {
		t.Fatal("coordinates must be required")
	}
	lat, lon := 10.0, 106.0
	v.Latitude = &lat
	v.Longitude = &lon
	if err := validatePublish(v); err != nil {
		t.Fatal(err)
	}
}
func TestCoordinatePairAndRange(t *testing.T) {
	lat := 91.0
	if !errors.Is(validateDraft(Venue{Name: "A", Latitude: &lat}), ErrInvalid) {
		t.Fatal("expected pair validation")
	}
	lon := 106.0
	if !errors.Is(validateDraft(Venue{Name: "A", Latitude: &lat, Longitude: &lon}), ErrInvalid) {
		t.Fatal("expected range validation")
	}
}
func TestNewServiceFailsClosed(t *testing.T) {
	if _, err := NewService(nil, nil, nil); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}

func TestVenueDraftNormalizesOptionalCollections(t *testing.T) {
	venue := venueFromDraft(Draft{Name: "Sân"})
	if venue.OpeningHours == nil || venue.Amenities == nil || venue.Photos == nil || venue.Courts == nil {
		t.Fatalf("optional collections must persist as empty arrays: %+v", venue)
	}
}
