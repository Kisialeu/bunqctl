package cli

import (
	"bytes"
	"errors"
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/Kisialeu/bunqctl/docs"
)

func Test_docTopics_matchEmbeddedFiles(t *testing.T) {
	files, err := fs.Glob(docs.FS, "*.txt")
	if err != nil {
		t.Fatal(err)
	}
	onDisk := map[string]bool{}
	for _, f := range files {
		onDisk[f] = true
	}
	inTopics := map[string]bool{}
	for _, tp := range docTopics {
		if inTopics[tp.Name] {
			t.Errorf("duplicate topic %q", tp.Name)
		}
		inTopics[tp.Name] = true
		if !onDisk[docFile(tp)] {
			t.Errorf("topic %q has no file docs/%s", tp.Name, docFile(tp))
		}
		if tp.Summary == "" {
			t.Errorf("topic %q has no summary", tp.Name)
		}
	}
	for f := range onDisk {
		if !inTopics[strings.TrimSuffix(f, ".txt")] {
			t.Errorf("docs/%s has no entry in docTopics", f)
		}
	}
}

func Test_docs_listAndRead(t *testing.T) {
	var out, errOut bytes.Buffer
	a := newApp("test", strings.NewReader(""), &out, &errOut)
	t.Setenv("HOME", "") // docs must not need a home directory
	t.Setenv("XDG_CONFIG_HOME", "")

	if err := a.run(t.Context(), []string{"docs"}); err != nil {
		t.Fatal(err)
	}
	for _, tp := range docTopics {
		if !strings.Contains(out.String(), tp.Name) {
			t.Errorf("topic list lacks %q", tp.Name)
		}
	}

	out.Reset()
	if err := a.run(t.Context(), []string{"docs", "safety"}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "SAFETY\n======\n") {
		t.Errorf("docs safety output starts %q", out.String()[:min(40, out.Len())])
	}

	for _, name := range []string{"Safety", "safety.txt", "INDEX"} {
		out.Reset()
		if err := a.run(t.Context(), []string{"docs", name}); err != nil || out.Len() == 0 {
			t.Errorf("docs %s: err=%v bytes=%d", name, err, out.Len())
		}
	}

	out.Reset()
	if err := a.run(t.Context(), []string{"docs", "--json"}); err != nil || !strings.Contains(out.String(), `"topic": "safety"`) {
		t.Errorf("docs --json: err=%v out=%q", err, out.String())
	}
}

func Test_docs_unknownTopicIsUsageError(t *testing.T) {
	a := newApp("test", strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	err := a.run(t.Context(), []string{"docs", "nonsense"})
	if !errors.Is(err, errUsage) || exitCode(err) != exitUsage {
		t.Errorf("err = %v, want a usage error", err)
	}
}

// markdown syntax that must not come back: the guides are read in a terminal.
//
// Code blocks are indented by four spaces or more and may legitimately hold
// pipes and backticks (shell pipelines, Go struct tags), so the checks marked
// proseOnly skip those lines.
var markdownSyntax = []struct {
	what      string
	re        *regexp.Regexp
	proseOnly bool
}{
	{"code fence", regexp.MustCompile("```"), false},
	{"heading marker", regexp.MustCompile(`^#{1,6} `), false},
	{"table row", regexp.MustCompile(`^ {0,3}\|`), false},
	{"table separator", regexp.MustCompile(`\|\s*-{3,}\s*\|`), false},
	{"bold", regexp.MustCompile(`\*\*\w`), false},
	{"link", regexp.MustCompile(`\[[^\]]+\]\((https?://|#|[\w./-]+\.md)`), false},
	{"inline code", regexp.MustCompile("`"), true},
}

// Test_docs_arePlainText fails when Markdown creeps back into a guide, when a
// heading has no underline of the right length, or when prose is not wrapped.
func Test_docs_arePlainText(t *testing.T) {
	for _, tp := range docTopics {
		b, err := docs.FS.ReadFile(docFile(tp))
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(b), "\n")
		if lines[len(lines)-1] != "" {
			t.Errorf("%s: no newline at the end", docFile(tp))
		}
		for i, line := range lines {
			where := func(what string) { t.Errorf("%s:%d: %s: %q", docFile(tp), i+1, what, line) }
			if strings.TrimRight(line, " ") != line {
				where("trailing spaces")
			}
			indentedCode := strings.HasPrefix(line, "    ")
			for _, m := range markdownSyntax {
				if m.proseOnly && indentedCode {
					continue
				}
				if m.re.MatchString(line) {
					where("markdown " + m.what)
				}
			}
			// Prose and tables stay within 100 columns; indented code may be wider.
			if len(line) > 100 && !strings.HasPrefix(line, "    ") {
				where("line is longer than 100 columns")
			}
			// A heading is a line followed by an underline of the same length.
			if i+1 < len(lines) && len(line) > 0 && isUnderline(lines[i+1]) && len(lines[i+1]) != len([]rune(line)) {
				where("heading underline has a different length")
			}
		}
		if !isUnderline(lines[1]) || !strings.HasPrefix(lines[1], "=") {
			t.Errorf("%s: the first line must be a title underlined with =", docFile(tp))
		}
	}
}

var underlineRe = regexp.MustCompile(`^(=+|-+|~+)$`)

func isUnderline(s string) bool { return len(s) >= 3 && underlineRe.MatchString(s) }
