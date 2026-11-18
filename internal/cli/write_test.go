package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kisialeu/bunqctl/internal/bunq"
	"github.com/Kisialeu/bunqctl/internal/profile"
)

// testApp returns an app over a temp config dir with an initialized-looking
// sandbox profile, for testing local logic without the API.
func testApp(t *testing.T, maxAmount string) (*app, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errOut bytes.Buffer
	a := newApp("test", strings.NewReader(""), &out, &errOut)
	a.env, a.dir = profile.Sandbox, t.TempDir()
	p := &profile.Profile{Env: profile.Sandbox, PermittedIPs: []string{"203.0.113.1"}, MaxAmount: maxAmount, DeviceID: 1, UserID: 1}
	if err := p.Save(a.dir); err != nil {
		t.Fatal(err)
	}
	return a, &out, &errOut
}

func Test_approve_dryRunPrintsBodyAndSendsNothing(t *testing.T) {
	a, out, _ := testApp(t, "")
	ok, err := a.approve(change{op: "move", body: map[string]string{"amount": "1.00"}, amount: 100}, &writeFlags{money: true, dryRun: true})
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v, want false, nil", ok, err)
	}
	if !strings.Contains(out.String(), `"amount": "1.00"`) {
		t.Errorf("dry run output = %q", out.String())
	}
}

func Test_approve_enforcesMaxAmount(t *testing.T) {
	a, _, _ := testApp(t, "25.00")
	_, err := a.approve(change{op: "move", body: 1, amount: 2501}, &writeFlags{money: true, yes: true})
	if err == nil || !strings.Contains(err.Error(), "max_amount") {
		t.Errorf("err = %v, want max_amount refusal", err)
	}
	if ok, err := a.approve(change{op: "move", body: 1, amount: 2500}, &writeFlags{money: true, yes: true}); err != nil || !ok {
		t.Errorf("at the limit: ok=%v err=%v", ok, err)
	}
}

func Test_approve_refusesRepeatUnlessForced(t *testing.T) {
	a, _, _ := testApp(t, "")
	ch := change{op: "move", body: map[string]string{"amount": "1.00"}, amount: 100}
	wf := &writeFlags{money: true, yes: true}
	if ok, err := a.approve(ch, wf); err != nil || !ok {
		t.Fatalf("first: ok=%v err=%v", ok, err)
	}
	if err := a.recordWrite(ch, wf); err != nil {
		t.Fatal(err)
	}
	if _, err := a.approve(ch, wf); err == nil {
		t.Error("identical write within the window was approved")
	}
	wf.force = true
	if ok, err := a.approve(ch, wf); err != nil || !ok {
		t.Errorf("--force: ok=%v err=%v", ok, err)
	}
}

func Test_approve_needsConfirmationWithoutYes(t *testing.T) {
	a, _, _ := testApp(t, "")
	if _, err := a.approve(change{op: "note", body: 1}, &writeFlags{}); !errors.Is(err, errCancelled) {
		t.Errorf("err = %v, want cancelled (no terminal)", err)
	}
}

func Test_approve_prodMoneyNeedsTypedAmountEvenWithYes(t *testing.T) {
	a, _, _ := testApp(t, "")
	a.env = profile.Production
	p := &profile.Profile{Env: profile.Production, PermittedIPs: []string{"203.0.113.1"}, DeviceID: 1, UserID: 1}
	if err := p.Save(a.dir); err != nil {
		t.Fatal(err)
	}
	_, err := a.approve(change{op: "move", body: 1, amount: 100}, &writeFlags{money: true, yes: true})
	if !errors.Is(err, errCancelled) {
		t.Errorf("err = %v, want cancelled (amount must be typed)", err)
	}
}

func Test_readBatch(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	good := write("good.json", `[
		{"to_iban":"NL32BUNQ2025313705","to_name":"A","amount":"1.10","description":"one"},
		{"to_iban":"nl32 bunq 2025 3137 05","to_name":"B","amount":"2.20","description":"two"}]`)
	entries, total, err := readBatch(good)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || total != 330 || entries[1].Counterparty.Value != "NL32BUNQ2025313705" {
		t.Errorf("entries=%+v total=%d", entries, total)
	}

	for name, body := range map[string]string{
		"empty.json":   `[]`,
		"badiban.json": `[{"to_iban":"NL32BUNQ2025313706","to_name":"A","amount":"1.00","description":"x"}]`,
		"amount.json":  `[{"to_iban":"NL32BUNQ2025313705","to_name":"A","amount":"1","description":"x"}]`,
		"noname.json":  `[{"to_iban":"NL32BUNQ2025313705","amount":"1.00","description":"x"}]`,
		"json.json":    `{`,
	} {
		if _, _, err := readBatch(write(name, body)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func Test_keepSurplus(t *testing.T) {
	acc := bunq.MonetaryAccount{Balance: bunq.Amount{Value: "547.82"}}
	got, desc, err := keepSurplus(acc, "500.00")
	if err != nil || got != 4782 || desc != "bunqctl sweep above 500.00" {
		t.Errorf("got %d %q %v", got, desc, err)
	}
	if got, _, _ := keepSurplus(acc, "600.00"); got >= 0 {
		t.Errorf("below the floor: got %d, want negative (nothing to sweep)", got)
	}
}

func Test_alias(t *testing.T) {
	for _, tt := range []struct {
		to, name, typ string
		wantErr       bool
	}{
		{"a@b.nl", "", "EMAIL", false},
		{"+31612345678", "", "PHONE_NUMBER", false},
		{"NL32 BUNQ 2025 3137 05", "Sugar", "IBAN", false},
		{"NL32BUNQ2025313705", "", "", true},
		{"nonsense", "", "", true},
	} {
		p, err := alias(tt.to, tt.name)
		if (err != nil) != tt.wantErr || err == nil && p.Type != tt.typ {
			t.Errorf("alias(%q) = %+v, %v", tt.to, p, err)
		}
	}
}

func Test_freeCategorySlot(t *testing.T) {
	cats := []bunq.Category{
		{Category: "GENERAL", Status: "ACTIVE"},
		{Category: "USER_DEFINED_1", Status: "ACTIVE"},
		{Category: "USER_DEFINED_2", Status: "INACTIVE"},
	}
	if got, err := freeCategorySlot(cats); err != nil || got != "USER_DEFINED_2" {
		t.Errorf("got %q, %v", got, err)
	}
	var full []bunq.Category
	for n := 1; n <= maxCategorySlots; n++ {
		full = append(full, bunq.Category{Category: fmt.Sprintf("USER_DEFINED_%d", n), Status: "ACTIVE"})
	}
	if _, err := freeCategorySlot(full); err == nil {
		t.Error("full slots: no error")
	}
}

func Test_checkDescription(t *testing.T) {
	long := strings.Repeat("x", maxExternalDescription+1)
	if err := checkDescription(long, true); err == nil {
		t.Error("long external description accepted")
	}
	if err := checkDescription(long, false); err != nil {
		t.Errorf("own account: %v", err)
	}
}

func Test_every(t *testing.T) {
	if got := every(bunq.Schedule{Unit: "MONTHLY", Size: 1}); got != "monthly" {
		t.Errorf("got %q", got)
	}
	if got := every(bunq.Schedule{Unit: "WEEKLY", Size: 2}); got != "2 x weekly" {
		t.Errorf("got %q", got)
	}
}

func Test_scheduleTime_rejectsPast(t *testing.T) {
	if _, err := scheduleTime("2020-01-01"); err == nil {
		t.Error("past date accepted")
	}
}
