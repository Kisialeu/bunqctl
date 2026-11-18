package bunq

import "context"

// MaxDraftEntries is the most entries bunq accepts in one draft payment.
const MaxDraftEntries = 350

// DraftEntry is one payment inside a draft.
type DraftEntry struct {
	Amount       Amount  `json:"amount"`
	Counterparty Pointer `json:"counterparty_alias"`
	Description  string  `json:"description"`
}

// NewDraftPayment is a payment that must be accepted in the bunq app.
type NewDraftPayment struct {
	Entries  []DraftEntry `json:"entries"`
	Accepts  int          `json:"number_of_required_accepts"` // only 1 is valid
	Schedule *Schedule    `json:"schedule,omitempty"`
}

// DraftPayment is the state of a draft.
type DraftPayment struct {
	ID      int    `json:"id"`
	Updated string `json:"updated"` // sent back verbatim when changing the draft
	Status  string `json:"status"`  // PENDING, ACCEPTED, REJECTED, EXPIRED
	Entries []struct {
		Amount       Amount               `json:"amount"`
		Counterparty LabelMonetaryAccount `json:"counterparty_alias"`
		Description  string               `json:"description"`
	} `json:"entries"`
}

func draftPath(userID, accountID int, rest ...any) string {
	return apiPath(append([]any{"user", userID, "monetary-account", accountID, "draft-payment"}, rest...)...)
}

// CreateDraftPayment creates a draft for approval in the app and returns its id.
func (c *Client) CreateDraftPayment(ctx context.Context, userID, accountID int, d NewDraftPayment) (int, error) {
	if d.Accepts == 0 {
		d.Accepts = 1
	}
	return create(ctx, c, draftPath(userID, accountID), d)
}

// DraftPayment returns the state of a draft.
func (c *Client) DraftPayment(ctx context.Context, userID, accountID, id int) (DraftPayment, error) {
	return get[DraftPayment](ctx, c, draftPath(userID, accountID, id))
}

// CancelDraftPayment rejects a pending draft; bunq has no CANCELLED status
// for drafts. It never sends ACCEPTED, which bunq would also take and which
// would approve the payment without the app. bunq needs the last updated
// timestamp to detect concurrent changes, so the draft is read first.
func (c *Client) CancelDraftPayment(ctx context.Context, userID, accountID, id int) error {
	d, err := c.DraftPayment(ctx, userID, accountID, id)
	if err != nil {
		return err
	}
	return update(ctx, c, draftPath(userID, accountID, id), map[string]any{
		"status":                     "REJECTED",
		"previous_updated_timestamp": d.Updated,
	})
}
