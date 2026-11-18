package bunq

import (
	"context"
	"iter"
)

// Limit is the user's account and spending limits.
type Limit struct {
	Accounts          int     `json:"limit_monetary_account"`
	AccountsRemaining int     `json:"limit_monetary_account_remaining"`
	MonthlyLimit      *Amount `json:"limit_amount_monthly"`
	MonthlySpent      *Amount `json:"spent_amount_monthly"`
	CardDebitMaestro  int     `json:"limit_card_debit_maestro"`
	CardDebitMaster   int     `json:"limit_card_debit_mastercard"`
	CardReplacement   int     `json:"limit_card_replacement"`
}

// Limits iterates the user's limits (bunq returns one item).
func (c *Client) Limits(ctx context.Context, userID int) iter.Seq2[Limit, error] {
	return listAs[Limit](ctx, c, apiPath("user", userID, "limit"), nil)
}
