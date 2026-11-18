package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Kisialeu/bunqctl/internal/bunq"
	"github.com/Kisialeu/bunqctl/internal/iban"
	"github.com/Kisialeu/bunqctl/internal/money"
	"github.com/Kisialeu/bunqctl/internal/output"
)

type requestView struct {
	ID           int       `json:"id"`
	Date         time.Time `json:"date"`
	Status       string    `json:"status"`
	Amount       string    `json:"amount"`
	Counterparty string    `json:"counterparty"`
	Description  string    `json:"description"`
}

func newRequestCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{Use: "request", Short: "Ask someone for money"}

	var (
		account, to, name, amount, description string
		wf                                     *writeFlags
	)
	create := &cobra.Command{
		Use:   "create",
		Short: "Send a payment request",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cents, err := money.Parse(amount)
			if err != nil {
				return usageErr(err.Error())
			}
			ptr, err := alias(to, name)
			if err != nil {
				return err
			}
			c, userID, m, err := a.withAccount(cmd, account)
			if err != nil {
				return err
			}
			body := bunq.NewRequestInquiry{Amount: eur(cents), From: ptr, Description: description, AllowBunqMe: true}
			ch := change{op: "request", body: body, summary: [][2]string{
				{"Into", accountLabel(m)}, {"From", to}, {"Amount", cents.String() + " EUR"}, {"Description", description},
			}}
			if ok, err := a.approve(ch, wf); err != nil || !ok {
				return err
			}
			id, err := c.CreateRequestInquiry(cmd.Context(), userID, m.ID, body)
			if err != nil {
				return err
			}
			fmt.Fprintf(a.errOut, "requested %s EUR from %s (request %d)\n", cents, to, id)
			return nil
		},
	}
	accountFlag(create, &account)
	create.Flags().StringVar(&to, "to", "", "email, phone number (+31...) or IBAN of the payer")
	create.Flags().StringVar(&name, "name", "", "payer name (required for an IBAN)")
	create.Flags().StringVar(&amount, "amount", "", "amount in EUR")
	create.Flags().StringVar(&description, "description", "", "what the request is for")
	for _, n := range []string{"to", "amount", "description"} {
		_ = create.MarkFlagRequired(n) // only fails for an unknown flag name
	}
	wf = addWriteFlags(create, false)

	var listAccount string
	list := &cobra.Command{
		Use:   "list",
		Short: "List sent payment requests",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, userID, m, err := a.withAccount(cmd, listAccount)
			if err != nil {
				return err
			}
			items, err := collect(c.RequestInquiries(cmd.Context(), userID, m.ID))
			if err != nil {
				return err
			}
			return output.List(a.printer(), items,
				func(r bunq.RequestInquiry) requestView {
					return requestView{r.ID, r.Created.Time, r.Status, r.Amount.Value, r.Counterparty.DisplayName, r.Description}
				},
				[]output.Column[bunq.RequestInquiry]{
					{Header: "DATE", Value: func(r bunq.RequestInquiry) string { return r.Created.Local().Format("2006-01-02 15:04") }},
					{Header: "AMOUNT", Value: func(r bunq.RequestInquiry) string { return r.Amount.Value }, Right: true},
					{Header: "FROM", Value: func(r bunq.RequestInquiry) string { return r.Counterparty.DisplayName }},
					{Header: "STATUS", Value: func(r bunq.RequestInquiry) string { return r.Status }},
					{Header: "DESCRIPTION", Value: func(r bunq.RequestInquiry) string { return oneLine(r.Description, 40) }},
					{Header: "ID", Value: func(r bunq.RequestInquiry) string { return strconv.Itoa(r.ID) }},
				})
		},
	}
	accountFlag(list, &listAccount)

	revoke := newIDActionCmd(a, "revoke <id>", "Withdraw a pending payment request", "request revoke",
		func(cmd *cobra.Command, c *bunq.Client, userID, accountID, id int) error {
			return c.RevokeRequestInquiry(cmd.Context(), userID, accountID, id)
		})

	cmd.AddCommand(create, list, revoke)
	return cmd
}

// alias turns an email, phone number or IBAN into a counterparty pointer.
func alias(to, name string) (bunq.Pointer, error) {
	switch {
	case strings.Contains(to, "@"):
		return bunq.Pointer{Type: "EMAIL", Value: to, Name: name}, nil
	case strings.HasPrefix(to, "+"):
		return bunq.Pointer{Type: "PHONE_NUMBER", Value: to, Name: name}, nil
	}
	n := iban.Normalize(to)
	if err := iban.Validate(n); err != nil {
		return bunq.Pointer{}, usageErr("--to is not an email, +phone or valid IBAN: " + err.Error())
	}
	if name == "" {
		return bunq.Pointer{}, usageErr("--name is required for an IBAN")
	}
	return bunq.Pointer{Type: "IBAN", Value: n, Name: name}, nil
}

// newIDActionCmd builds a guarded "<verb> <id> [--account A]" command.
func newIDActionCmd(a *app, use, short, op string, act func(cmd *cobra.Command, c *bunq.Client, userID, accountID, id int) error) *cobra.Command {
	var (
		account string
		wf      *writeFlags
	)
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			c, userID, m, err := a.withAccount(cmd, account)
			if err != nil {
				return err
			}
			ch := change{op: op, body: map[string]int{"id": id}, summary: [][2]string{{"Action", op}, {"ID", args[0]}, {"Account", accountLabel(m)}}}
			if ok, err := a.approve(ch, wf); err != nil || !ok {
				return err
			}
			if err := act(cmd, c, userID, m.ID, id); err != nil {
				return err
			}
			fmt.Fprintf(a.errOut, "%s %d: done\n", op, id)
			return nil
		},
	}
	accountFlag(cmd, &account)
	wf = addWriteFlags(cmd, false)
	return cmd
}
