package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Kisialeu/bunqctl/docs"
	"github.com/Kisialeu/bunqctl/internal/output"
)

// docTopic is one built-in guide. Its text is the embedded docs/<Name>.txt.
type docTopic struct {
	Name    string `json:"topic"`
	Summary string `json:"summary"`
}

// docTopics lists the guides in reading order. A test fails when a file in
// docs/ has no topic here, so a new guide cannot be forgotten.
var docTopics = []docTopic{
	{"index", "What bunqctl does and does not do, and where to start"},
	{"getting-started", "Connect to the sandbox and to production, where state is stored, maintenance"},
	{"commands", "The command tree and a reference of every command"},
	{"use-cases", "Recipes: balances, spending, bookkeeping, tax, bills, savings, receipts, cards"},
	{"safety", "The write guard, what protects your money and where protection ends"},
	{"automation", "Scripts and cron: what can run unattended, exit codes, idempotency"},
	{"troubleshooting", "Messages, causes and fixes"},
	{"api-notes", "bunq endpoints used and how the real API behaves"},
	{"building", "Compile, release build, install, Docker"},
	{"architecture", "Code layout, request lifecycle, how to add a feature, how to write the docs"},
}

func newDocsCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "docs [topic]",
		Short: "Read the built-in documentation",
		Args:  cobra.MaximumNArgs(1),
		// The guides need no configuration, so skip the environment setup:
		// docs must also work where there is no home directory (a container).
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return output.List(a.printer(), docTopics, func(t docTopic) docTopic { return t }, []output.Column[docTopic]{
					{Header: "TOPIC", Value: func(t docTopic) string { return t.Name }},
					{Header: "SUMMARY", Value: func(t docTopic) string { return t.Summary }},
				})
			}
			t, ok := findTopic(args[0])
			if !ok {
				return usageErr(fmt.Sprintf("unknown topic %q (run 'bunqctl docs' for the list)", args[0]))
			}
			b, err := docs.FS.ReadFile(docFile(t))
			if err != nil {
				return fmt.Errorf("read built-in doc %s: %w", docFile(t), err)
			}
			_, err = a.out.Write(b)
			return err
		},
	}
}

// docFile is the embedded file of a topic.
func docFile(t docTopic) string { return t.Name + ".txt" }

// findTopic looks a topic up by name, ignoring case and a ".txt" suffix.
func findTopic(name string) (docTopic, bool) {
	name = strings.TrimSuffix(strings.ToLower(name), ".txt")
	for _, t := range docTopics {
		if t.Name == name {
			return t, true
		}
	}
	return docTopic{}, false
}
