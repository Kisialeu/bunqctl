package bunq

import (
	"context"
	"iter"
)

// Payment is a booked transaction on a monetary account. Amount is negative
// for outgoing payments.
type Payment struct {
	ID           int                  `json:"id"`
	AccountID    int                  `json:"monetary_account_id"`
	Created      Time                 `json:"created"`
	Amount       Amount               `json:"amount"`
	Description  string               `json:"description"`
	Type         string               `json:"type"`
	SubType      string               `json:"sub_type"`
	Counterparty LabelMonetaryAccount `json:"counterparty_alias"`
	BalanceAfter Amount               `json:"balance_after_mutation"`
}

// NewPayment is the body of a direct payment.
type NewPayment struct {
	Amount       Amount  `json:"amount"`
	Counterparty Pointer `json:"counterparty_alias"`
	Description  string  `json:"description"`
}

// Payments iterates the payments of an account, newest first.
func (c *Client) Payments(ctx context.Context, userID, accountID int) iter.Seq2[Payment, error] {
	return listAs[Payment](ctx, c, apiPath("user", userID, "monetary-account", accountID, "payment"), nil)
}

// CreatePayment makes a direct payment from an account and returns its id.
func (c *Client) CreatePayment(ctx context.Context, userID, accountID int, p NewPayment) (int, error) {
	return create(ctx, c, apiPath("user", userID, "monetary-account", accountID, "payment"), p)
}
