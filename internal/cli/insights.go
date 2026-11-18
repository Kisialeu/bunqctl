package cli

import (
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/Kisialeu/bunqctl/internal/bunq"
	"github.com/Kisialeu/bunqctl/internal/output"
)

type insightView struct {
	Category     string `json:"category"`
	Total        string `json:"total"`
	Currency     string `json:"currency"`
	Transactions int    `json:"transactions"`
}

func newInsightsCmd(a *app) *cobra.Command {
	var from, to string
	cmd := &cobra.Command{
		Use:   "insights",
		Short: "Spending per category (bunq's own categorisation)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := (&txFlags{from: from, to: to}).filter()
			if err != nil {
				return err
			}
			if f.from.IsZero() {
				y, m, _ := time.Now().Date()
				f.from = time.Date(y, m, 1, 0, 0, 0, 0, time.Local)
			}
			if f.to.IsZero() {
				f.to = time.Now()
			}
			c, userID, err := a.session()
			if err != nil {
				return err
			}
			items, err := collect(c.Insights(cmd.Context(), userID, f.from, f.to))
			if err != nil {
				return err
			}
			return output.List(a.printer(), items,
				func(i bunq.Insight) insightView {
					return insightView{i.Category.Description, i.Total.Value, i.Total.Currency, i.Transactions}
				},
				[]output.Column[bunq.Insight]{
					{Header: "CATEGORY", Value: func(i bunq.Insight) string { return i.Category.Description }},
					{Header: "TOTAL", Value: func(i bunq.Insight) string { return i.Total.Value }, Right: true},
					{Header: "CUR", Value: func(i bunq.Insight) string { return i.Total.Currency }},
					{Header: "COUNT", Value: func(i bunq.Insight) string { return strconv.Itoa(i.Transactions) }, Right: true},
				})
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "first day, YYYY-MM-DD (default: first day of this month)")
	cmd.Flags().StringVar(&to, "to", "", "last day, inclusive, YYYY-MM-DD (default: now)")
	return cmd
}
