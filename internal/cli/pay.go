package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/Kisialeu/bunqctl/internal/bunq"
	"github.com/Kisialeu/bunqctl/internal/iban"
	"github.com/Kisialeu/bunqctl/internal/money"
	"github.com/Kisialeu/bunqctl/internal/output"
)

func newPayCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{Use: "pay", Short: "Draft payments, approved in the bunq app"}
	cmd.AddCommand(
		newPayDraftCmd(a),
		newPayBatchCmd(a),
		newPayStatusCmd(a),
		newIDActionCmd(a, "cancel <draft-id>", "Cancel a pending draft payment", "draft cancel",
			func(cmd *cobra.Command, c *bunq.Client, userID, accountID, id int) error {
				return c.CancelDraftPayment(cmd.Context(), userID, accountID, id)
			}),
	)
	return cmd
}

func newPayDraftCmd(a *app) *cobra.Command {
	var (
		account, amount, description string
		to                           payee
		wf                           *writeFlags
	)
	cmd := &cobra.Command{
		Use:   "draft",
		Short: "Create a draft payment to approve in the bunq app",
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
			ptr, external, err := to.pointer(cmd, c, userID)
			if err != nil {
				return err
			}
			if err := checkDescription(description, external); err != nil {
				return err
			}
			body := bunq.NewDraftPayment{Accepts: 1, Entries: []bunq.DraftEntry{{Amount: eur(cents), Counterparty: ptr, Description: description}}}
			ch := change{op: "draft payment", body: body, amount: cents, summary: [][2]string{
				{"From", accountLabel(m)},
				{"To", ptr.Name + " " + ptr.Value},
				{"Amount", cents.String() + " EUR"},
				{"Description", description},
			}}
			return a.sendDraft(cmd, c, userID, m.ID, ch, body, wf)
		},
	}
	accountFlag(cmd, &account)
	to.register(cmd)
	cmd.Flags().StringVar(&amount, "amount", "", "amount in EUR, e.g. 12.34")
	cmd.Flags().StringVar(&description, "description", "", "payment description (max 140 characters for external IBANs)")
	_ = cmd.MarkFlagRequired("amount")      // only fails for an unknown flag name
	_ = cmd.MarkFlagRequired("description") // only fails for an unknown flag name
	wf = addWriteFlags(cmd, true)
	return cmd
}

// batchEntry is one line of a pay batch file.
type batchEntry struct {
	ToIBAN      string `json:"to_iban"`
	ToName      string `json:"to_name"`
	Amount      string `json:"amount"`
	Description string `json:"description"`
}

func newPayBatchCmd(a *app) *cobra.Command {
	var (
		account string
		wf      *writeFlags
	)
	cmd := &cobra.Command{
		Use:   "batch <file.json>",
		Short: "Create one draft with many payments from a JSON file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, total, err := readBatch(args[0])
			if err != nil {
				return err
			}
			c, userID, m, err := a.withAccount(cmd, account)
			if err != nil {
				return err
			}
			body := bunq.NewDraftPayment{Accepts: 1, Entries: entries}
			ch := change{op: "draft batch", body: body, amount: total, summary: [][2]string{
				{"From", accountLabel(m)},
				{"Payments", strconv.Itoa(len(entries))},
				{"Total", total.String() + " EUR"},
			}}
			return a.sendDraft(cmd, c, userID, m.ID, ch, body, wf)
		},
	}
	accountFlag(cmd, &account)
	wf = addWriteFlags(cmd, true)
	return cmd
}

func readBatch(path string) ([]bunq.DraftEntry, money.Cents, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	var lines []batchEntry
	if err := json.Unmarshal(b, &lines); err != nil {
		return nil, 0, usageErr(fmt.Sprintf("%s: %v", path, err))
	}
	if len(lines) == 0 || len(lines) > bunq.MaxDraftEntries {
		return nil, 0, usageErr(fmt.Sprintf("%s: need 1 to %d entries, got %d", path, bunq.MaxDraftEntries, len(lines)))
	}
	var (
		entries []bunq.DraftEntry
		total   money.Cents
	)
	for i, l := range lines {
		cents, err := money.Parse(l.Amount)
		if err != nil {
			return nil, 0, usageErr(fmt.Sprintf("entry %d: %v", i+1, err))
		}
		n := iban.Normalize(l.ToIBAN)
		if err := iban.Validate(n); err != nil {
			return nil, 0, usageErr(fmt.Sprintf("entry %d: %v", i+1, err))
		}
		if l.ToName == "" || l.Description == "" {
			return nil, 0, usageErr(fmt.Sprintf("entry %d: to_name and description are required", i+1))
		}
		if err := checkDescription(l.Description, true); err != nil {
			return nil, 0, fmt.Errorf("entry %d: %w", i+1, err)
		}
		total += cents
		entries = append(entries, bunq.DraftEntry{
			Amount:       eur(cents),
			Counterparty: bunq.Pointer{Type: "IBAN", Value: n, Name: l.ToName},
			Description:  l.Description,
		})
	}
	return entries, total, nil
}

func (a *app) sendDraft(cmd *cobra.Command, c *bunq.Client, userID, accountID int, ch change, body bunq.NewDraftPayment, wf *writeFlags) error {
	if ok, err := a.approve(ch, wf); err != nil || !ok {
		return err
	}
	id, err := c.CreateDraftPayment(cmd.Context(), userID, accountID, body)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.errOut, "created draft %d: approve it in the bunq app (bunqctl pay status %d)\n", id, id)
	return a.recordWrite(ch, wf)
}

func newPayStatusCmd(a *app) *cobra.Command {
	var account string
	cmd := &cobra.Command{
		Use:   "status <draft-id>",
		Short: "Show the state of a draft payment",
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
			d, err := c.DraftPayment(cmd.Context(), userID, m.ID, id)
			if err != nil {
				return err
			}
			var total money.Cents
			for _, e := range d.Entries {
				v, err := money.ParseAPI(e.Amount.Value)
				if err != nil {
					return err
				}
				total += v.Abs()
			}
			type draftView struct {
				ID      int    `json:"id"`
				Status  string `json:"status"`
				Entries int    `json:"entries"`
				Total   string `json:"total"`
			}
			v := draftView{d.ID, d.Status, len(d.Entries), total.String()}
			pairs := [][2]string{{"ID", strconv.Itoa(v.ID)}, {"Status", v.Status}, {"Entries", strconv.Itoa(v.Entries)}, {"Total", v.Total + " EUR"}}
			if len(d.Entries) == 1 {
				e := d.Entries[0]
				pairs = append(pairs, [2]string{"To", e.Counterparty.DisplayName + " " + e.Counterparty.IBAN}, [2]string{"Description", e.Description})
			}
			return output.Printer{W: a.out, JSON: a.json}.Fields(v, pairs)
		},
	}
	accountFlag(cmd, &account)
	return cmd
}
