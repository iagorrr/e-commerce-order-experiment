package order

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"orders-api/internal/customer"
	"orders-api/internal/geocoding"
	"orders-api/internal/payment"
	"orders-api/internal/product"
	"orders-api/internal/warehouse"
)

type Service struct {
	orders     Store
	products   product.Store
	customers  customer.Store
	warehouses warehouse.Store
	geocoder   geocoding.Geocoder
	gateway    payment.Gateway
	uow        UnitOfWork
}

func NewService(orders Store, products product.Store, customers customer.Store, warehouses warehouse.Store, geocoder geocoding.Geocoder, gateway payment.Gateway, uow UnitOfWork) *Service {
	return &Service{
		orders:     orders,
		products:   products,
		customers:  customers,
		warehouses: warehouses,
		geocoder:   geocoder,
		gateway:    gateway,
		uow:        uow,
	}
}

// Create stores n as a PENDING order and enqueues GeocodeAddress in the same
// transaction, and reports whether it replayed an earlier order instead. If
// the customer already used n's idempotency key for the same request, it
// returns that order; for a different request, it returns ErrKeyReused.
func (s *Service) Create(ctx context.Context, n NewOrder) (Order, bool, error) {
	if err := n.Validate(); err != nil {
		return Order{}, false, fmt.Errorf("the new order provided is invalid: %w", err)
	}
	o := Order{
		CustomerID:      n.CustomerID,
		IdempotencyKey:  n.IdempotencyKey,
		ShippingAddress: n.ShippingAddress,
		Status:          StatusPending,
		Items:           make([]Line, len(n.Items)),
	}
	ids := make([]uuid.UUID, len(n.Items))
	for i, it := range n.Items {
		ids[i] = it.ProductID
	}
	products, err := s.products.GetAll(ctx, ids)
	if err != nil {
		return Order{}, false, fmt.Errorf("getting products: %w", err)
	}
	for i, it := range n.Items {
		p, ok := products[it.ProductID]
		if !ok {
			return Order{}, false, fmt.Errorf("getting product %s: %w", it.ProductID, product.ErrNotFound)
		}
		o.Items[i] = Line{ProductID: p.ID, Quantity: it.Quantity, UnitPriceCents: p.PriceCents}
		o.TotalCents += int64(it.Quantity) * p.PriceCents
	}

	existing, err := s.orders.GetByIdempotencyKey(ctx, n.CustomerID, n.IdempotencyKey)
	if err == nil {
		return replay(existing, o)
	}
	if !errors.Is(err, ErrNotFound) {
		return Order{}, false, fmt.Errorf("looking up idempotency key: %w", err)
	}

	var inserted Order
	err = s.uow.Do(ctx, func(tx Tx) error {
		var err error
		inserted, err = tx.Orders().Insert(ctx, o)
		if err != nil {
			return err
		}
		return tx.Enqueue(ctx, StatusPending, inserted.ID)
	})

	if errors.Is(err, ErrKeyTaken) {
		// A concurrent request with the same key committed first. The
		// transaction is already rolled back, so this read doesn't hold a
		// second pool connection.
		existing, err := s.orders.GetByIdempotencyKey(ctx, n.CustomerID, n.IdempotencyKey)
		if err != nil {
			return Order{}, false, fmt.Errorf("looking up idempotency key: %w", err)
		}
		return replay(existing, o)
	}

	if err != nil {
		return Order{}, false, fmt.Errorf("inserting order: %w", err)
	}

	return inserted, false, nil
}

// Get returns the order with its lines, ErrInvalidID when id is missing or
// isn't a version 7 UUID, or ErrNotFound.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (Order, error) {
	if err := validateID("order id", id); err != nil {
		return Order{}, err
	}
	return s.orders.Get(ctx, id)
}

// replay returns existing as a replay of requested, or ErrKeyReused when they
// are different requests.
func replay(existing, requested Order) (Order, bool, error) {
	if !sameRequest(existing, requested) {
		return Order{}, false, ErrKeyReused
	}
	return existing, true, nil
}

// sameRequest reports whether a and b ship to the same address and have the
// same lines, at the same prices, in any order.
func sameRequest(a, b Order) bool {
	if a.ShippingAddress != b.ShippingAddress || len(a.Items) != len(b.Items) {
		return false
	}
	byProduct := func(x, y Line) int { return bytes.Compare(x.ProductID[:], y.ProductID[:]) }
	aLines, bLines := slices.Clone(a.Items), slices.Clone(b.Items)
	slices.SortFunc(aLines, byProduct)
	slices.SortFunc(bLines, byProduct)
	return slices.Equal(aLines, bLines)
}
