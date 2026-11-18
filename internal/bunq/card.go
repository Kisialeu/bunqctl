package bunq

import (
	"context"
	"fmt"
	"iter"
)

// Reversible card statuses. LOST, STOLEN and CANCELLED are permanent and are
// deliberately not offered.
const (
	CardActive      = "ACTIVE"
	CardDeactivated = "DEACTIVATED"
)

// Card is a payment card.
type Card struct {
	ID          int                  `json:"id"`
	Type        string               `json:"type"`
	ProductType string               `json:"product_type"`
	Status      string               `json:"status"`
	OrderStatus string               `json:"order_status"`
	SecondLine  string               `json:"second_line"`
	NameOnCard  string               `json:"name_on_card"`
	ExpiryDate  string               `json:"expiry_date"`
	Account     LabelMonetaryAccount `json:"label_monetary_account_current"`
	Numbers     []struct {
		FourDigit string `json:"four_digit"`
		Status    string `json:"status"`
	} `json:"primary_account_numbers"`
}

// LastDigits returns the last four digits of the first card number, or "".
func (c Card) LastDigits() string {
	if len(c.Numbers) == 0 {
		return ""
	}
	return c.Numbers[0].FourDigit
}

// Cards iterates the user's cards.
func (c *Client) Cards(ctx context.Context, userID int) iter.Seq2[Card, error] {
	return listAs[Card](ctx, c, apiPath("user", userID, "card"), nil)
}

// Card returns one card.
func (c *Client) Card(ctx context.Context, userID, cardID int) (Card, error) {
	return get[Card](ctx, c, apiPath("user", userID, "card", cardID))
}

// SetCardStatus changes a card's status. Only CardActive and CardDeactivated
// are accepted, so a typo can never block a card permanently.
func (c *Client) SetCardStatus(ctx context.Context, userID, cardID int, status string) error {
	if status != CardActive && status != CardDeactivated {
		return fmt.Errorf("refusing card status %q: only %s and %s are reversible", status, CardActive, CardDeactivated)
	}
	return update(ctx, c, apiPath("user", userID, "card", cardID), map[string]string{"status": status})
}
