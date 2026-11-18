package bunq

import (
	"context"
	"iter"
	"net/url"
	"time"
)

// Insight is the spending of one category in a period.
type Insight struct {
	Category struct {
		Category    string `json:"category"`
		Description string `json:"description_translated"`
	} `json:"category"`
	Total        Amount `json:"amount_total"`
	Transactions int    `json:"number_of_transactions"`
}

// insightTimeLayout is the time format of the insights period parameters.
const insightTimeLayout = "2006-01-02 15:04:05"

// Insights iterates the spending per category in [from, to). bunq requires
// both bounds.
func (c *Client) Insights(ctx context.Context, userID int, from, to time.Time) iter.Seq2[Insight, error] {
	q := url.Values{
		"time_start": {from.UTC().Format(insightTimeLayout)},
		"time_end":   {to.UTC().Format(insightTimeLayout)},
	}
	return listAs[Insight](ctx, c, apiPath("user", userID, "insights"), q)
}
