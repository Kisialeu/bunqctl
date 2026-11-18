package cli

import (
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Kisialeu/bunqctl/internal/bunq"
	"github.com/Kisialeu/bunqctl/internal/output"
)

type eventView struct {
	ID           int       `json:"id"`
	Date         time.Time `json:"date"`
	Action       string    `json:"action"`
	Type         string    `json:"type"`
	AccountID    int       `json:"account_id"`
	Amount       string    `json:"amount,omitempty"`
	Currency     string    `json:"currency,omitempty"`
	Counterparty string    `json:"counterparty,omitempty"`
	Description  string    `json:"description,omitempty"`
}

func viewEvent(e bunq.Event) eventView {
	v := eventView{
		ID: e.ID, Date: e.Created.Time, Action: e.Action, Type: e.Object.Type,
		AccountID: int(e.AccountID), Counterparty: e.Object.Counterparty, Description: e.Object.Description,
	}
	if e.Object.Amount != nil {
		v.Amount, v.Currency = e.Object.Amount.Value, e.Object.Amount.Currency
	}
	return v
}

var eventColumns = []output.Column[bunq.Event]{
	{Header: "DATE", Value: func(e bunq.Event) string { return e.Created.Local().Format("2006-01-02 15:04") }},
	{Header: "TYPE", Value: func(e bunq.Event) string { return e.Object.Type }},
	{Header: "ACTION", Value: func(e bunq.Event) string { return strings.ToLower(e.Action) }},
	{Header: "AMOUNT", Value: func(e bunq.Event) string { return viewEvent(e).Amount }, Right: true},
	{Header: "COUNTERPARTY", Value: func(e bunq.Event) string { return e.Object.Counterparty }},
	{Header: "DESCRIPTION", Value: func(e bunq.Event) string { return oneLine(e.Object.Description, 40) }},
	{Header: "ID", Value: func(e bunq.Event) string { return strconv.Itoa(e.ID) }},
}

func newEventsCmd(a *app) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Activity feed: payments, card transactions, requests and more",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if limit <= 0 {
				return usageErr("--limit must be positive")
			}
			c, userID, err := a.session()
			if err != nil {
				return err
			}
			var events []bunq.Event
			for e, err := range c.Events(cmd.Context(), userID) {
				if err != nil {
					return err
				}
				if events = append(events, e); len(events) == limit {
					break
				}
			}
			return output.List(a.printer(), events, viewEvent, eventColumns)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "maximum number of events")
	return cmd
}
