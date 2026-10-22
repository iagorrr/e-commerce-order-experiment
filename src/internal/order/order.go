// Package order takes orders and moves them through their state machine.
package order

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"orders-api/internal/db"
	"orders-api/internal/geocoding"
	"orders-api/internal/warehouse"
)

var (
	ErrNotFound = errors.New("order not found")
	// ErrKeyTaken means the customer already used the order's idempotency
	// key.
	ErrKeyTaken = errors.New("idempotency key already taken")
	// ErrKeyReused means an idempotency key was sent again with a different
	// request than the one that first used it.
	ErrKeyReused = errors.New("idempotency key already used for a different request")
	// ErrCustomerNotFound means the order's customer doesn't exist.
	ErrCustomerNotFound = errors.New("customer not found")
	// ErrInvalidID means an ID is missing or isn't a version 7 UUID.
	ErrInvalidID = errors.New("invalid id")
	// ErrInvalidShippingAddress means the shipping address is missing.
	ErrInvalidShippingAddress = errors.New("invalid shipping address")
	// ErrInvalidItems means the items are missing, repeat a product or have
	// a quantity that isn't positive.
	ErrInvalidItems = errors.New("invalid items")
)

// Status is where an order is in its state machine. CONFIRMED and FAILED are
// final. PAYMENT_UNKNOWN is deferred past v1.
type Status string

const (
	StatusPending   Status = "PENDING"
	StatusGeocoded  Status = "GEOCODED"
	StatusReserved  Status = "RESERVED"
	StatusReleasing Status = "RELEASING"
	StatusConfirmed Status = "CONFIRMED"
	StatusFailed    Status = "FAILED"
)

// NewOrder is what Create needs to take an order.
type NewOrder struct {
	CustomerID      uuid.UUID
	IdempotencyKey  uuid.UUID
	ShippingAddress string
	Items           []Item
}

// Validate reports whether n can be taken as an order. Its error wraps
// ErrInvalidID, ErrInvalidShippingAddress or ErrInvalidItems.
func (n NewOrder) Validate() error {
	if err := validateID("idempotency key", n.IdempotencyKey); err != nil {
		return err
	}

	if err := validateID("customer id", n.CustomerID); err != nil {
		return err
	}

	if n.ShippingAddress == "" {
		return fmt.Errorf("%w: shipping address is required", ErrInvalidShippingAddress)
	}

	if len(n.Items) == 0 {
		return fmt.Errorf("%w: an order must have at least one item", ErrInvalidItems)
	}

	seen := make(map[uuid.UUID]bool, len(n.Items))
	for i, it := range n.Items {
		if err := it.Validate(); err != nil {
			return fmt.Errorf("items[%d]: %w", i, err)
		}
		if seen[it.ProductID] {
			return fmt.Errorf("%w: each product can appear in only one item", ErrInvalidItems)
		}
		seen[it.ProductID] = true
	}

	return nil
}

// Item is a requested quantity of one product.
type Item struct {
	ProductID uuid.UUID
	Quantity  int
}

// Validate reports whether i is a valid item. Its error wraps ErrInvalidID
// or ErrInvalidItems.
func (i Item) Validate() error {
	if err := validateID("product id", i.ProductID); err != nil {
		return err
	}

	if i.Quantity <= 0 {
		return fmt.Errorf("%w: quantity must be greater than 0", ErrInvalidItems)
	}

	return nil
}

// validateID reports, under name, whether id is missing or isn't a version 7
// UUID. Its error wraps ErrInvalidID.
func validateID(name string, id uuid.UUID) error {
	if id == uuid.Nil {
		return fmt.Errorf("%w: %s is required", ErrInvalidID, name)
	}
	if id.Version() != 7 || id.Variant() != uuid.RFC4122 {
		return fmt.Errorf("%w: %s must be a version 7 UUID", ErrInvalidID, name)
	}

	return nil
}

// Order is a stored order.
type Order struct {
	ID              uuid.UUID
	CustomerID      uuid.UUID
	IdempotencyKey  uuid.UUID
	ShippingAddress string
	Status          Status
	TotalCents      int64
	Items           []Line
	// Location is set once the order is GEOCODED.
	Location *geocoding.Point
	// WarehouseID is set once the order is RESERVED.
	WarehouseID uuid.UUID
}

// Line is an ordered item with the unit price charged for it.
type Line struct {
	ProductID      uuid.UUID
	Quantity       int
	UnitPriceCents int64
}

// Tx gives access to what runs inside one transaction.
type Tx interface {
	Orders() Store
	Warehouses() warehouse.Store
	// Enqueue enqueues the step that moves the order out of s.
	Enqueue(ctx context.Context, s Status, orderID uuid.UUID) error
}

// UnitOfWork runs fn in one transaction: if fn returns an error, nothing it
// did is kept.
type UnitOfWork interface {
	Do(ctx context.Context, fn func(tx Tx) error) error
}

// Store persists orders.
type Store interface {
	// WithTx returns a Store that runs its queries inside tx.
	WithTx(tx db.DBTX) Store
	// Insert stores o and its lines and returns it with its generated ID, or
	// ErrKeyTaken when the customer already used o's idempotency key.
	Insert(ctx context.Context, o Order) (Order, error)
	// Get returns the order with its lines, or ErrNotFound.
	Get(ctx context.Context, id uuid.UUID) (Order, error)
	// GetByIdempotencyKey returns the order with its lines, or ErrNotFound
	// when the customer never used key.
	GetByIdempotencyKey(ctx context.Context, customerID, key uuid.UUID) (Order, error)
	// Transition moves the order from one status to another only if it is
	// still in from, and reports whether it moved.
	Transition(ctx context.Context, id uuid.UUID, from, to Status) (bool, error)
	// SetLocation stores where the order ships to.
	SetLocation(ctx context.Context, id uuid.UUID, p geocoding.Point) error
	// SetWarehouse stores the warehouse the order's stock is reserved in.
	SetWarehouse(ctx context.Context, id, warehouseID uuid.UUID) error
}
