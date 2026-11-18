package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Kisialeu/bunqctl/internal/bunq"
	"github.com/Kisialeu/bunqctl/internal/money"
	"github.com/Kisialeu/bunqctl/internal/output"
)

// txView is the stable JSON schema of a transaction.
type txView struct {
	ID           int       `json:"id"`
	AccountID    int       `json:"account_id"`
	Date         time.Time `json:"date"`
	Amount       string    `json:"amount"`
	Currency     string    `json:"currency"`
	Counterparty string    `json:"counterparty"`
	IBAN         string    `json:"counterparty_iban"`
	Description  string    `json:"description"`
	Type         string    `json:"type"`
	BalanceAfter string    `json:"balance_after"`
}

func viewTx(p bunq.Payment) txView {
	return txView{
		ID:           p.ID,
		AccountID:    p.AccountID,
		Date:         p.Created.Time,
		Amount:       p.Amount.Value,
		Currency:     p.Amount.Currency,
		Counterparty: p.Counterparty.DisplayName,
		IBAN:         p.Counterparty.IBAN,
		Description:  p.Description,
		Type:         p.Type,
		BalanceAfter: p.BalanceAfter.Value,
	}
}

var txColumns = []output.Column[bunq.Payment]{
	{Header: "DATE", Value: func(p bunq.Payment) string { return p.Created.Local().Format("2006-01-02 15:04") }},
	{Header: "AMOUNT", Value: func(p bunq.Payment) string { return p.Amount.Value }, Right: true},
	{Header: "CUR", Value: func(p bunq.Payment) string { return p.Amount.Currency }},
	{Header: "COUNTERPARTY", Value: func(p bunq.Payment) string { return p.Counterparty.DisplayName }},
	{Header: "DESCRIPTION", Value: func(p bunq.Payment) string { return oneLine(p.Description, 50) }},
	{Header: "ID", Value: func(p bunq.Payment) string { return strconv.Itoa(p.ID) }},
}

// txFilter selects transactions. Zero fields do not filter.
type txFilter struct {
	from, to time.Time   // to is exclusive
	min, max money.Cents // compared with the absolute amount
	search   string      // case-insensitive, in description and counterparty
	limit    int
}

// txFlags binds the filter flags shared by tx list and export.
type txFlags struct {
	from, to, min, max, search string
	limit                      int
}

func (f *txFlags) register(cmd *cobra.Command, withLimit bool) {
	fl := cmd.Flags()
	fl.StringVar(&f.from, "from", "", "first day, YYYY-MM-DD (local time)")
	fl.StringVar(&f.to, "to", "", "last day, inclusive, YYYY-MM-DD (local time)")
	fl.StringVar(&f.min, "min", "", "minimum absolute amount, e.g. 10.00")
	fl.StringVar(&f.max, "max", "", "maximum absolute amount, e.g. 100.00")
	fl.StringVar(&f.search, "search", "", "text in description or counterparty")
	if withLimit {
		fl.IntVar(&f.limit, "limit", 50, "maximum number of transactions, 0 for all")
	}
}

func (f *txFlags) filter() (txFilter, error) {
	var out txFilter
	var err error
	if out.from, err = parseDay(f.from); err != nil {
		return out, err
	}
	if out.to, err = parseDay(f.to); err != nil {
		return out, err
	}
	if !out.to.IsZero() {
		out.to = out.to.AddDate(0, 0, 1)
	}
	if !out.from.IsZero() && !out.to.IsZero() && !out.from.Before(out.to) {
		return out, usageErr("--from must not be after --to")
	}
	if f.min != "" {
		if out.min, err = money.Parse(f.min); err != nil {
			return out, usageErr(err.Error())
		}
	}
	if f.max != "" {
		if out.max, err = money.Parse(f.max); err != nil {
			return out, usageErr(err.Error())
		}
	}
	if f.limit < 0 {
		return out, usageErr("--limit must not be negative")
	}
	out.search, out.limit = strings.ToLower(f.search), f.limit
	return out, nil
}

func parseDay(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.ParseInLocation(time.DateOnly, s, time.Local)
	if err != nil {
		return time.Time{}, usageErr(fmt.Sprintf("date %q must look like 2026-01-31", s))
	}
	return t, nil
}

func (f txFilter) match(p bunq.Payment) (bool, error) {
	if !f.to.IsZero() && !p.Created.Before(f.to) {
		return false, nil
	}
	if f.min != 0 || f.max != 0 {
		amount, err := money.ParseAPI(p.Amount.Value)
		if err != nil {
			return false, fmt.Errorf("payment %d: %w", p.ID, err)
		}
		if a := amount.Abs(); a < f.min || f.max != 0 && a > f.max {
			return false, nil
		}
	}
	if f.search != "" &&
		!strings.Contains(strings.ToLower(p.Description), f.search) &&
		!strings.Contains(strings.ToLower(p.Counterparty.DisplayName), f.search) {
		return false, nil
	}
	return true, nil
}

// payments returns the matching payments of an account, newest first. Paging
// stops at the limit or at the first payment older than f.from.
func payments(ctx context.Context, c *bunq.Client, userID, accountID int, f txFilter) ([]bunq.Payment, error) {
	var out []bunq.Payment
	for p, err := range c.Payments(ctx, userID, accountID) {
		if err != nil {
			return nil, err
		}
		if !f.from.IsZero() && p.Created.Before(f.from) {
			break
		}
		ok, err := f.match(p)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		out = append(out, p)
		if f.limit > 0 && len(out) == f.limit {
			break
		}
	}
	return out, nil
}

func newTxCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{Use: "tx", Aliases: []string{"transactions"}, Short: "Show transactions"}

	var (
		account string
		flags   txFlags
	)
	list := &cobra.Command{
		Use:   "list",
		Short: "List transactions of an account, newest first",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := flags.filter()
			if err != nil {
				return err
			}
			c, userID, err := a.session()
			if err != nil {
				return err
			}
			m, err := a.accountOrPrimary(cmd, c, userID, account)
			if err != nil {
				return err
			}
			txs, err := payments(cmd.Context(), c, userID, m.ID, f)
			if err != nil {
				return err
			}
			return output.List(a.printer(), txs, viewTx, txColumns)
		},
	}
	list.Flags().StringVar(&account, "account", "", "account id or IBAN (default: primary account)")
	flags.register(list, true)

	cmd.AddCommand(list)
	cmd.AddCommand(notesCmds(a)...)
	return cmd
}

// oneLine collapses whitespace and cuts s to n runes for table cells.
func oneLine(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n-3]) + "..."
	}
	return s
}
