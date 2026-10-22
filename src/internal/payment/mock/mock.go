// Package mock implements payment.Gateway without calling a provider.
package mock

import (
	"context"

	"orders-api/internal/payment"
)

// DeclinedCard is the card number Gateway declines.
const DeclinedCard = "4000000000000002"

var _ payment.Gateway = Gateway{}

// Gateway approves every charge except those to DeclinedCard.
type Gateway struct{}

func (Gateway) Charge(_ context.Context, c payment.Charge) error {
	if c.CardNumber == DeclinedCard {
		return payment.ErrDeclined
	}
	return nil
}
