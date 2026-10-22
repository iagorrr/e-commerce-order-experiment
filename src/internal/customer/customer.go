// Package customer holds the customers who place orders.
package customer

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("customer not found")

// Customer is a buyer and the card their orders are charged to. Production
// would store a provider token instead of the card number.
type Customer struct {
	ID         uuid.UUID
	CardNumber string
}

// Store reads customers.
type Store interface {
	Get(ctx context.Context, id uuid.UUID) (Customer, error)
}
