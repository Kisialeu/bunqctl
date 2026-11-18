package bunq

import (
	"context"
	"iter"
)

// NewRequestInquiry asks a counterparty for money.
type NewRequestInquiry struct {
	Amount      Amount  `json:"amount_inquired"`
	From        Pointer `json:"counterparty_alias"`
	Description string  `json:"description"`
	AllowBunqMe bool    `json:"allow_bunqme"`
}

// RequestInquiry is a sent money request.
type RequestInquiry struct {
	ID           int                  `json:"id"`
	Created      Time                 `json:"created"`
	Status       string               `json:"status"`
	Amount       Amount               `json:"amount_inquired"`
	Counterparty LabelMonetaryAccount `json:"counterparty_alias"`
	Description  string               `json:"description"`
}

func requestPath(userID, accountID int, rest ...any) string {
	return apiPath(append([]any{"user", userID, "monetary-account", accountID, "request-inquiry"}, rest...)...)
}

// CreateRequestInquiry sends a money request from the given account and
// returns its id.
func (c *Client) CreateRequestInquiry(ctx context.Context, userID, accountID int, r NewRequestInquiry) (int, error) {
	return create(ctx, c, requestPath(userID, accountID), r)
}

// RequestInquiries iterates the money requests of an account.
func (c *Client) RequestInquiries(ctx context.Context, userID, accountID int) iter.Seq2[RequestInquiry, error] {
	return listAs[RequestInquiry](ctx, c, requestPath(userID, accountID), nil)
}

// RevokeRequestInquiry withdraws a pending money request.
func (c *Client) RevokeRequestInquiry(ctx context.Context, userID, accountID, id int) error {
	return update(ctx, c, requestPath(userID, accountID, id), map[string]string{"status": "REVOKED"})
}
