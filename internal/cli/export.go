package cli

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/Kisialeu/bunqctl/internal/atomicfile"
	"github.com/Kisialeu/bunqctl/internal/bunq"
)

const allAccounts = "all"

func newExportCmd(a *app) *cobra.Command {
	var (
		account, out string
		force        bool
		flags        txFlags
	)
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export transactions of a period to a JSON file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := flags.filter()
			if err != nil {
				return err
			}
			if err := checkNotExists(out, force); err != nil {
				return err
			}

			c, userID, err := a.session()
			if err != nil {
				return err
			}
			accounts, err := a.exportAccounts(cmd, c, userID, account)
			if err != nil {
				return err
			}

			var txs []bunq.Payment
			for _, m := range accounts {
				p, err := payments(cmd.Context(), c, userID, m.ID, f)
				if err != nil {
					return fmt.Errorf("account %d: %w", m.ID, err)
				}
				txs = append(txs, p...)
			}
			slices.SortStableFunc(txs, func(x, y bunq.Payment) int { return y.Created.Compare(x.Created.Time) })

			views := make([]txView, 0, len(txs))
			for _, p := range txs {
				views = append(views, viewTx(p))
			}
			data, err := json.MarshalIndent(views, "", "  ")
			if err != nil {
				return fmt.Errorf("encode json: %w", err)
			}
			if err := atomicfile.Write(out, append(data, '\n')); err != nil {
				return err
			}
			fmt.Fprintf(a.errOut, "exported %d transactions from %d accounts to %s\n", len(views), len(accounts), out)
			return nil
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&account, "account", "", `account id or IBAN, or "all" (default: primary account)`)
	fl.StringVar(&out, "out", "", "output file (written with mode 0600)")
	fl.BoolVar(&force, "force", false, "overwrite an existing file")
	flags.register(cmd, false)
	for _, name := range []string{"out", "from", "to"} {
		_ = cmd.MarkFlagRequired(name) // only fails for an unknown flag name
	}
	return cmd
}

// exportAccounts resolves --account, where "all" means every account,
// including closed ones that still have history.
func (a *app) exportAccounts(cmd *cobra.Command, c *bunq.Client, userID int, ref string) ([]bunq.MonetaryAccount, error) {
	if ref == allAccounts {
		return collect(c.MonetaryAccounts(cmd.Context(), userID))
	}
	m, err := a.accountOrPrimary(cmd, c, userID, ref)
	if err != nil {
		return nil, err
	}
	return []bunq.MonetaryAccount{m}, nil
}
