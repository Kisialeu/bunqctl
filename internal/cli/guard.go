package cli

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/Kisialeu/bunqctl/internal/journal"
	"github.com/Kisialeu/bunqctl/internal/money"
	"github.com/Kisialeu/bunqctl/internal/output"
	"github.com/Kisialeu/bunqctl/internal/profile"
)

// repeatWindow is how long an identical money write is refused.
const repeatWindow = 10 * time.Minute

// writeFlags are the safety flags of a command that changes something.
type writeFlags struct {
	money  bool // the command moves money
	dryRun bool
	yes    bool
	force  bool
}

func addWriteFlags(cmd *cobra.Command, moves bool) *writeFlags {
	w := &writeFlags{money: moves}
	f := cmd.Flags()
	f.BoolVar(&w.dryRun, "dry-run", false, "print the request body and send nothing")
	f.BoolVarP(&w.yes, "yes", "y", false, "skip the confirmation (prod money writes still ask for the amount)")
	if moves {
		f.BoolVar(&w.force, "force", false, "allow an identical payment again within 10 minutes")
	}
	return w
}

// change describes one write for the guard.
type change struct {
	op      string      // e.g. "pay draft"; part of the repeat fingerprint
	summary [][2]string // shown before sending
	body    any         // request body: printed by --dry-run, fingerprinted for money writes
	amount  money.Cents // total moved, money writes only
}

// approve runs every safety check before a write. It returns false when
// nothing must be sent (dry run) and an error when the write is refused.
func (a *app) approve(ch change, w *writeFlags) (bool, error) {
	summary := append([][2]string{{"Environment", string(a.env)}}, ch.summary...)
	if err := (output.Printer{W: a.errOut}).Fields(nil, summary); err != nil {
		return false, err
	}
	if w.dryRun {
		fmt.Fprintln(a.errOut, "dry run: nothing sent; request body:")
		return false, output.Printer{W: a.out}.Value(ch.body)
	}

	if w.money {
		if err := a.checkMaxAmount(ch.amount); err != nil {
			return false, err
		}
		key, err := journal.Key(ch.op, ch.body)
		if err != nil {
			return false, err
		}
		if at, seen, err := a.journal().Seen(key, repeatWindow); err != nil {
			return false, err
		} else if seen && !w.force {
			return false, fmt.Errorf("an identical %s was sent at %s: use --force to send it again", ch.op, at.Local().Format(time.TimeOnly))
		}
	}

	a.warnProduction()
	switch {
	case w.money && a.env == profile.Production:
		return true, a.confirm("Send "+ch.op+"?", ch.amount.String())
	case !w.yes:
		return true, a.confirm("Send "+ch.op+"?", "")
	}
	return true, nil
}

// recordWrite remembers a successful money write for the repeat check.
func (a *app) recordWrite(ch change, w *writeFlags) error {
	if !w.money {
		return nil
	}
	key, err := journal.Key(ch.op, ch.body)
	if err != nil {
		return err
	}
	return a.journal().Add(key)
}

func (a *app) journal() *journal.Journal {
	return journal.New(filepath.Join(a.dir, string(a.env)+"-journal.json"))
}

func (a *app) checkMaxAmount(amount money.Cents) error {
	m, err := a.manager()
	if err != nil {
		return err
	}
	limit := m.Profile().MaxAmount
	if limit == "" {
		return nil
	}
	max, err := money.Parse(limit)
	if err != nil {
		return fmt.Errorf("profile max_amount: %w", err)
	}
	if amount > max {
		return fmt.Errorf("amount %s is above max_amount %s (bunqctl config set max-amount)", amount, max)
	}
	return nil
}
