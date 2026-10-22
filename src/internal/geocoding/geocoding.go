// Package geocoding converts free-text addresses into coordinates.
package geocoding

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("address not found")

// Point is a location in decimal degrees.
type Point struct {
	Lat float64
	Lng float64
}

// Geocoder converts an address into coordinates.
type Geocoder interface {
	Geocode(ctx context.Context, address string) (Point, error)
}
