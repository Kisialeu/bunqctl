package cli

import (
	"errors"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/Kisialeu/bunqctl/internal/bunq"
	"github.com/Kisialeu/bunqctl/internal/output"
)

func newLimitsCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "limits",
		Short: "Account, card and monthly spending limits",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, userID, err := a.session()
			if err != nil {
				return err
			}
			limits, err := collect(c.Limits(cmd.Context(), userID))
			if err != nil {
				return err
			}
			if len(limits) == 0 {
				return errors.New("bunq returned no limits")
			}
			l := limits[0]
			return a.printer().Fields(l, [][2]string{
				{"Monthly limit", amountText(l.MonthlyLimit)},
				{"Spent this month", amountText(l.MonthlySpent)},
				{"Accounts", strconv.Itoa(l.Accounts)},
				{"Accounts remaining", strconv.Itoa(l.AccountsRemaining)},
				{"Maestro cards", strconv.Itoa(l.CardDebitMaestro)},
				{"Mastercard debit cards", strconv.Itoa(l.CardDebitMaster)},
				{"Card replacements", strconv.Itoa(l.CardReplacement)},
			})
		},
	}
}

func amountText(a *bunq.Amount) string {
	if a == nil {
		return "-"
	}
	return a.Value + " " + a.Currency
}

func newAutoAllocateCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{Use: "auto-allocate", Short: "Rules that split incoming payments"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List auto-allocate rules",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, userID, err := a.session()
			if err != nil {
				return err
			}
			rules, err := collect(c.AutoAllocations(cmd.Context(), userID))
			if err != nil {
				return err
			}
			return output.List(a.printer(), rules, func(r bunq.AutoAllocation) bunq.AutoAllocation { return r },
				[]output.Column[bunq.AutoAllocation]{
					{Header: "ID", Value: func(r bunq.AutoAllocation) string { return strconv.Itoa(r.ID) }},
					{Header: "TYPE", Value: func(r bunq.AutoAllocation) string { return r.Type }},
					{Header: "STATUS", Value: func(r bunq.AutoAllocation) string { return r.Status }},
					{Header: "TRIGGER", Value: func(r bunq.AutoAllocation) string { return r.TriggerAmount.Value }, Right: true},
				})
		},
	})
	return cmd
}
