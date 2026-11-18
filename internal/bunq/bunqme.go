package bunq

import (
	"context"
	"iter"
)

// BunqMeTab is a shareable bunq.me payment link.
type BunqMeTab struct {
	ID       int    `json:"id"`
	Created  Time   `json:"created"`
	Status   string `json:"status"`
	ShareURL string `json:"bunqme_tab_share_url"`
	Entry    struct {
		Amount      Amount `json:"amount_inquired"`
		Description string `json:"description"`
	} `json:"bunqme_tab_entry"`
}

func bunqMePath(userID, accountID int, rest ...any) string {
	return apiPath(append([]any{"user", userID, "monetary-account", accountID, "bunqme-tab"}, rest...)...)
}

// CreateBunqMeTab creates a payment link and returns its id.
func (c *Client) CreateBunqMeTab(ctx context.Context, userID, accountID int, amount Amount, description string) (int, error) {
	return create(ctx, c, bunqMePath(userID, accountID), map[string]any{
		"bunqme_tab_entry": map[string]any{"amount_inquired": amount, "description": description},
	})
}

// BunqMeTab returns one payment link, including its share URL.
func (c *Client) BunqMeTab(ctx context.Context, userID, accountID, id int) (BunqMeTab, error) {
	return get[BunqMeTab](ctx, c, bunqMePath(userID, accountID, id))
}

// BunqMeTabs iterates the payment links of an account.
func (c *Client) BunqMeTabs(ctx context.Context, userID, accountID int) iter.Seq2[BunqMeTab, error] {
	return listAs[BunqMeTab](ctx, c, bunqMePath(userID, accountID), nil)
}

// CancelBunqMeTab closes a payment link.
func (c *Client) CancelBunqMeTab(ctx context.Context, userID, accountID, id int) error {
	return update(ctx, c, bunqMePath(userID, accountID, id), map[string]string{"status": "CANCELLED"})
}
