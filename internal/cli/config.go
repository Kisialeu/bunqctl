package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Kisialeu/bunqctl/internal/money"
)

func newConfigCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Local settings of the environment"}

	show := &cobra.Command{
		Use:   "show",
		Short: "Show local settings",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			m, err := a.manager()
			if err != nil {
				return err
			}
			p := m.Profile()
			view := struct {
				Env          string   `json:"env"`
				PermittedIPs []string `json:"permitted_ips"`
				Region       string   `json:"region"`
				Country      string   `json:"country"`
				MaxAmount    string   `json:"max_amount"`
			}{string(p.Env), p.PermittedIPs, p.Region, p.Country, p.MaxAmount}
			return a.printer().Fields(view, [][2]string{
				{"Environment", view.Env},
				{"Permitted IPs", strings.Join(view.PermittedIPs, ", ")},
				{"Region", orDefault(view.Region, "en_US")},
				{"Country", orDefault(view.Country, "NL")},
				{"Max amount", orDefault(view.MaxAmount, "none")},
			})
		},
	}

	set := &cobra.Command{
		Use:   "set max-amount <amount|none>",
		Short: "Refuse money writes above an amount",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			if args[0] != "max-amount" {
				return usageErr("only max-amount can be set")
			}
			value := args[1]
			if value == "none" {
				value = ""
			} else if _, err := money.Parse(value); err != nil {
				return usageErr(err.Error())
			}
			m, err := a.manager()
			if err != nil {
				return err
			}
			p := m.Profile()
			p.MaxAmount = value
			if err := p.Save(a.dir); err != nil {
				return err
			}
			fmt.Fprintf(a.errOut, "max-amount for %s: %s\n", a.env, orDefault(value, "none"))
			return nil
		},
	}

	cmd.AddCommand(show, set)
	return cmd
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
