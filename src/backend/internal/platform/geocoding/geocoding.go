package geocoding

import (
	"context"
	"errors"
)

var ErrUnavailable = errors.New("geocoding provider is unavailable")

type Point struct{ Latitude, Longitude float64 }

type Provider interface {
	Resolve(context.Context, string) (Point, error)
	DistanceKM(context.Context, Point, Point) (float64, error)
}

// Disabled preserves the current area-based MVP behavior until a provider is selected.
type Disabled struct{}

func (Disabled) Resolve(context.Context, string) (Point, error) { return Point{}, ErrUnavailable }
func (Disabled) DistanceKM(context.Context, Point, Point) (float64, error) {
	return 0, ErrUnavailable
}
