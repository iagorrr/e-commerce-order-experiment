// Package payment charges cards through an external payment provider.
//
// For now it runs inside the orders service; it should become a separate
// service later.
package payment

import (
	"context"
	"errors"
)

var ErrDeclined = errors.New("card declined")

// Charge is a request to charge a card.
type Charge struct {
	CardNumber  string
	AmountCents int64
	Description string
	// IdempotencyKey makes a repeated charge with the same key charge only
	// once.
	IdempotencyKey string
}

// Gateway charges cards.
type Gateway interface {
	// Charge charges c. A declined card is returned as ErrDeclined; any other
	// error may be retried with the same IdempotencyKey.
	//
	// TODO: return the provider's result (payment ID, decline reason) instead
	// of only an error.
	Charge(ctx context.Context, c Charge) error
}
