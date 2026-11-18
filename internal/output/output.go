// Package output renders command results as a table or as JSON.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// Column describes one table column of a T.
type Column[T any] struct {
	Header string
	Value  func(T) string
	Right  bool // right-align, e.g. amounts
}

// Printer writes results in the selected format.
type Printer struct {
	W    io.Writer
	JSON bool
}

// List prints items as a table with cols, or as a JSON array of the values
// returned by view. view lets commands keep a stable JSON schema that does
// not depend on the bunq response.
func List[T, V any](p Printer, items []T, view func(T) V, cols []Column[T]) error {
	if p.JSON {
		out := make([]V, 0, len(items))
		for _, it := range items {
			out = append(out, view(it))
		}
		return p.Value(out)
	}
	return table(p.W, items, cols)
}

// Value prints v as indented JSON.
func (p Printer) Value(v any) error {
	enc := json.NewEncoder(p.W)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}

// Fields prints key-value pairs as a two-column table, or v as JSON.
func (p Printer) Fields(v any, pairs [][2]string) error {
	if p.JSON {
		return p.Value(v)
	}
	tw := tabwriter.NewWriter(p.W, 0, 0, 2, ' ', 0)
	for _, kv := range pairs {
		fmt.Fprintf(tw, "%s:\t%s\n", kv[0], kv[1])
	}
	return tw.Flush()
}

func table[T any](w io.Writer, items []T, cols []Column[T]) error {
	rows := make([][]string, 0, len(items)+1)
	header := make([]string, len(cols))
	for i, c := range cols {
		header[i] = c.Header
	}
	rows = append(rows, header)
	for _, it := range items {
		row := make([]string, len(cols))
		for i, c := range cols {
			row[i] = c.Value(it)
		}
		rows = append(rows, row)
	}

	widths := make([]int, len(cols))
	for _, row := range rows {
		for i, cell := range row {
			widths[i] = max(widths[i], len([]rune(cell)))
		}
	}

	var b strings.Builder
	for _, row := range rows {
		for i, cell := range row {
			pad := strings.Repeat(" ", widths[i]-len([]rune(cell)))
			if i > 0 {
				b.WriteString("  ")
			}
			switch {
			case cols[i].Right:
				b.WriteString(pad + cell)
			case i < len(row)-1:
				b.WriteString(cell + pad)
			default:
				b.WriteString(cell)
			}
		}
		b.WriteString("\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}
