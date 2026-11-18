package bunq

import (
	"context"
	"iter"
)

// Schedule recurrence units.
const (
	RecurOnce    = "ONCE"
	RecurDaily   = "DAILY"
	RecurWeekly  = "WEEKLY"
	RecurMonthly = "MONTHLY"
	RecurYearly  = "YEARLY"
)

// Schedule describes when a scheduled payment runs. Times are UTC in the
// bunq timestamp format.
type Schedule struct {
	Start  string `json:"time_start"`
	End    string `json:"time_end,omitempty"`
	Unit   string `json:"recurrence_unit"`
	Size   int    `json:"recurrence_size"`
	Status string `json:"status,omitempty"`
}

// ScheduledPayment is a standing order.
type ScheduledPayment struct {
	ID      int    `json:"id"`
	Status  string `json:"status"`
	Payment struct {
		Amount       Amount               `json:"amount"`
		Counterparty LabelMonetaryAccount `json:"counterparty_alias"`
		Description  string               `json:"description"`
	} `json:"payment"`
	Schedule Schedule `json:"schedule"`
}

// NewScheduledPayment is the body of a new standing order.
type NewScheduledPayment struct {
	Payment  NewPayment `json:"payment"`
	Schedule Schedule   `json:"schedule"`
}

func schedulePath(userID, accountID int, rest ...any) string {
	return apiPath(append([]any{"user", userID, "monetary-account", accountID, "schedule-payment"}, rest...)...)
}

// ScheduledPayments iterates the standing orders of an account.
func (c *Client) ScheduledPayments(ctx context.Context, userID, accountID int) iter.Seq2[ScheduledPayment, error] {
	return listAs[ScheduledPayment](ctx, c, schedulePath(userID, accountID), nil)
}

// CreateScheduledPayment creates a standing order and returns its id.
func (c *Client) CreateScheduledPayment(ctx context.Context, userID, accountID int, p NewScheduledPayment) (int, error) {
	return create(ctx, c, schedulePath(userID, accountID), p)
}

// CancelScheduledPayment deletes a standing order.
func (c *Client) CancelScheduledPayment(ctx context.Context, userID, accountID, id int) error {
	return remove(ctx, c, schedulePath(userID, accountID, id))
}
