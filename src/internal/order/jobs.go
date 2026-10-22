package order

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"

	"orders-api/internal/geocoding"
	"orders-api/internal/payment"
	"orders-api/internal/warehouse"
)

// The Service methods in this file move an order through its state machine,
// one step each. A job runs each step, and a step enqueues the next one in
// the same transaction that moves the order. A step does nothing when the
// order already left the status it starts from, so running it twice is safe.

// reserveAttempts is how many times AllocateStock picks a warehouse again
// after another order took its stock, before returning an error so the job
// is retried.
const reserveAttempts = 3

// errStockTaken means another order reserved the stock first.
var errStockTaken = errors.New("stock taken by another order")

// GeocodeAddress moves a PENDING order to GEOCODED, or to FAILED when its
// address can't be found.
func (s *Service) GeocodeAddress(ctx context.Context, id uuid.UUID) error {
	o, err := s.orders.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("getting order %s: %w", id, err)
	}
	if o.Status != StatusPending {
		return nil
	}

	p, err := s.geocoder.Geocode(ctx, o.ShippingAddress)
	if errors.Is(err, geocoding.ErrNotFound) {
		return s.advance(ctx, o.ID, StatusPending, StatusFailed, nil)
	}
	if err != nil {
		return fmt.Errorf("order %s: geocoding: %w", o.ID, err)
	}
	return s.advance(ctx, o.ID, StatusPending, StatusGeocoded, func(tx Tx) error {
		if err := tx.Orders().SetLocation(ctx, o.ID, p); err != nil {
			return err
		}
		return tx.Enqueue(ctx, StatusGeocoded, o.ID)
	})
}

// AllocateStock moves a GEOCODED order to RESERVED, or to FAILED when no
// single warehouse has every item.
func (s *Service) AllocateStock(ctx context.Context, id uuid.UUID) error {
	o, err := s.orders.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("getting order %s: %w", id, err)
	}
	if o.Status != StatusGeocoded {
		return nil
	}
	items := stockItems(o)

	for range reserveAttempts {
		warehouseID, err := s.warehouses.Closest(ctx, *o.Location, items)
		if errors.Is(err, warehouse.ErrNoWarehouse) {
			return s.advance(ctx, o.ID, StatusGeocoded, StatusFailed, nil)
		}
		if err != nil {
			return fmt.Errorf("order %s: finding a warehouse: %w", o.ID, err)
		}

		err = s.advance(ctx, o.ID, StatusGeocoded, StatusReserved, func(tx Tx) error {
			reserved, err := tx.Warehouses().Reserve(ctx, warehouseID, items)
			if err != nil {
				return err
			}
			if !reserved {
				return errStockTaken
			}
			if err := tx.Orders().SetWarehouse(ctx, o.ID, warehouseID); err != nil {
				return err
			}
			return tx.Enqueue(ctx, StatusReserved, o.ID)
		})
		if !errors.Is(err, errStockTaken) {
			return err
		}
	}
	return fmt.Errorf("order %s: %w %d times", o.ID, errStockTaken, reserveAttempts)
}

// ChargePayment moves a RESERVED order to CONFIRMED, or to RELEASING when the
// payment is declined.
func (s *Service) ChargePayment(ctx context.Context, id uuid.UUID) error {
	o, err := s.orders.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("getting order %s: %w", id, err)
	}
	if o.Status != StatusReserved {
		return nil
	}
	c, err := s.customers.Get(ctx, o.CustomerID)
	if err != nil {
		return fmt.Errorf("order %s: getting customer %s: %w", o.ID, o.CustomerID, err)
	}

	err = s.gateway.Charge(ctx, payment.Charge{
		CardNumber:     c.CardNumber,
		AmountCents:    o.TotalCents,
		Description:    "Order " + o.ID.String(),
		IdempotencyKey: o.ID.String(),
	})
	if errors.Is(err, payment.ErrDeclined) {
		return s.advance(ctx, o.ID, StatusReserved, StatusReleasing, func(tx Tx) error {
			return tx.Enqueue(ctx, StatusReleasing, o.ID)
		})
	}
	if err != nil {
		return fmt.Errorf("order %s: charging: %w", o.ID, err)
	}
	return s.advance(ctx, o.ID, StatusReserved, StatusConfirmed, nil)
}

// ReleaseStock moves a RELEASING order to FAILED after giving its reserved
// stock back.
func (s *Service) ReleaseStock(ctx context.Context, id uuid.UUID) error {
	o, err := s.orders.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("getting order %s: %w", id, err)
	}
	if o.Status != StatusReleasing {
		return nil
	}
	return s.advance(ctx, o.ID, StatusReleasing, StatusFailed, func(tx Tx) error {
		return tx.Warehouses().Release(ctx, o.WarehouseID, stockItems(o))
	})
}

// Discarded handles an order whose step, run while the order waits in
// waiting, ran out of retries, and reports whether the step must be retried.
// Orders still waiting to be geocoded or allocated hold no stock and move to
// FAILED. A discarded charge or release is retried, forever: the charge is
// safe to repeat because the order id is the payment's idempotency key, and
// reserved stock must always be given back. It does nothing when the order
// already moved on.
func (s *Service) Discarded(ctx context.Context, id uuid.UUID, waiting Status) (bool, error) {
	o, err := s.orders.Get(ctx, id)
	if err != nil {
		return false, fmt.Errorf("getting order %s: %w", id, err)
	}
	if o.Status != waiting {
		return false, nil
	}

	switch waiting {
	case StatusPending, StatusGeocoded:
		if err := s.advance(ctx, o.ID, waiting, StatusFailed, nil); err != nil {
			return false, err
		}
		log.Printf("order %s: step discarded while %s, moved %s -> %s", o.ID, waiting, waiting, StatusFailed)
		return false, nil
	default:
		return true, nil
	}
}

// advance moves the order from one status to another and runs apply (if any)
// in the same transaction. If the order already left from, the step was a
// duplicate and nothing happens. If apply fails, nothing is saved.
func (s *Service) advance(ctx context.Context, id uuid.UUID, from, to Status, apply func(tx Tx) error) error {
	return s.uow.Do(ctx, func(tx Tx) error {
		moved, err := tx.Orders().Transition(ctx, id, from, to)
		if err != nil {
			return fmt.Errorf("order %s %s -> %s: %w", id, from, to, err)
		}
		if moved && apply != nil {
			if err := apply(tx); err != nil {
				return fmt.Errorf("order %s %s -> %s: %w", id, from, to, err)
			}
		}
		return nil
	})
}

// stockItems returns the quantities o takes from stock.
func stockItems(o Order) []warehouse.Item {
	items := make([]warehouse.Item, len(o.Items))
	for i, l := range o.Items {
		items[i] = warehouse.Item{ProductID: l.ProductID, Quantity: l.Quantity}
	}
	return items
}
