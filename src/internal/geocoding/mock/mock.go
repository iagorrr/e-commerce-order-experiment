// Package mock implements geocoding.Geocoder without calling a provider.
package mock

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"strings"

	"orders-api/internal/geocoding"
)

// NotFound is the text that makes Geocoder fail to find an address.
const NotFound = "not found"

var _ geocoding.Geocoder = Geocoder{}

// Geocoder places every address at a point derived from its hash, so the
// same address always lands on the same point. Addresses containing NotFound
// are not found.
type Geocoder struct{}

func (Geocoder) Geocode(_ context.Context, address string) (geocoding.Point, error) {
	if strings.Contains(address, NotFound) {
		return geocoding.Point{}, geocoding.ErrNotFound
	}
	sum := sha256.Sum256([]byte(address))
	return geocoding.Point{
		Lat: fraction(sum[0:8])*180 - 90,
		Lng: fraction(sum[8:16])*360 - 180,
	}, nil
}

// fraction maps 8 bytes to a number in [0, 1].
func fraction(b []byte) float64 {
	return float64(binary.BigEndian.Uint64(b)) / float64(^uint64(0))
}
