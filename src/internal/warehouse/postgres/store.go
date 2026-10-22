// Package postgres implements warehouse.Store on Postgres.
package postgres

import (
	"bytes"
	"context"
	"errors"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"orders-api/internal/db"
	"orders-api/internal/geocoding"
	"orders-api/internal/warehouse"
)

var _ warehouse.Store = (*Store)(nil)

// Store keeps warehouses and their stock in the warehouses and stock tables.
type Store struct {
	db db.DBTX
}

// NewStore returns a Store that runs its queries on q, usually the pool.
func NewStore(q db.DBTX) *Store {
	return &Store{db: q}
}

func (s *Store) WithTx(tx db.DBTX) warehouse.Store {
	return &Store{db: tx}
}

// Closest ranks the warehouses that have every item by haversine distance
// to p, in kilometers.
func (s *Store) Closest(ctx context.Context, p geocoding.Point, items []warehouse.Item) (uuid.UUID, error) {
	productIDs := make([]string, len(items))
	quantities := make([]int32, len(items))
	for i, it := range items {
		productIDs[i] = it.ProductID.String()
		quantities[i] = int32(it.Quantity)
	}

	var id uuid.UUID
	err := s.db.QueryRow(ctx, `
		SELECT w.id
		FROM warehouses w
		JOIN stock s ON s.warehouse_id = w.id
		JOIN unnest($1::uuid[], $2::integer[]) AS i (product_id, quantity)
			ON i.product_id = s.product_id AND s.available >= i.quantity
		GROUP BY w.id
		HAVING count(*) = $3
		ORDER BY 2 * 6371 * asin(sqrt(
			power(sin(radians(w.lat - $4) / 2), 2) +
			cos(radians($4)) * cos(radians(w.lat)) * power(sin(radians(w.lng - $5) / 2), 2)
		))
		LIMIT 1`,
		productIDs, quantities, len(items), p.Lat, p.Lng,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, warehouse.ErrNoWarehouse
	}
	if err != nil {
		return uuid.UUID{}, err
	}
	return id, nil
}

// Reserve takes each item only if enough is still available, in product
// order so two orders sharing products lock their rows in the same order and
// can't deadlock.
func (s *Store) Reserve(ctx context.Context, warehouseID uuid.UUID, items []warehouse.Item) (bool, error) {
	for _, it := range byProduct(items) {
		tag, err := s.db.Exec(ctx, `
			UPDATE stock SET available = available - $3
			WHERE warehouse_id = $1 AND product_id = $2 AND available >= $3`,
			warehouseID, it.ProductID, it.Quantity)
		if err != nil {
			return false, err
		}
		if tag.RowsAffected() != 1 {
			return false, nil
		}
	}
	return true, nil
}

func (s *Store) Release(ctx context.Context, warehouseID uuid.UUID, items []warehouse.Item) error {
	for _, it := range byProduct(items) {
		_, err := s.db.Exec(ctx, `
			UPDATE stock SET available = available + $3
			WHERE warehouse_id = $1 AND product_id = $2`,
			warehouseID, it.ProductID, it.Quantity)
		if err != nil {
			return err
		}
	}
	return nil
}

// byProduct returns a copy of items sorted by product ID.
func byProduct(items []warehouse.Item) []warehouse.Item {
	sorted := slices.Clone(items)
	slices.SortFunc(sorted, func(a, b warehouse.Item) int {
		return bytes.Compare(a.ProductID[:], b.ProductID[:])
	})
	return sorted
}
