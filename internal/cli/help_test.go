package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func newTestRoot() *cobra.Command {
	return newRootCmd(newApp("test", strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}))
}

// commands returns every command except the root, keyed by help path.
func commands(root *cobra.Command) map[string]*cobra.Command {
	out := map[string]*cobra.Command{}
	walkCommands(root, func(c *cobra.Command) {
		if c != root {
			out[helpKey(c)] = c
		}
	})
	return out
}

func isLeaf(c *cobra.Command) bool { return len(c.Commands()) == 0 }

func Test_help_everyCommandIsDocumented(t *testing.T) {
	root := newTestRoot()
	for path, c := range commands(root) {
		if c.Short == "" {
			t.Errorf("%s: no Short", path)
		}
		if c.Long == "" {
			t.Errorf("%s: no Long: add an entry to helpTable", path)
		}
		if isLeaf(c) && c.Example == "" {
			t.Errorf("%s: no Example", path)
		}
		if !strings.Contains(path, " ") && c.GroupID == "" {
			t.Errorf("%s: top-level command is in no group (groupOf)", path)
		}
	}
}

func Test_help_noStaleEntries(t *testing.T) {
	cmds := commands(newTestRoot())
	for key := range helpTable {
		if _, ok := cmds[key]; !ok {
			t.Errorf("helpTable has %q but there is no such command", key)
		}
	}
	for name := range groupOf {
		if _, ok := cmds[name]; !ok {
			t.Errorf("groupOf has %q but there is no such command", name)
		}
	}
}

func Test_help_everyFlagHasUsage(t *testing.T) {
	root := newTestRoot()
	check := func(where string, fs *pflag.FlagSet) {
		fs.VisitAll(func(f *pflag.Flag) {
			if strings.TrimSpace(f.Usage) == "" {
				t.Errorf("%s: flag --%s has no description", where, f.Name)
			}
		})
	}
	check("root", root.PersistentFlags())
	for path, c := range commands(root) {
		check(path, c.LocalFlags())
	}
}

func Test_help_writesExplainTheirSafety(t *testing.T) {
	for path, c := range commands(newTestRoot()) {
		if !isLeaf(c) || c.Flags().Lookup("dry-run") == nil {
			continue
		}
		if !strings.Contains(c.Long, "Safety:") {
			t.Errorf("%s: a write command needs a Safety paragraph", path)
		}
		if c.Flags().Lookup("force") != nil && !strings.Contains(c.Long, "type the amount") {
			t.Errorf("%s: a money command must say that prod asks for the amount", path)
		}
	}
}

func Test_help_rootGuide(t *testing.T) {
	root := newTestRoot()
	for _, want := range []string{"Environments", "Safety", "Exit codes", "bunqctl docs", "--dry-run", "sandbox", "prod"} {
		if !strings.Contains(root.Long, want) {
			t.Errorf("root help does not mention %q", want)
		}
	}
	// Every exit code is documented once, in the root help.
	for _, code := range []string{"0 ", "1 ", "2 ", "3 ", "4 "} {
		if !strings.Contains(root.Long, code) {
			t.Errorf("root help does not explain exit code %q", code)
		}
	}
}

// Test_help_examplesAreValidCommands runs every example line through cobra's
// own parsing: the command must exist, the flags must exist and satisfy
// required, exclusive and one-required groups, and the arguments must fit.
func Test_help_examplesAreValidCommands(t *testing.T) {
	checked := 0
	for path, c := range commands(newTestRoot()) {
		for _, line := range strings.Split(c.Example, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			checked++
			args, ok := exampleArgs(line)
			if !ok {
				t.Errorf("%s: example %q does not start with bunqctl", path, line)
				continue
			}

			root := newTestRoot() // fresh flags for every example
			target, rest, err := root.Find(args)
			if err != nil {
				t.Errorf("%s: %q: %v", path, line, err)
				continue
			}
			if helpKey(target) != path {
				t.Errorf("%s: %q runs %q", path, line, helpKey(target))
				continue
			}
			if err := target.ParseFlags(rest); err != nil {
				t.Errorf("%s: %q: %v", path, line, err)
				continue
			}
			if err := target.ValidateArgs(target.Flags().Args()); err != nil {
				t.Errorf("%s: %q: %v", path, line, err)
			}
			if err := target.ValidateRequiredFlags(); err != nil {
				t.Errorf("%s: %q: %v", path, line, err)
			}
			if err := target.ValidateFlagGroups(); err != nil {
				t.Errorf("%s: %q: %v", path, line, err)
			}
		}
	}
	if checked < 40 {
		t.Errorf("only %d examples checked, expected more than 40", checked)
	}
}

// exampleArgs returns the arguments of the bunqctl command on a line. A
// leading "something |" is dropped, a trailing "| pager" too.
func exampleArgs(line string) ([]string, bool) {
	if i := strings.LastIndex(line, "| bunqctl "); i >= 0 {
		line = line[i+2:]
	}
	if !strings.HasPrefix(line, "bunqctl ") && line != "bunqctl" {
		return nil, false
	}
	if i := strings.Index(line, " | "); i >= 0 {
		line = line[:i]
	}
	return splitArgs(strings.TrimPrefix(line, "bunqctl")), true
}

// splitArgs splits a shell-like line: spaces separate words, double quotes
// group them.
func splitArgs(s string) []string {
	var args []string
	var cur strings.Builder
	inQuote, has := false, false
	for _, r := range s {
		switch {
		case r == '"':
			inQuote, has = !inQuote, true
		case r == ' ' && !inQuote:
			if has {
				args = append(args, cur.String())
				cur.Reset()
				has = false
			}
		default:
			cur.WriteRune(r)
			has = true
		}
	}
	if has {
		args = append(args, cur.String())
	}
	return args
}

func Test_splitArgs(t *testing.T) {
	got := splitArgs(` --env prod pay draft --to-name "Rent  Co" --amount 9.00`)
	want := []string{"--env", "prod", "pay", "draft", "--to-name", "Rent  Co", "--amount", "9.00"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("splitArgs = %q, want %q", got, want)
	}
	if args, ok := exampleArgs("pbpaste | bunqctl auth init --api-key-stdin"); !ok || args[0] != "auth" {
		t.Errorf("piped example: %q %v", args, ok)
	}
	if args, ok := exampleArgs("bunqctl docs safety | less"); !ok || len(args) != 2 {
		t.Errorf("example with a pager: %q %v", args, ok)
	}
}

// Test_help_rendersGroupsAndSafety renders real help output.
func Test_help_rendersGroupsAndSafety(t *testing.T) {
	var out, errOut bytes.Buffer
	a := newApp("test", strings.NewReader(""), &out, &errOut)
	if err := a.run(t.Context(), []string{"--help"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Setup:", "Read and export:", "Money:", "Getting paid:", "Sandbox:", "Help:", "Exit codes", "bunqctl docs"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("root help lacks %q", want)
		}
	}

	out.Reset()
	if err := newApp("test", strings.NewReader(""), &out, &errOut).run(t.Context(), []string{"move", "--help"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Safety: this moves money", "Examples:", "--dry-run"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("move help lacks %q", want)
		}
	}
}
