package cli

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Kisialeu/bunqctl/internal/auth"
	"github.com/Kisialeu/bunqctl/internal/bunq"
	"github.com/Kisialeu/bunqctl/internal/secret"
)

func payment(day string, amount, desc, counterparty string) bunq.Payment {
	t, err := time.ParseInLocation(time.DateOnly, day, time.Local)
	if err != nil {
		panic(err)
	}
	return bunq.Payment{
		Created:      bunq.Time{Time: t.Add(12 * time.Hour)},
		Amount:       bunq.Amount{Value: amount, Currency: "EUR"},
		Description:  desc,
		Counterparty: bunq.LabelMonetaryAccount{DisplayName: counterparty},
	}
}

func Test_txFilter_match(t *testing.T) {
	coffee := payment("2026-11-18", "-7.50", "Coffee and cake", "Cafe")
	salary := payment("2026-11-01", "1000.00", "November", "Employer")

	tests := []struct {
		name  string
		flags txFlags
		p     bunq.Payment
		want  bool
	}{
		{"no filter", txFlags{}, coffee, true},
		{"to is inclusive", txFlags{to: "2026-11-18"}, coffee, true},
		{"after to", txFlags{to: "2026-11-17"}, coffee, false},
		{"min uses absolute amount", txFlags{min: "5.00"}, coffee, true},
		{"below min", txFlags{min: "10.00"}, coffee, false},
		{"above max", txFlags{max: "100.00"}, salary, false},
		{"search description", txFlags{search: "CAKE"}, coffee, true},
		{"search counterparty", txFlags{search: "employer"}, salary, true},
		{"search miss", txFlags{search: "rent"}, salary, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := tt.flags.filter()
			if err != nil {
				t.Fatal(err)
			}
			got, err := f.match(tt.p)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("match = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_txFlags_filter_rejects(t *testing.T) {
	for _, tt := range []struct {
		name  string
		flags txFlags
	}{
		{"bad date", txFlags{from: "18.11.2026"}},
		{"from after to", txFlags{from: "2026-11-19", to: "2026-11-18"}},
		{"bad amount", txFlags{min: "7.5"}},
		{"negative limit", txFlags{limit: -1}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.flags.filter(); !errors.Is(err, errUsage) {
				t.Errorf("err = %v, want usage error", err)
			}
		})
	}
}

func Test_exitCode(t *testing.T) {
	for _, tt := range []struct {
		err  error
		want int
	}{
		{nil, exitOK},
		{usageErr("x"), exitUsage},
		{fmt.Errorf("wrap: %w", auth.ErrNotInitialized), exitAuth},
		{fmt.Errorf("get: %w", secret.ErrNotFound), exitAuth},
		{errCancelled, exitCancelled},
		{context.Canceled, exitCancelled},
		{errors.New("boom"), exitError},
	} {
		if got := exitCode(tt.err); got != tt.want {
			t.Errorf("exitCode(%v) = %d, want %d", tt.err, got, tt.want)
		}
	}
}

func Test_oneLine(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"short", "short"},
		{"two\nlines   x", "two lin..."},
		{"a\nb   c", "a b c"},
		{"abcdefghijkl", "abcdefg..."},
	} {
		if got := oneLine(tt.in, 10); got != tt.want {
			t.Errorf("oneLine(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
