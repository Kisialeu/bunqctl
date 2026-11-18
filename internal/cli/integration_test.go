//go:build integration

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Kisialeu/bunqctl/internal/auth"
	"github.com/Kisialeu/bunqctl/internal/profile"
	"github.com/Kisialeu/bunqctl/internal/publicip"
	"github.com/Kisialeu/bunqctl/internal/secret"
)

// Test_Integration_sandbox runs bunqctl end to end against the bunq sandbox:
//
//	go test -tags integration -run Integration -v ./internal/cli/
//
// It creates a new sandbox user with in-memory secrets and a temporary config
// directory, so the developer's own Keychain entries and profile are not
// touched. Each run uses 2 of the roughly 10 setup calls bunq allows per day.
func Test_Integration_sandbox(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("BUNQCTL_ENV", "")
	h := &harness{t: t, store: &secret.Memory{}}

	ip, err := publicip.Lookup(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	h.must("auth", "init", "--permitted-ip", ip)
	if _, err := h.run("auth", "init", "--permitted-ip", ip); err == nil {
		t.Fatal("second auth init succeeded without --force")
	}

	var user struct{ ID int }
	h.decode(&user, "user", "--json")
	if user.ID == 0 {
		t.Fatal("user id is 0")
	}

	var accounts []accountView
	h.decode(&accounts, "accounts", "list", "--json")
	if len(accounts) == 0 || accounts[0].IBAN == "" {
		t.Fatalf("accounts = %+v", accounts)
	}

	h.must("sandbox", "fund", "--amount", "100.00")
	h.waitTx(1)
	h.must("sandbox", "spend", "--amount", "1.23", "--description", "integration coffee")
	h.must("sandbox", "spend", "--amount", "4.56", "--description", "integration lunch")

	txs := h.waitTx(3)
	if txs[0].Amount != "-4.56" || txs[0].Description != "integration lunch" {
		t.Errorf("newest transaction = %+v", txs[0])
	}

	var found []txView
	h.decode(&found, "tx", "list", "--search", "COFFEE", "--json")
	if len(found) != 1 || found[0].Amount != "-1.23" {
		t.Errorf("search = %+v", found)
	}
	h.decode(&found, "tx", "list", "--min", "50.00", "--json")
	if len(found) != 1 || found[0].Amount != "100.00" {
		t.Errorf("min filter = %+v", found)
	}

	today := time.Now().Format(time.DateOnly)
	out := filepath.Join(t.TempDir(), "export.json")
	exportArgs := []string{"export", "--account", "all", "--from", today, "--to", today, "--out", out}
	h.must(exportArgs...)
	var exported []txView
	h.decodeFile(&exported, out)
	if len(exported) != 3 {
		t.Errorf("exported %d transactions, want 3", len(exported))
	}
	if _, err := h.run(exportArgs...); err == nil {
		t.Error("export overwrote an existing file without --force")
	}
	h.must(append(exportArgs, "--force")...)

	// Phase 2: read-only views.
	// bunq indexes insights with a delay, so a brand-new user may have none yet.
	var insights []insightView
	h.decode(&insights, "insights", "--from", today, "--to", today, "--json")
	var events []eventView
	h.decode(&events, "events", "--limit", "3", "--json")
	if len(events) != 3 {
		t.Errorf("events = %d, want 3", len(events))
	}
	h.must("limits")
	h.must("cards", "list")
	h.must("categories", "list")
	h.must("statement", "--format", "csv", "--from", today, "--to", today, "--out", filepath.Join(t.TempDir(), "s.csv"))

	// Drafts: create, read, reject. Never accepted from the CLI.
	h.must("pay", "draft", "--to-iban", sugarDaddyIBAN, "--to-name", "Sugar Daddy",
		"--amount", "2.00", "--description", "integration draft", "--yes")
	draftID := h.lastCreatedID("draft")
	if _, err := h.run("pay", "draft", "--to-iban", sugarDaddyIBAN, "--to-name", "Sugar Daddy",
		"--amount", "2.00", "--description", "integration draft", "--yes"); err == nil {
		t.Error("identical draft within 10 minutes was sent")
	}
	h.must("pay", "cancel", draftID, "--yes")
	var draft struct{ Status string }
	h.decode(&draft, "pay", "status", draftID, "--json")
	if draft.Status != "REJECTED" {
		t.Errorf("draft status = %s, want REJECTED", draft.Status)
	}

	// Requests, links, notes and categories.
	h.must("request", "create", "--to", "friend@example.com", "--amount", "3.00", "--description", "integration request", "--yes")
	var requests []requestView
	h.decode(&requests, "request", "list", "--json")
	h.must("request", "revoke", strconv.Itoa(requests[0].ID), "--yes")
	h.decode(&requests, "request", "list", "--json")
	if requests[0].Status != "REVOKED" {
		t.Errorf("request status = %s, want REVOKED", requests[0].Status)
	}

	var tab bunqMeView
	h.decode(&tab, "bunqme", "create", "--amount", "4.00", "--description", "integration link", "--yes", "--json")
	if tab.URL == "" {
		t.Error("bunq.me link has no URL")
	}
	h.must("bunqme", "cancel", strconv.Itoa(tab.ID), "--yes")

	payment := strconv.Itoa(txs[0].ID)
	h.must("tx", "note", payment, "integration note", "--yes")
	var notes []struct{ Content string }
	h.decode(&notes, "tx", "notes", payment, "--json")
	if len(notes) != 1 || notes[0].Content != "integration note" {
		t.Errorf("notes = %+v", notes)
	}
	h.must("categories", "create", "Integration", "--yes")

	h.must("schedule", "create", "--to-iban", sugarDaddyIBAN, "--to-name", "Sugar Daddy", "--amount", "1.00",
		"--every", "monthly", "--start", time.Now().AddDate(0, 1, 0).Format(time.DateOnly), "--description", "integration schedule", "--yes")
	var schedules []scheduleView
	h.decode(&schedules, "schedule", "list", "--json")
	if len(schedules) != 1 || schedules[0].Every != "monthly" {
		t.Fatalf("schedules = %+v", schedules)
	}
	h.must("schedule", "cancel", strconv.Itoa(schedules[0].ID), "--yes")

	h.must("auth", "session")
	if err := h.store.Set(secret.SessionToken, "invalid"); err != nil {
		t.Fatal(err)
	}
	h.must("user")

	_, err = h.run("--env", "prod", "sandbox", "fund")
	if !errors.Is(err, errUsage) {
		t.Errorf("sandbox fund in prod: err = %v, want usage error", err)
	}
}

// sugarDaddyIBAN is the sandbox counterparty that funds and receives test money.
const sugarDaddyIBAN = "NL32BUNQ2025313705"

// lastCreatedID returns the id from the last "created <what> N" message.
func (h *harness) lastCreatedID(what string) string {
	h.t.Helper()
	m := regexp.MustCompile(`created ` + what + ` (\d+)`).FindStringSubmatch(h.lastErr)
	if m == nil {
		h.t.Fatalf("no %q id in %q", what, h.lastErr)
	}
	return m[1]
}

type harness struct {
	lastErr string
	t       *testing.T
	store   *secret.Memory
}

// run executes one bunqctl command line in-process and returns stdout.
func (h *harness) run(args ...string) (string, error) {
	var out, errOut bytes.Buffer
	a := newApp("test", strings.NewReader(""), &out, &errOut)
	a.store = func(profile.Env) auth.Store { return h.store }
	err := a.run(context.Background(), args)
	h.t.Logf("bunqctl %s -> err=%v\n%s", strings.Join(args, " "), err, errOut.String())
	h.lastErr = errOut.String()
	return out.String(), err
}

func (h *harness) must(args ...string) string {
	h.t.Helper()
	out, err := h.run(args...)
	if err != nil {
		h.t.Fatalf("bunqctl %s: %v", strings.Join(args, " "), err)
	}
	return out
}

func (h *harness) decode(v any, args ...string) {
	h.t.Helper()
	if err := json.Unmarshal([]byte(h.must(args...)), v); err != nil {
		h.t.Fatalf("bunqctl %s: decode: %v", strings.Join(args, " "), err)
	}
}

func (h *harness) decodeFile(v any, path string) {
	h.t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		h.t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		h.t.Fatalf("decode %s: %v", path, err)
	}
}

// waitTx polls until the account has n transactions: the sandbox books
// funding requests asynchronously.
func (h *harness) waitTx(n int) []txView {
	h.t.Helper()
	var txs []txView
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(2 * time.Second) {
		h.decode(&txs, "tx", "list", "--json")
		if len(txs) >= n {
			return txs
		}
	}
	h.t.Fatalf("got %d transactions after 30s, want %d", len(txs), n)
	return nil
}
