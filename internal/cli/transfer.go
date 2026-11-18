package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/Kisialeu/bunqctl/internal/bunq"
	"github.com/Kisialeu/bunqctl/internal/money"
)

// ownTransfer is a direct payment between two of the user's accounts. Money
// never leaves the user, which is why it does not need a draft.
type ownTransfer struct {
	op          string
	from, to    bunq.MonetaryAccount
	amount      money.Cents
	description string
}

func (a *app) runOwnTransfer(cmd *cobra.Command, c *bunq.Client, userID int, t ownTransfer, wf *writeFlags) error {
	if t.from.ID == t.to.ID {
		return usageErr("--from and --to are the same account")
	}
	ptr, _, err := a2p(cmd, c, userID, t.to)
	if err != nil {
		return err
	}
	body := bunq.NewPayment{Amount: eur(t.amount), Counterparty: ptr, Description: t.description}
	ch := change{op: t.op, body: body, amount: t.amount, summary: [][2]string{
		{"From", accountLabel(t.from)},
		{"To", accountLabel(t.to)},
		{"Amount", t.amount.String() + " EUR"},
		{"Description", t.description},
	}}
	if ok, err := a.approve(ch, wf); err != nil || !ok {
		return err
	}
	id, err := c.CreatePayment(cmd.Context(), userID, t.from.ID, body)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.errOut, "moved %s EUR from %d to %d (payment %d)\n", t.amount, t.from.ID, t.to.ID, id)
	return a.recordWrite(ch, wf)
}

func accountLabel(m bunq.MonetaryAccount) string {
	return fmt.Sprintf("%d %s (%s)", m.ID, m.IBAN(), m.Description)
}

// ownPair resolves --from (default first active) and --to (required).
func (a *app) ownPair(cmd *cobra.Command, fromRef, toRef string) (*bunq.Client, int, bunq.MonetaryAccount, bunq.MonetaryAccount, error) {
	var none bunq.MonetaryAccount
	c, userID, from, err := a.withAccount(cmd, fromRef)
	if err != nil {
		return nil, 0, none, none, err
	}
	to, err := findAccount(cmd.Context(), c, userID, toRef)
	if err != nil {
		return nil, 0, none, none, err
	}
	return c, userID, from, to, nil
}

func newMoveCmd(a *app) *cobra.Command {
	var (
		from, to, amount, description string
		wf                            *writeFlags
	)
	cmd := &cobra.Command{
		Use:   "move",
		Short: "Move money between your own accounts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cents, err := money.Parse(amount)
			if err != nil {
				return usageErr(err.Error())
			}
			c, userID, src, dst, err := a.ownPair(cmd, from, to)
			if err != nil {
				return err
			}
			return a.runOwnTransfer(cmd, c, userID, ownTransfer{op: "move", from: src, to: dst, amount: cents, description: description}, wf)
		},
	}
	f := cmd.Flags()
	f.StringVar(&from, "from", "", "source account id or IBAN (default: primary account)")
	f.StringVar(&to, "to", "", "target account id or IBAN (must be your own)")
	f.StringVar(&amount, "amount", "", "amount in EUR, e.g. 50.00")
	f.StringVar(&description, "description", "bunqctl move", "payment description")
	_ = cmd.MarkFlagRequired("to")     // only fails for an unknown flag name
	_ = cmd.MarkFlagRequired("amount") // only fails for an unknown flag name
	wf = addWriteFlags(cmd, true)
	return cmd
}

func newSweepCmd(a *app) *cobra.Command {
	var (
		from, to, keep, since, until string
		percent                      int
		wf                           *writeFlags
	)
	cmd := &cobra.Command{
		Use:   "sweep",
		Short: "Move a balance surplus or a share of income to another own account",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if (keep == "") == (percent == 0) {
				return usageErr("use exactly one of --keep or --percent")
			}
			c, userID, src, dst, err := a.ownPair(cmd, from, to)
			if err != nil {
				return err
			}

			var cents money.Cents
			var description string
			if keep != "" {
				cents, description, err = keepSurplus(src, keep)
			} else {
				cents, description, err = a.incomeShare(cmd, c, userID, src, percent, since, until)
			}
			if err != nil || cents <= 0 {
				if err == nil {
					fmt.Fprintln(a.errOut, "nothing to sweep")
				}
				return err
			}
			return a.runOwnTransfer(cmd, c, userID, ownTransfer{op: "sweep", from: src, to: dst, amount: cents, description: description}, wf)
		},
	}
	f := cmd.Flags()
	f.StringVar(&from, "from", "", "source account id or IBAN (default: primary account)")
	f.StringVar(&to, "to", "", "target account id or IBAN (must be your own)")
	f.StringVar(&keep, "keep", "", "keep this balance, move the rest, e.g. 500.00")
	f.IntVar(&percent, "percent", 0, "percentage of income to move (1-100)")
	f.StringVar(&since, "since", "", "income period start, YYYY-MM-DD (with --percent)")
	f.StringVar(&until, "until", "", "income period end, inclusive, YYYY-MM-DD (with --percent)")
	_ = cmd.MarkFlagRequired("to") // only fails for an unknown flag name
	cmd.MarkFlagsRequiredTogether("percent", "since", "until")
	wf = addWriteFlags(cmd, true)
	return cmd
}

func keepSurplus(src bunq.MonetaryAccount, keep string) (money.Cents, string, error) {
	floor, err := money.Parse(keep)
	if err != nil {
		return 0, "", usageErr(err.Error())
	}
	balance, err := money.ParseAPI(src.Balance.Value)
	if err != nil {
		return 0, "", err
	}
	return balance - floor, "bunqctl sweep above " + floor.String(), nil
}

// incomeShare returns percent of the money received on src from outside
// the user's own accounts in the period. It returns 0 when the period has
// already been swept.
func (a *app) incomeShare(cmd *cobra.Command, c *bunq.Client, userID int, src bunq.MonetaryAccount, percent int, since, until string) (money.Cents, string, error) {
	if percent < 1 || percent > 100 {
		return 0, "", usageErr("--percent must be between 1 and 100")
	}
	f, err := (&txFlags{from: since, to: until}).filter()
	if err != nil {
		return 0, "", err
	}
	marker := fmt.Sprintf("bunqctl sweep %d%% %s..%s", percent, since, until)

	own := map[string]bool{}
	for m, err := range c.MonetaryAccounts(cmd.Context(), userID) {
		if err != nil {
			return 0, "", err
		}
		own[m.IBAN()] = true
	}

	// Look at everything since the period start: a sweep payment made after
	// the period still marks it as done.
	txs, err := payments(cmd.Context(), c, userID, src.ID, txFilter{from: f.from})
	if err != nil {
		return 0, "", err
	}
	var income money.Cents
	for _, p := range txs {
		if p.Description == marker {
			fmt.Fprintf(a.errOut, "period already swept by payment %s\n", strconv.Itoa(p.ID))
			return 0, marker, nil
		}
		amount, err := money.ParseAPI(p.Amount.Value)
		if err != nil {
			return 0, "", err
		}
		if amount > 0 && p.Created.Before(f.to) && !own[p.Counterparty.IBAN] {
			income += amount
		}
	}
	return income * money.Cents(percent) / 100, marker, nil
}
