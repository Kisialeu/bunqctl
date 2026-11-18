package cli

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/Kisialeu/bunqctl/internal/bunq"
	"github.com/Kisialeu/bunqctl/internal/money"
	"github.com/Kisialeu/bunqctl/internal/output"
)

type bunqMeView struct {
	ID          int       `json:"id"`
	Date        time.Time `json:"date"`
	Status      string    `json:"status"`
	Amount      string    `json:"amount"`
	Description string    `json:"description"`
	URL         string    `json:"url"`
}

func viewBunqMe(t bunq.BunqMeTab) bunqMeView {
	return bunqMeView{t.ID, t.Created.Time, t.Status, t.Entry.Amount.Value, t.Entry.Description, t.ShareURL}
}

func newBunqMeCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{Use: "bunqme", Short: "Shareable bunq.me payment links"}

	var (
		account, amount, description string
		wf                           *writeFlags
	)
	create := &cobra.Command{
		Use:   "create",
		Short: "Create a payment link and print its URL",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cents, err := money.Parse(amount)
			if err != nil {
				return usageErr(err.Error())
			}
			c, userID, m, err := a.withAccount(cmd, account)
			if err != nil {
				return err
			}
			body := map[string]any{"amount": eur(cents), "description": description}
			ch := change{op: "bunqme", body: body, summary: [][2]string{
				{"Into", accountLabel(m)}, {"Amount", cents.String() + " EUR"}, {"Description", description},
			}}
			if ok, err := a.approve(ch, wf); err != nil || !ok {
				return err
			}
			id, err := c.CreateBunqMeTab(cmd.Context(), userID, m.ID, eur(cents), description)
			if err != nil {
				return err
			}
			tab, err := c.BunqMeTab(cmd.Context(), userID, m.ID, id)
			if err != nil {
				return err
			}
			if a.json {
				return a.printer().Value(viewBunqMe(tab))
			}
			fmt.Fprintln(a.out, tab.ShareURL)
			return nil
		},
	}
	accountFlag(create, &account)
	create.Flags().StringVar(&amount, "amount", "", "amount in EUR")
	create.Flags().StringVar(&description, "description", "", "what the link is for")
	_ = create.MarkFlagRequired("amount")      // only fails for an unknown flag name
	_ = create.MarkFlagRequired("description") // only fails for an unknown flag name
	wf = addWriteFlags(create, false)

	var listAccount string
	list := &cobra.Command{
		Use:   "list",
		Short: "List payment links",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, userID, m, err := a.withAccount(cmd, listAccount)
			if err != nil {
				return err
			}
			tabs, err := collect(c.BunqMeTabs(cmd.Context(), userID, m.ID))
			if err != nil {
				return err
			}
			return output.List(a.printer(), tabs, viewBunqMe, []output.Column[bunq.BunqMeTab]{
				{Header: "ID", Value: func(t bunq.BunqMeTab) string { return strconv.Itoa(t.ID) }},
				{Header: "AMOUNT", Value: func(t bunq.BunqMeTab) string { return t.Entry.Amount.Value }, Right: true},
				{Header: "STATUS", Value: func(t bunq.BunqMeTab) string { return t.Status }},
				{Header: "DESCRIPTION", Value: func(t bunq.BunqMeTab) string { return oneLine(t.Entry.Description, 30) }},
				{Header: "URL", Value: func(t bunq.BunqMeTab) string { return t.ShareURL }},
			})
		},
	}
	accountFlag(list, &listAccount)

	cancel := newIDActionCmd(a, "cancel <id>", "Close a payment link", "bunqme cancel",
		func(cmd *cobra.Command, c *bunq.Client, userID, accountID, id int) error {
			return c.CancelBunqMeTab(cmd.Context(), userID, accountID, id)
		})

	cmd.AddCommand(create, list, cancel)
	return cmd
}
