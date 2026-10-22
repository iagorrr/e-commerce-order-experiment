// Package warehouse holds the warehouses orders ship from and their stock.
package warehouse

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"orders-api/internal/db"
	"orders-api/internal/geocoding"
)

var ErrNoWarehouse = errors.New("no warehouse has every item")

// Item is a quantity of one product taken from or given back to stock. It is
// the warehouse's own type so this package doesn't import order, which
// imports it.
type Item struct {
	ProductID uuid.UUID
	Quantity  int
}

// Store reads warehouses and moves their stock.
type Store interface {
	// WithTx returns a Store that runs its queries inside tx.
	WithTx(tx db.DBTX) Store
	// Closest returns the warehouse closest to p that has every item
	// available, or ErrNoWarehouse when none has.
	Closest(ctx context.Context, p geocoding.Point, items []Item) (uuid.UUID, error)
	// Reserve takes items from the warehouse's stock and reports whether it
	// could. When it can't, another order took the stock first and the
	// caller must roll back, because some items may already be taken.
	Reserve(ctx context.Context, warehouseID uuid.UUID, items []Item) (bool, error)
	// Release gives items back to the warehouse's stock.
	Release(ctx context.Context, warehouseID uuid.UUID, items []Item) error
}
