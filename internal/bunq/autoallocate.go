package bunq

import (
	"context"
	"iter"
)

// AutoAllocation is a rule that splits incoming payments.
type AutoAllocation struct {
	ID            int    `json:"id"`
	Type          string `json:"type"` // ONCE or RECURRING
	Status        string `json:"status"`
	TriggerAmount Amount `json:"trigger_amount"`
}

// AutoAllocations iterates the user's auto-allocate rules.
func (c *Client) AutoAllocations(ctx context.Context, userID int) iter.Seq2[AutoAllocation, error] {
	return listAs[AutoAllocation](ctx, c, apiPath("user", userID, "payment-auto-allocate"), nil)
}
