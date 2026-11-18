package bunq

import (
	"context"
	"encoding/json"
	"iter"
)

// MonetaryAccount is any kind of bunq account (bank, savings, joint, ...).
type MonetaryAccount struct {
	Type        string   `json:"-"` // bunq type name, e.g. MonetaryAccountBank
	ID          int      `json:"id"`
	Description string   `json:"description"`
	Currency    string   `json:"currency"`
	Status      string   `json:"status"`
	Balance     Amount   `json:"balance"`
	Alias       Pointers `json:"alias"`

	// Savings accounts only.
	SavingsGoal         *Amount     `json:"savings_goal"`
	SavingsGoalProgress json.Number `json:"savings_goal_progress"`
}

func (a *MonetaryAccount) setType(t string) { a.Type = t }

// IBAN returns the IBAN alias of the account, or "".
func (a MonetaryAccount) IBAN() string { return a.Alias.Value("IBAN") }

// MonetaryAccounts iterates all accounts of the user.
func (c *Client) MonetaryAccounts(ctx context.Context, userID int) iter.Seq2[MonetaryAccount, error] {
	return listAs[MonetaryAccount](ctx, c, apiPath("user", userID, "monetary-account"), nil)
}
