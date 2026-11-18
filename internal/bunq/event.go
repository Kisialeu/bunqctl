package bunq

import (
	"context"
	"encoding/json"
	"iter"
)

// Event is one entry of the user's activity feed.
type Event struct {
	ID        int         `json:"id"`
	Created   Time        `json:"created"`
	Action    string      `json:"action"`
	Status    string      `json:"status"`
	AccountID Int         `json:"monetary_account_id"`
	Object    EventObject `json:"object"`
}

// EventObject is the object an event is about, e.g. a Payment. Only the
// fields shared by most object types are decoded.
type EventObject struct {
	Type         string
	Amount       *Amount
	Description  string
	Counterparty string
}

// UnmarshalJSON decodes {"<Type>": {...}}.
func (o *EventObject) UnmarshalJSON(b []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	for typ, body := range m {
		var v struct {
			Amount         *Amount              `json:"amount"`
			AmountBilling  *Amount              `json:"amount_billing"`
			AmountInquired *Amount              `json:"amount_inquired"`
			Description    string               `json:"description"`
			Counterparty   LabelMonetaryAccount `json:"counterparty_alias"`
		}
		if err := json.Unmarshal(body, &v); err != nil {
			return err
		}
		o.Type, o.Description, o.Counterparty = typ, v.Description, v.Counterparty.DisplayName
		for _, a := range []*Amount{v.Amount, v.AmountBilling, v.AmountInquired} {
			if a != nil {
				o.Amount = a
				break
			}
		}
	}
	return nil
}

// Events iterates the activity feed of the user, newest first.
func (c *Client) Events(ctx context.Context, userID int) iter.Seq2[Event, error] {
	return listAs[Event](ctx, c, apiPath("user", userID, "event"), nil)
}
