package bunq

import (
	"context"
	"iter"
	"net/http"
)

// Category is a transaction category, built in or user defined.
type Category struct {
	Category    string `json:"category"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Color       string `json:"color"`
	Icon        string `json:"icon"`
}

// Categories iterates the available transaction categories.
func (c *Client) Categories(ctx context.Context, userID int) iter.Seq2[Category, error] {
	return listAs[Category](ctx, c, apiPath("user", userID, "additional-transaction-information-category"), nil)
}

// NewCategory is a user-defined category.
type NewCategory struct {
	Category    string `json:"category"`
	Status      string `json:"status"` // ACTIVE
	Description string `json:"description"`
	Color       string `json:"color,omitempty"`
	Icon        string `json:"icon,omitempty"`
}

// CreateCategory creates or replaces a user-defined category. bunq returns
// no id for it.
func (c *Client) CreateCategory(ctx context.Context, userID int, cat NewCategory) error {
	_, err := c.do(ctx, http.MethodPost, apiPath("user", userID, "additional-transaction-information-category-user-defined"), nil, cat)
	return err
}
