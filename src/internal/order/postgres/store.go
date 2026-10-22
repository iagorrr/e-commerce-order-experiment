// Package postgres implements order.Store on Postgres.
package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"orders-api/internal/db"
	"orders-api/internal/geocoding"
	"orders-api/internal/order"
)

var _ order.Store = (*Store)(nil)

// todo: come up with some architecture that allow us to force the transaction
// Store keeps orders in the orders and order_items tables.
type Store struct {
	db db.DBTX
}

// NewStore returns a Store that runs its queries on q
func NewStore(q db.DBTX) *Store {
	return &Store{db: q}
}

func (s *Store) WithTx(tx db.DBTX) order.Store {
	return &Store{db: tx}
}

func (s *Store) Insert(ctx context.Context, o order.Order) (order.Order, error) {
	// Concurrent requests with the same key insert once: the others get no row
	// back and ErrKeyTaken, and the service replays the stored order.
	err := s.db.QueryRow(ctx, `
		INSERT INTO orders (customer_id, idempotency_key, shipping_address, status, total_cents)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (customer_id, idempotency_key) DO NOTHING
		RETURNING id`,
		o.CustomerID, o.IdempotencyKey, o.ShippingAddress, o.Status, o.TotalCents,
	).Scan(&o.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return order.Order{}, order.ErrKeyTaken
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == "orders_customer_id_fkey" {
		return order.Order{}, order.ErrCustomerNotFound
	}
	if err != nil {
		return order.Order{}, err
	}
	productIDs := make([]uuid.UUID, len(o.Items))
	quantities := make([]int32, len(o.Items))
	prices := make([]int64, len(o.Items))
	for i, l := range o.Items {
		productIDs[i], quantities[i], prices[i] = l.ProductID, int32(l.Quantity), l.UnitPriceCents
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO order_items (order_id, product_id, quantity, unit_price_cents)
		SELECT $1, product_id, quantity, unit_price_cents
		FROM unnest($2::uuid[], $3::integer[], $4::bigint[]) AS l (product_id, quantity, unit_price_cents)`,
		o.ID, productIDs, quantities, prices)
	if err != nil {
		return order.Order{}, err
	}
	return o, nil
}

func (s *Store) Get(ctx context.Context, id uuid.UUID) (order.Order, error) {
	return s.get(ctx, `id = $1`, id)
}

func (s *Store) GetByIdempotencyKey(ctx context.Context, customerID, key uuid.UUID) (order.Order, error) {
	return s.get(ctx, `customer_id = $1 AND idempotency_key = $2`, customerID, key)
}

// get returns the order matching where, with its lines.
func (s *Store) get(ctx context.Context, where string, args ...any) (order.Order, error) {
	var o order.Order
	var lat, lng *float64
	var warehouseID *uuid.UUID
	err := s.db.QueryRow(ctx, `
		SELECT id, customer_id, idempotency_key, shipping_address, status, total_cents, lat, lng, warehouse_id
		FROM orders WHERE `+where,
		args...,
	).Scan(&o.ID, &o.CustomerID, &o.IdempotencyKey, &o.ShippingAddress, &o.Status, &o.TotalCents, &lat, &lng, &warehouseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return order.Order{}, order.ErrNotFound
	}
	if err != nil {
		return order.Order{}, err
	}
	if lat != nil && lng != nil {
		o.Location = &geocoding.Point{Lat: *lat, Lng: *lng}
	}
	if warehouseID != nil {
		o.WarehouseID = *warehouseID
	}

	rows, err := s.db.Query(ctx, `
		SELECT product_id, quantity, unit_price_cents
		FROM order_items WHERE order_id = $1`,
		o.ID)
	if err != nil {
		return order.Order{}, err
	}
	o.Items, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (order.Line, error) {
		var l order.Line
		err := row.Scan(&l.ProductID, &l.Quantity, &l.UnitPriceCents)
		return l, err
	})
	if err != nil {
		return order.Order{}, err
	}
	return o, nil
}

func (s *Store) Transition(ctx context.Context, id uuid.UUID, from, to order.Status) (bool, error) {
	tag, err := s.db.Exec(ctx, `UPDATE orders SET status = $3 WHERE id = $1 AND status = $2`, id, from, to)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *Store) SetLocation(ctx context.Context, id uuid.UUID, p geocoding.Point) error {
	_, err := s.db.Exec(ctx, `UPDATE orders SET lat = $2, lng = $3 WHERE id = $1`, id, p.Lat, p.Lng)
	return err
}

func (s *Store) SetWarehouse(ctx context.Context, id, warehouseID uuid.UUID) error {
	_, err := s.db.Exec(ctx, `UPDATE orders SET warehouse_id = $2 WHERE id = $1`, id, warehouseID)
	return err
}
