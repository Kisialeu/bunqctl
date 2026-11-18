package cli

import (
	"context"
	"fmt"
	"iter"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Kisialeu/bunqctl/internal/bunq"
	"github.com/Kisialeu/bunqctl/internal/iban"
	"github.com/Kisialeu/bunqctl/internal/output"
)

const statusActive = "ACTIVE"

// accountView is the stable JSON schema of an account.
type accountView struct {
	ID          int    `json:"id"`
	Type        string `json:"type"`
	Description string `json:"description"`
	IBAN        string `json:"iban"`
	Balance     string `json:"balance"`
	Currency    string `json:"currency"`
	Status      string `json:"status"`
}

func viewAccount(m bunq.MonetaryAccount) accountView {
	return accountView{
		ID:          m.ID,
		Type:        strings.TrimPrefix(m.Type, "MonetaryAccount"),
		Description: m.Description,
		IBAN:        m.IBAN(),
		Balance:     m.Balance.Value,
		Currency:    m.Balance.Currency,
		Status:      m.Status,
	}
}

var accountColumns = []output.Column[bunq.MonetaryAccount]{
	{Header: "ID", Value: func(m bunq.MonetaryAccount) string { return strconv.Itoa(m.ID) }},
	{Header: "DESCRIPTION", Value: func(m bunq.MonetaryAccount) string { return m.Description }},
	{Header: "IBAN", Value: bunq.MonetaryAccount.IBAN},
	{Header: "BALANCE", Value: func(m bunq.MonetaryAccount) string { return m.Balance.Value }, Right: true},
	{Header: "CUR", Value: func(m bunq.MonetaryAccount) string { return m.Balance.Currency }},
	{Header: "STATUS", Value: func(m bunq.MonetaryAccount) string { return m.Status }},
}

func newAccountsCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{Use: "accounts", Aliases: []string{"account"}, Short: "Show monetary accounts"}

	var all bool
	list := &cobra.Command{
		Use:   "list",
		Short: "List accounts with balances",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, userID, err := a.session()
			if err != nil {
				return err
			}
			accounts, err := collect(c.MonetaryAccounts(cmd.Context(), userID))
			if err != nil {
				return err
			}
			if !all {
				accounts = filter(accounts, func(m bunq.MonetaryAccount) bool { return m.Status == statusActive })
			}
			return output.List(a.printer(), accounts, viewAccount, accountColumns)
		},
	}
	list.Flags().BoolVar(&all, "all", false, "include closed and blocked accounts")

	show := &cobra.Command{
		Use:   "show <id|IBAN>",
		Short: "Show one account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, userID, err := a.session()
			if err != nil {
				return err
			}
			m, err := findAccount(cmd.Context(), c, userID, args[0])
			if err != nil {
				return err
			}
			v := viewAccount(m)
			return a.printer().Fields(v, [][2]string{
				{"ID", strconv.Itoa(v.ID)},
				{"Type", v.Type},
				{"Description", v.Description},
				{"IBAN", v.IBAN},
				{"Balance", v.Balance + " " + v.Currency},
				{"Status", v.Status},
			})
		},
	}

	savings := &cobra.Command{
		Use:   "savings",
		Short: "Savings accounts with goal and progress",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, userID, err := a.session()
			if err != nil {
				return err
			}
			accounts, err := collect(c.MonetaryAccounts(cmd.Context(), userID))
			if err != nil {
				return err
			}
			accounts = filter(accounts, func(m bunq.MonetaryAccount) bool { return m.Type == "MonetaryAccountSavings" })
			return output.List(a.printer(), accounts, viewSavings, []output.Column[bunq.MonetaryAccount]{
				accountColumns[0], accountColumns[1], accountColumns[3],
				{Header: "GOAL", Value: func(m bunq.MonetaryAccount) string { return viewSavings(m).Goal }, Right: true},
				{Header: "PROGRESS", Value: func(m bunq.MonetaryAccount) string { return viewSavings(m).Progress }, Right: true},
				accountColumns[5],
			})
		},
	}

	cmd.AddCommand(list, show, savings)
	return cmd
}

type savingsView struct {
	accountView
	Goal     string `json:"goal"`
	Progress string `json:"progress"`
}

func viewSavings(m bunq.MonetaryAccount) savingsView {
	v := savingsView{accountView: viewAccount(m), Progress: m.SavingsGoalProgress.String()}
	if m.SavingsGoal != nil {
		v.Goal = m.SavingsGoal.Value
	}
	return v
}

// findAccount resolves an account by id or IBAN. Commands that take an
// --account flag use it, so they all accept the same forms.
func findAccount(ctx context.Context, c *bunq.Client, userID int, ref string) (bunq.MonetaryAccount, error) {
	id, idErr := strconv.Atoi(ref)
	want := iban.Normalize(ref)
	for m, err := range c.MonetaryAccounts(ctx, userID) {
		if err != nil {
			return bunq.MonetaryAccount{}, err
		}
		if idErr == nil && m.ID == id || m.IBAN() == want {
			return m, nil
		}
	}
	return bunq.MonetaryAccount{}, fmt.Errorf("no account %q", ref)
}

// collect drains an iterator into a slice, stopping at the first error.
func collect[T any](seq iter.Seq2[T, error]) ([]T, error) {
	var out []T
	for v, err := range seq {
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func filter[T any](items []T, keep func(T) bool) []T {
	out := items[:0]
	for _, it := range items {
		if keep(it) {
			out = append(out, it)
		}
	}
	return out
}
