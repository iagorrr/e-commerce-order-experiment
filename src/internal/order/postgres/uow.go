package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"orders-api/internal/order"
	orderriver "orders-api/internal/order/river"
	"orders-api/internal/warehouse"
)

var _ order.UnitOfWork = (*UnitOfWork)(nil)

// UnitOfWork runs order work in one Postgres transaction. Jobs are enqueued
// in the same transaction, since River keeps them in Postgres.
type UnitOfWork struct {
	pool       *pgxpool.Pool
	river      *river.Client[pgx.Tx]
	warehouses warehouse.Store
}

// NewUnitOfWork returns a UnitOfWork that opens its transactions on pool,
// enqueues jobs with river and moves stock with warehouses.
func NewUnitOfWork(pool *pgxpool.Pool, river *river.Client[pgx.Tx], warehouses warehouse.Store) *UnitOfWork {
	return &UnitOfWork{
		pool:       pool,
		river:      river,
		warehouses: warehouses,
	}
}

func (u *UnitOfWork) Do(ctx context.Context, fn func(tx order.Tx) error) error {
	pgTx, err := u.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = pgTx.Rollback(ctx) }()

	if err := fn(&tx{pgTx: pgTx, river: u.river, warehouses: u.warehouses.WithTx(pgTx)}); err != nil {
		return err
	}

	return pgTx.Commit(ctx)
}

type tx struct {
	pgTx       pgx.Tx
	river      *river.Client[pgx.Tx]
	warehouses warehouse.Store
}

func (t *tx) Orders() order.Store {
	return NewStore(t.pgTx)
}

func (t *tx) Warehouses() warehouse.Store {
	return t.warehouses
}

func (t *tx) Enqueue(ctx context.Context, s order.Status, orderID uuid.UUID) error {
	args, err := orderriver.NewJob(s, orderID)
	if err != nil {
		return err
	}

	_, err = t.river.InsertTx(ctx, t.pgTx, args, nil)
	return err
}
