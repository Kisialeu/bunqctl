package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Kisialeu/bunqctl/internal/bunq"
	"github.com/Kisialeu/bunqctl/internal/money"
	"github.com/Kisialeu/bunqctl/internal/output"
)

var recurrences = map[string]string{
	"once": bunq.RecurOnce, "daily": bunq.RecurDaily, "weekly": bunq.RecurWeekly,
	"monthly": bunq.RecurMonthly, "yearly": bunq.RecurYearly,
}

type scheduleView struct {
	ID           int    `json:"id"`
	Status       string `json:"status"`
	Amount       string `json:"amount"`
	Counterparty string `json:"counterparty"`
	IBAN         string `json:"counterparty_iban"`
	Description  string `json:"description"`
	Every        string `json:"every"`
	Start        string `json:"start"`
	End          string `json:"end,omitempty"`
}

func viewSchedule(s bunq.ScheduledPayment) scheduleView {
	return scheduleView{
		ID: s.ID, Status: s.Status, Amount: s.Payment.Amount.Value,
		Counterparty: s.Payment.Counterparty.DisplayName, IBAN: s.Payment.Counterparty.IBAN,
		Description: s.Payment.Description, Every: every(s.Schedule), Start: localDay(s.Schedule.Start), End: localDay(s.Schedule.End),
	}
}

// localDay shows a bunq UTC timestamp as a local date; unparsable values are
// shown as they are.
func localDay(s string) string {
	if s == "" {
		return ""
	}
	t, err := bunq.ParseTime(s)
	if err != nil {
		return s
	}
	return t.Local().Format(time.DateOnly)
}

func every(s bunq.Schedule) string {
	unit := strings.ToLower(s.Unit)
	if s.Size <= 1 {
		return unit
	}
	return fmt.Sprintf("%d x %s", s.Size, unit)
}

func newScheduleCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{Use: "schedule", Short: "Scheduled payments (standing orders)"}

	var listAccount string
	list := &cobra.Command{
		Use:   "list",
		Short: "List scheduled payments of an account",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, userID, m, err := a.withAccount(cmd, listAccount)
			if err != nil {
				return err
			}
			items, err := collect(c.ScheduledPayments(cmd.Context(), userID, m.ID))
			if err != nil {
				return err
			}
			return output.List(a.printer(), items, viewSchedule, []output.Column[bunq.ScheduledPayment]{
				{Header: "ID", Value: func(s bunq.ScheduledPayment) string { return strconv.Itoa(s.ID) }},
				{Header: "AMOUNT", Value: func(s bunq.ScheduledPayment) string { return s.Payment.Amount.Value }, Right: true},
				{Header: "TO", Value: func(s bunq.ScheduledPayment) string { return s.Payment.Counterparty.DisplayName }},
				{Header: "EVERY", Value: func(s bunq.ScheduledPayment) string { return every(s.Schedule) }},
				{Header: "START", Value: func(s bunq.ScheduledPayment) string { return localDay(s.Schedule.Start) }},
				{Header: "STATUS", Value: func(s bunq.ScheduledPayment) string { return s.Status }},
				{Header: "DESCRIPTION", Value: func(s bunq.ScheduledPayment) string { return oneLine(s.Payment.Description, 30) }},
			})
		},
	}
	accountFlag(list, &listAccount)

	cmd.AddCommand(list, newScheduleCreateCmd(a), newScheduleCancelCmd(a))
	return cmd
}

func newScheduleCreateCmd(a *app) *cobra.Command {
	var (
		account, amount, description, unit, start, end string
		size                                           int
		to                                             payee
		wf                                             *writeFlags
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a scheduled payment (runs without approval in the app)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cents, err := money.Parse(amount)
			if err != nil {
				return usageErr(err.Error())
			}
			recur, ok := recurrences[unit]
			if !ok {
				return usageErr("--every must be once, daily, weekly, monthly or yearly")
			}
			if size < 1 {
				return usageErr("--interval must be at least 1")
			}
			startAt, err := scheduleTime(start)
			if err != nil {
				return err
			}
			endAt := ""
			if end != "" {
				if endAt, err = scheduleTime(end); err != nil {
					return err
				}
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
			body := bunq.NewScheduledPayment{
				Payment:  bunq.NewPayment{Amount: eur(cents), Counterparty: ptr, Description: description},
				Schedule: bunq.Schedule{Start: startAt, End: endAt, Unit: recur, Size: size},
			}
			ch := change{op: "scheduled payment", body: body, amount: cents, summary: [][2]string{
				{"From", fmt.Sprintf("%d %s", m.ID, m.IBAN())},
				{"To", ptr.Name + " " + ptr.Value},
				{"Amount", cents.String() + " EUR"},
				{"Every", every(body.Schedule)},
				{"Start", startAt},
				{"Description", description},
			}}
			if ok, err := a.approve(ch, wf); err != nil || !ok {
				return err
			}
			id, err := c.CreateScheduledPayment(cmd.Context(), userID, m.ID, body)
			if err != nil {
				return err
			}
			fmt.Fprintf(a.errOut, "created scheduled payment %d\n", id)
			return a.recordWrite(ch, wf)
		},
	}
	f := cmd.Flags()
	accountFlag(cmd, &account)
	to.register(cmd)
	f.StringVar(&amount, "amount", "", "amount in EUR, e.g. 100.00")
	f.StringVar(&description, "description", "", "payment description")
	f.StringVar(&unit, "every", "monthly", "once, daily, weekly, monthly or yearly")
	f.IntVar(&size, "interval", 1, "repeat every N units, e.g. 2 with --every weekly")
	f.StringVar(&start, "start", "", "first run, YYYY-MM-DD (local midnight)")
	f.StringVar(&end, "end", "", "last run, YYYY-MM-DD (default: no end)")
	for _, name := range []string{"amount", "description", "start"} {
		_ = cmd.MarkFlagRequired(name) // only fails for an unknown flag name
	}
	wf = addWriteFlags(cmd, true)
	return cmd
}

// scheduleTime converts a local day to the UTC bunq timestamp of its midnight.
func scheduleTime(day string) (string, error) {
	t, err := parseDay(day)
	if err != nil {
		return "", err
	}
	y, m, d := time.Now().Date()
	if t.Before(time.Date(y, m, d, 0, 0, 0, 0, time.Local)) {
		return "", usageErr("date " + day + " is in the past")
	}
	return t.UTC().Format("2006-01-02 15:04:05"), nil
}

func newScheduleCancelCmd(a *app) *cobra.Command {
	var (
		account string
		wf      *writeFlags
	)
	cmd := &cobra.Command{
		Use:   "cancel <id>",
		Short: "Delete a scheduled payment",
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
			ch := change{op: "schedule cancel", body: map[string]int{"id": id}, summary: [][2]string{{"Scheduled payment", args[0]}}}
			if ok, err := a.approve(ch, wf); err != nil || !ok {
				return err
			}
			if err := c.CancelScheduledPayment(cmd.Context(), userID, m.ID, id); err != nil {
				return err
			}
			fmt.Fprintf(a.errOut, "cancelled scheduled payment %d\n", id)
			return nil
		},
	}
	accountFlag(cmd, &account)
	wf = addWriteFlags(cmd, false)
	return cmd
}
