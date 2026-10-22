// Package postgres implements customer.Store on Postgres.
package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"orders-api/internal/customer"
	"orders-api/internal/db"
)

var _ customer.Store = (*Store)(nil)

// Store reads customers from the customers table.
type Store struct {
	db db.DBTX
}

// NewStore returns a Store that runs its queries on q, usually the pool.
func NewStore(q db.DBTX) *Store {
	return &Store{db: q}
}

func (s *Store) Get(ctx context.Context, id uuid.UUID) (customer.Customer, error) {
	var c customer.Customer
	err := s.db.QueryRow(ctx, `SELECT id, card_number FROM customers WHERE id = $1`, id).
		Scan(&c.ID, &c.CardNumber)
	if errors.Is(err, pgx.ErrNoRows) {
		return customer.Customer{}, customer.ErrNotFound
	}
	if err != nil {
		return customer.Customer{}, err
	}
	return c, nil
}
