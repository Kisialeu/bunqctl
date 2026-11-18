package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Kisialeu/bunqctl/internal/bunq"
	"github.com/Kisialeu/bunqctl/internal/iban"
	"github.com/Kisialeu/bunqctl/internal/money"
	"github.com/Kisialeu/bunqctl/internal/profile"
)

const (
	sugarDaddy    = "sugardaddy@bunq.com"
	maxSandboxFee = money.Cents(500_00)
)

func newSandboxCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sandbox",
		Short: "Sandbox-only helpers",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.setup(); err != nil {
				return err
			}
			return a.requireEnv(profile.Sandbox)
		},
	}

	var amount, account string
	fund := &cobra.Command{
		Use:   "fund",
		Short: "Request up to 500.00 EUR of sandbox money from " + sugarDaddy,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cents, err := money.Parse(amount)
			if err != nil {
				return usageErr(err.Error())
			}
			if cents > maxSandboxFee {
				return usageErr("sandbox funding is limited to " + maxSandboxFee.String())
			}
			c, userID, err := a.session()
			if err != nil {
				return err
			}
			m, err := a.accountOrPrimary(cmd, c, userID, account)
			if err != nil {
				return err
			}
			id, err := c.CreateRequestInquiry(cmd.Context(), userID, m.ID, bunq.NewRequestInquiry{
				Amount:      bunq.Amount{Value: cents.String(), Currency: "EUR"},
				From:        bunq.Pointer{Type: "EMAIL", Value: sugarDaddy, Name: "Sugar Daddy"},
				Description: "bunqctl sandbox funding",
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(a.errOut, "requested %s EUR into account %d (request %d)\n", cents, m.ID, id)
			return nil
		},
	}
	fund.Flags().StringVar(&amount, "amount", maxSandboxFee.String(), "amount in EUR, e.g. 100.00")
	fund.Flags().StringVar(&account, "account", "", "account id or IBAN (default: primary account)")

	cmd.AddCommand(fund, newSandboxSpendCmd(a))
	return cmd
}

// newSandboxSpendCmd makes a direct payment to create test transactions.
// Direct payments exist only in the sandbox; production uses drafts.
func newSandboxSpendCmd(a *app) *cobra.Command {
	var amount, to, name, description, account string
	cmd := &cobra.Command{
		Use:   "spend",
		Short: "Make a direct test payment (sandbox only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cents, err := money.Parse(amount)
			if err != nil {
				return usageErr(err.Error())
			}
			counterparty := bunq.Pointer{Type: "EMAIL", Value: to, Name: name}
			if !strings.Contains(to, "@") {
				counterparty = bunq.Pointer{Type: "IBAN", Value: iban.Normalize(to), Name: name}
				if name == "" {
					return usageErr("--name is required for an IBAN")
				}
			}
			c, userID, err := a.session()
			if err != nil {
				return err
			}
			m, err := a.accountOrPrimary(cmd, c, userID, account)
			if err != nil {
				return err
			}
			id, err := c.CreatePayment(cmd.Context(), userID, m.ID, bunq.NewPayment{
				Amount:       bunq.Amount{Value: cents.String(), Currency: "EUR"},
				Counterparty: counterparty,
				Description:  description,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(a.errOut, "paid %s EUR from account %d to %s (payment %d)\n", cents, m.ID, to, id)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&amount, "amount", "", "amount in EUR, e.g. 12.34")
	f.StringVar(&to, "to", sugarDaddy, "recipient email or IBAN")
	f.StringVar(&name, "name", "", "recipient name (required for an IBAN)")
	f.StringVar(&description, "description", "bunqctl test payment", "payment description")
	f.StringVar(&account, "account", "", "account id or IBAN (default: primary account)")
	_ = cmd.MarkFlagRequired("amount") // only fails for an unknown flag name
	return cmd
}
