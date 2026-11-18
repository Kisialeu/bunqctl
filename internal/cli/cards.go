package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/Kisialeu/bunqctl/internal/bunq"
	"github.com/Kisialeu/bunqctl/internal/output"
)

type cardView struct {
	ID          int    `json:"id"`
	Type        string `json:"type"`
	Product     string `json:"product_type"`
	Status      string `json:"status"`
	OrderStatus string `json:"order_status"`
	LastDigits  string `json:"last_digits"`
	Name        string `json:"name_on_card"`
	Expiry      string `json:"expiry_date"`
	IBAN        string `json:"account_iban"`
}

func viewCard(c bunq.Card) cardView {
	return cardView{c.ID, c.Type, c.ProductType, c.Status, c.OrderStatus, c.LastDigits(), c.NameOnCard, c.ExpiryDate, c.Account.IBAN}
}

func newCardsCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{Use: "cards", Aliases: []string{"card"}, Short: "Payment cards"}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List cards",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				c, userID, err := a.session()
				if err != nil {
					return err
				}
				cards, err := collect(c.Cards(cmd.Context(), userID))
				if err != nil {
					return err
				}
				return output.List(a.printer(), cards, viewCard, []output.Column[bunq.Card]{
					{Header: "ID", Value: func(c bunq.Card) string { return strconv.Itoa(c.ID) }},
					{Header: "TYPE", Value: func(c bunq.Card) string { return c.Type }},
					{Header: "LAST4", Value: bunq.Card.LastDigits},
					{Header: "NAME", Value: func(c bunq.Card) string { return c.NameOnCard }},
					{Header: "EXPIRY", Value: func(c bunq.Card) string { return c.ExpiryDate }},
					{Header: "STATUS", Value: func(c bunq.Card) string { return c.Status }},
					{Header: "ACCOUNT", Value: func(c bunq.Card) string { return c.Account.IBAN }},
				})
			},
		},
		newCardStatusCmd(a, "freeze", "Temporarily block a card (reversible)", bunq.CardDeactivated),
		newCardStatusCmd(a, "unfreeze", "Unblock a frozen card", bunq.CardActive),
	)
	return cmd
}

// newCardStatusCmd builds freeze and unfreeze. Permanent statuses (LOST,
// STOLEN, CANCELLED) are intentionally not available.
func newCardStatusCmd(a *app, use, short, status string) *cobra.Command {
	var wf *writeFlags
	cmd := &cobra.Command{
		Use:   use + " <card-id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			c, userID, err := a.session()
			if err != nil {
				return err
			}
			body := map[string]string{"status": status}
			ch := change{op: "card " + use, body: body, summary: [][2]string{{"Card", args[0]}, {"New status", status}}}
			if ok, err := a.approve(ch, wf); err != nil || !ok {
				return err
			}
			if err := c.SetCardStatus(cmd.Context(), userID, id, status); err != nil {
				return err
			}
			// bunq accepts the change for cards that cannot take it yet
			// (e.g. not activated), so report what the card really is.
			card, err := c.Card(cmd.Context(), userID, id)
			if err != nil {
				return err
			}
			if card.Status != status {
				return fmt.Errorf("card %d is still %s (order status %s): activate it in the bunq app first", id, card.Status, card.OrderStatus)
			}
			fmt.Fprintf(a.errOut, "card %d is now %s\n", id, card.Status)
			return nil
		},
	}
	wf = addWriteFlags(cmd, false)
	return cmd
}
