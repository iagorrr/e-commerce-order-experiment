// Package postgres implements product.Store on Postgres.
package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"orders-api/internal/db"
	"orders-api/internal/product"
)

var _ product.Store = (*Store)(nil)

// Store reads products from the products table.
type Store struct {
	db db.DBTX
}

// NewStore returns a Store that runs its queries on q, usually the pool.
func NewStore(q db.DBTX) *Store {
	return &Store{db: q}
}

func (s *Store) Get(ctx context.Context, id uuid.UUID) (product.Product, error) {
	var p product.Product
	err := s.db.QueryRow(ctx, `SELECT id, price_cents FROM products WHERE id = $1`, id).
		Scan(&p.ID, &p.PriceCents)
	if errors.Is(err, pgx.ErrNoRows) {
		return product.Product{}, product.ErrNotFound
	}
	if err != nil {
		return product.Product{}, err
	}
	return p, nil
}

func (s *Store) GetAll(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]product.Product, error) {
	rows, err := s.db.Query(ctx, `SELECT id, price_cents FROM products WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	ps, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (product.Product, error) {
		var p product.Product
		err := row.Scan(&p.ID, &p.PriceCents)
		return p, err
	})
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]product.Product, len(ps))
	for _, p := range ps {
		byID[p.ID] = p
	}
	return byID, nil
}
