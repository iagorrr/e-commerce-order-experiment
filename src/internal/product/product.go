// Package product holds the products that can be ordered.
package product

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("product not found")

// Product is something that can be ordered, priced in cents.
type Product struct {
	ID         uuid.UUID
	PriceCents int64
}

// Store reads products.
type Store interface {
	Get(ctx context.Context, id uuid.UUID) (Product, error)
	// GetAll returns the products with the given ids, keyed by id. Ids with no
	// product are left out.
	GetAll(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]Product, error)
}
