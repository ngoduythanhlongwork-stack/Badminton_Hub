// Package venues owns the venue catalogue and exposes only explicit contracts.
package venues

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"badmintonhub/internal/platform/id"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Status string

const (
	StatusDraft     Status = "DRAFT"
	StatusPublished Status = "PUBLISHED"
	StatusHidden    Status = "HIDDEN"
)

type ManagementScope string

const (
	ScopeAssignedVenues ManagementScope = "ASSIGNED_VENUES"
	ScopeAllVenues      ManagementScope = "ALL_VENUES"
)

// Actor carries the current decision from the external authorization policy.
type Actor struct {
	UserID string
	Scope  ManagementScope
}
type Authorization interface {
	VenueManagementScope(context.Context, string) (ManagementScope, error)
}
type Source struct {
	Kind, Reference string
	UpdatedAt       time.Time
}
type PriceGuidance struct {
	AmountMinor    int64
	Currency, Unit string
	Source         Source
}
type Venue struct {
	ID, Name, Address, Area, TimeZone       string
	Latitude, Longitude                     *float64
	OpeningHours, Amenities, Photos, Courts []string
	Price                                   *PriceGuidance
	Status                                  Status
	CreatedAt, UpdatedAt                    time.Time
}
type Draft struct {
	Name, Address, Area, TimeZone           string
	Latitude, Longitude                     *float64
	OpeningHours, Amenities, Photos, Courts []string
	Price                                   *PriceGuidance
}
type Filters struct {
	Area          string
	MaxPriceMinor *int64
	Limit         int
	Cursor        string
}
type Page struct {
	Venues     []Venue
	NextCursor string
}

var (
	ErrForbidden    = errors.New("venue management forbidden")
	ErrNotFound     = errors.New("venue not found")
	ErrInvalid      = errors.New("invalid venue")
	ErrNotPublished = errors.New("venue is not published")
)

type Repository interface {
	Create(context.Context, Venue, Actor) (Venue, error)
	Update(context.Context, Venue, Actor) (Venue, error)
	SetStatus(context.Context, string, Status, Actor, time.Time) (Venue, error)
	AssignManager(context.Context, string, string, Actor, time.Time) error
	PublicList(context.Context, Filters) (Page, error)
	PublicDetail(context.Context, string) (Venue, error)
}
type Service struct {
	repo          Repository
	authorization Authorization
	now           func() time.Time
}

func NewService(repo Repository, authorization Authorization, now func() time.Time) (Service, error) {
	if repo == nil || authorization == nil {
		return Service{}, fmt.Errorf("%w: repository and authorization are required", ErrInvalid)
	}
	if now == nil {
		now = time.Now
	}
	return Service{repo: repo, authorization: authorization, now: now}, nil
}
func (s Service) actor(ctx context.Context, userID string) (Actor, error) {
	scope, e := s.authorization.VenueManagementScope(ctx, userID)
	if e != nil {
		return Actor{}, e
	}
	if scope != ScopeAssignedVenues && scope != ScopeAllVenues {
		return Actor{}, ErrForbidden
	}
	return Actor{UserID: userID, Scope: scope}, nil
}
func (s Service) Create(ctx context.Context, userID string, d Draft) (Venue, error) {
	actor, e := s.actor(ctx, userID)
	if e != nil {
		return Venue{}, e
	}
	v := venueFromDraft(d)
	if err := validateDraft(v); err != nil {
		return Venue{}, err
	}
	v.ID, _ = id.New()
	v.Status = StatusDraft
	v.CreatedAt = s.now().UTC()
	v.UpdatedAt = v.CreatedAt
	return s.repo.Create(ctx, v, actor)
}
func (s Service) Update(ctx context.Context, userID, venueID string, d Draft) (Venue, error) {
	actor, e := s.actor(ctx, userID)
	if e != nil {
		return Venue{}, e
	}
	v := venueFromDraft(d)
	v.ID = venueID
	if err := validateDraft(v); err != nil {
		return Venue{}, err
	}
	v.UpdatedAt = s.now().UTC()
	return s.repo.Update(ctx, v, actor)
}
func (s Service) Publish(ctx context.Context, userID, venueID string) (Venue, error) {
	actor, e := s.actor(ctx, userID)
	if e != nil {
		return Venue{}, e
	}
	return s.repo.SetStatus(ctx, venueID, StatusPublished, actor, s.now().UTC())
}
func (s Service) Hide(ctx context.Context, userID, venueID string) (Venue, error) {
	actor, e := s.actor(ctx, userID)
	if e != nil {
		return Venue{}, e
	}
	return s.repo.SetStatus(ctx, venueID, StatusHidden, actor, s.now().UTC())
}
func (s Service) AssignManager(ctx context.Context, userID, venueID, managerID string) error {
	actor, e := s.actor(ctx, userID)
	if e != nil {
		return e
	}
	if actor.Scope != ScopeAllVenues {
		return ErrForbidden
	}
	return s.repo.AssignManager(ctx, venueID, managerID, actor, s.now().UTC())
}
func (s Service) List(ctx context.Context, f Filters) (Page, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 20
	}
	return s.repo.PublicList(ctx, f)
}
func (s Service) Detail(ctx context.Context, id string) (Venue, error) {
	return s.repo.PublicDetail(ctx, id)
}
func venueFromDraft(d Draft) Venue {
	return Venue{Name: strings.TrimSpace(d.Name), Address: strings.TrimSpace(d.Address), Area: strings.TrimSpace(d.Area), TimeZone: strings.TrimSpace(d.TimeZone), Latitude: d.Latitude, Longitude: d.Longitude, OpeningHours: nonNil(d.OpeningHours), Amenities: nonNil(d.Amenities), Photos: nonNil(d.Photos), Courts: nonNil(d.Courts), Price: d.Price}
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
func validateDraft(v Venue) error {
	if v.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalid)
	}
	if (v.Latitude == nil) != (v.Longitude == nil) {
		return fmt.Errorf("%w: coordinate pair required", ErrInvalid)
	}
	if v.Latitude != nil && (*v.Latitude < -90 || *v.Latitude > 90 || *v.Longitude < -180 || *v.Longitude > 180) {
		return fmt.Errorf("%w: coordinates out of range", ErrInvalid)
	}
	if v.Price != nil {
		if v.Price.AmountMinor < 0 || v.Price.Currency == "" || v.Price.Unit == "" || v.Price.Source.Kind == "" || v.Price.Source.UpdatedAt.IsZero() {
			return fmt.Errorf("%w: price requires amount, currency, unit, source and updated time", ErrInvalid)
		}
	}
	return nil
}
func validatePublish(v Venue) error {
	if err := validateDraft(v); err != nil {
		return err
	}
	if v.Address == "" || v.Area == "" || v.Latitude == nil || v.Longitude == nil || v.TimeZone == "" || len(v.OpeningHours) == 0 {
		return fmt.Errorf("%w: address, area, coordinates, timezone and opening hours are required", ErrInvalid)
	}
	if _, err := time.LoadLocation(v.TimeZone); err != nil {
		return fmt.Errorf("%w: invalid IANA timezone", ErrInvalid)
	}
	return nil
}

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}
func (r *PostgresRepository) allowed(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, venueID string, a Actor) error {
	if a.UserID == "" {
		return ErrForbidden
	}
	if a.Scope == ScopeAllVenues {
		return nil
	}
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM venues.manager_assignments WHERE venue_id=$1 AND manager_id=$2 AND revoked_at IS NULL)`, venueID, a.UserID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}
func (r *PostgresRepository) Create(ctx context.Context, v Venue, a Actor) (Venue, error) {
	if a.Scope != ScopeAllVenues {
		return Venue{}, ErrForbidden
	}
	_, err := r.pool.Exec(ctx, `INSERT INTO venues.venues(id,name,address,area,latitude,longitude,time_zone,opening_hours,amenities,photos,courts,price_amount_minor,price_currency,price_unit,price_source_kind,price_source_reference,price_updated_at,status,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`, v.ID, v.Name, v.Address, v.Area, v.Latitude, v.Longitude, v.TimeZone, v.OpeningHours, v.Amenities, v.Photos, v.Courts, priceAmount(v.Price), priceText(v.Price, 0), priceText(v.Price, 1), sourceText(v.Price, 0), sourceText(v.Price, 1), priceTime(v.Price), v.Status, v.CreatedAt, v.UpdatedAt)
	return v, err
}
func (r *PostgresRepository) Update(ctx context.Context, v Venue, a Actor) (Venue, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Venue{}, err
	}
	defer tx.Rollback(context.Background())
	if err = r.allowed(ctx, tx, v.ID, a); err != nil {
		return Venue{}, err
	}
	current, err := r.get(ctx, tx, v.ID, false)
	if err != nil {
		return Venue{}, err
	}
	if current.Status == StatusPublished {
		if err = validatePublish(v); err != nil {
			return Venue{}, err
		}
	}
	tag, err := tx.Exec(ctx, `UPDATE venues.venues SET name=$2,address=$3,area=$4,latitude=$5,longitude=$6,time_zone=$7,opening_hours=$8,amenities=$9,photos=$10,courts=$11,price_amount_minor=$12,price_currency=$13,price_unit=$14,price_source_kind=$15,price_source_reference=$16,price_updated_at=$17,updated_at=$18 WHERE id=$1`, v.ID, v.Name, v.Address, v.Area, v.Latitude, v.Longitude, v.TimeZone, v.OpeningHours, v.Amenities, v.Photos, v.Courts, priceAmount(v.Price), priceText(v.Price, 0), priceText(v.Price, 1), sourceText(v.Price, 0), sourceText(v.Price, 1), priceTime(v.Price), v.UpdatedAt)
	if err != nil {
		return Venue{}, err
	}
	if tag.RowsAffected() == 0 {
		return Venue{}, ErrNotFound
	}
	if err = tx.Commit(ctx); err != nil {
		return Venue{}, err
	}
	v.Status = current.Status
	v.CreatedAt = current.CreatedAt
	return v, nil
}
func (r *PostgresRepository) SetStatus(ctx context.Context, id string, status Status, a Actor, at time.Time) (Venue, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Venue{}, err
	}
	defer tx.Rollback(context.Background())
	if err = r.allowed(ctx, tx, id, a); err != nil {
		return Venue{}, err
	}
	v, err := r.get(ctx, tx, id, false)
	if err != nil {
		return Venue{}, err
	}
	if status == StatusPublished {
		if err = validatePublish(v); err != nil {
			return Venue{}, err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE venues.venues SET status=$2,updated_at=$3 WHERE id=$1`, id, status, at)
	if err != nil {
		return Venue{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Venue{}, err
	}
	v.Status = status
	v.UpdatedAt = at
	return v, nil
}
func (r *PostgresRepository) AssignManager(ctx context.Context, venueID, managerID string, a Actor, at time.Time) error {
	if a.Scope != ScopeAllVenues {
		return ErrForbidden
	}
	_, err := r.pool.Exec(ctx, `INSERT INTO venues.manager_assignments(venue_id,manager_id,assigned_by,assigned_at) VALUES($1,$2,$3,$4) ON CONFLICT(venue_id,manager_id) DO UPDATE SET assigned_by=EXCLUDED.assigned_by,assigned_at=EXCLUDED.assigned_at,revoked_at=NULL`, venueID, managerID, a.UserID, at)
	return err
}
func (r *PostgresRepository) PublicDetail(ctx context.Context, id string) (Venue, error) {
	return r.get(ctx, r.pool, id, true)
}
func (r *PostgresRepository) PublicList(ctx context.Context, f Filters) (Page, error) {
	args := []any{f.Limit + 1}
	q := `SELECT id FROM venues.venues WHERE status='PUBLISHED'`
	n := 2
	if f.Area != "" {
		q += fmt.Sprintf(" AND area=$%d", n)
		args = append(args, f.Area)
		n++
	}
	if f.MaxPriceMinor != nil {
		q += fmt.Sprintf(" AND price_amount_minor <= $%d", n)
		args = append(args, *f.MaxPriceMinor)
		n++
	}
	if f.Cursor != "" {
		q += fmt.Sprintf(" AND id > $%d", n)
		args = append(args, f.Cursor)
	}
	q += ` ORDER BY id LIMIT $1`
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return Page{}, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return Page{}, err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return Page{}, err
	}
	rows.Close()
	var out Page
	for _, id := range ids {
		v, e := r.get(ctx, r.pool, id, true)
		if e != nil {
			return Page{}, e
		}
		out.Venues = append(out.Venues, v)
	}
	if len(out.Venues) > f.Limit {
		out.NextCursor = out.Venues[f.Limit-1].ID
		out.Venues = out.Venues[:f.Limit]
	}
	return out, nil
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (r *PostgresRepository) get(ctx context.Context, q rowQuerier, id string, public bool) (Venue, error) {
	sql := `SELECT id,name,address,area,latitude,longitude,time_zone,opening_hours,amenities,photos,courts,price_amount_minor,price_currency,price_unit,price_source_kind,price_source_reference,price_updated_at,status,created_at,updated_at FROM venues.venues WHERE id=$1`
	if public {
		sql += ` AND status='PUBLISHED'`
	}
	var v Venue
	var amount *int64
	var currency, unit, kind, ref *string
	var updated *time.Time
	err := q.QueryRow(ctx, sql, id).Scan(&v.ID, &v.Name, &v.Address, &v.Area, &v.Latitude, &v.Longitude, &v.TimeZone, &v.OpeningHours, &v.Amenities, &v.Photos, &v.Courts, &amount, &currency, &unit, &kind, &ref, &updated, &v.Status, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		if public {
			return Venue{}, ErrNotPublished
		}
		return Venue{}, ErrNotFound
	}
	if err != nil {
		return Venue{}, err
	}
	if amount != nil {
		v.Price = &PriceGuidance{AmountMinor: *amount, Currency: *currency, Unit: *unit, Source: Source{Kind: *kind, UpdatedAt: *updated}}
		if ref != nil {
			v.Price.Source.Reference = *ref
		}
	}
	return v, nil
}
func priceAmount(p *PriceGuidance) *int64 {
	if p == nil {
		return nil
	}
	return &p.AmountMinor
}
func priceText(p *PriceGuidance, n int) *string {
	if p == nil {
		return nil
	}
	if n == 0 {
		return &p.Currency
	}
	return &p.Unit
}
func sourceText(p *PriceGuidance, n int) *string {
	if p == nil {
		return nil
	}
	if n == 0 {
		return &p.Source.Kind
	}
	if p.Source.Reference == "" {
		return nil
	}
	return &p.Source.Reference
}
func priceTime(p *PriceGuidance) *time.Time {
	if p == nil {
		return nil
	}
	return &p.Source.UpdatedAt
}
