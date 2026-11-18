package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Kisialeu/bunqctl/internal/atomicfile"
	"github.com/Kisialeu/bunqctl/internal/bunq"
)

const (
	statementPoll    = 2 * time.Second
	statementTimeout = 2 * time.Minute
)

func newStatementCmd(a *app) *cobra.Command {
	var (
		account, format, from, to, out, regional string
		attachments, force                       bool
	)
	cmd := &cobra.Command{
		Use:   "statement",
		Short: "Download an official account statement (CSV, MT940 or PDF)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := bunq.NewStatement{Format: strings.ToUpper(format), DateStart: from, DateEnd: to, Attachments: attachments}
			switch req.Format {
			case bunq.StatementCSV:
				switch regional {
				case "uk":
					req.RegionalFormat = "UK_US"
				case "eu":
					req.RegionalFormat = "EUROPEAN"
				default:
					return usageErr("--regional must be uk (comma) or eu (semicolon)")
				}
			case bunq.StatementMT940, bunq.StatementPDF:
			default:
				return usageErr("--format must be csv, mt940 or pdf")
			}
			if _, err := (&txFlags{from: from, to: to}).filter(); err != nil {
				return err
			}
			if err := checkNotExists(out, force); err != nil {
				return err
			}

			c, userID, m, err := a.withAccount(cmd, account)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			id, err := c.CreateStatement(ctx, userID, m.ID, req)
			if err != nil {
				return err
			}
			if err := waitFor(ctx, func() (bool, error) {
				s, err := c.Statement(ctx, userID, m.ID, id)
				if err == nil && s.Status == "FAILED" {
					err = errors.New("bunq failed to generate it")
				}
				return s.Status == "COMPLETED", err
			}); err != nil {
				return fmt.Errorf("statement %d: %w", id, err)
			}
			data, err := c.StatementContent(ctx, userID, m.ID, id)
			if err != nil {
				return err
			}
			return a.writeOutput(out, data, fmt.Sprintf("%s statement of account %d", req.Format, m.ID))
		},
	}
	f := cmd.Flags()
	accountFlag(cmd, &account)
	f.StringVar(&format, "format", "pdf", "csv, mt940 or pdf")
	f.StringVar(&from, "from", "", "first day, YYYY-MM-DD")
	f.StringVar(&to, "to", "", "last day, YYYY-MM-DD")
	f.StringVar(&out, "out", "", "output file (mode 0600)")
	f.StringVar(&regional, "regional", "eu", "CSV only: uk (comma-separated) or eu (semicolon-separated)")
	f.BoolVar(&attachments, "attachments", false, "PDF only: include receipts and other attachments")
	f.BoolVar(&force, "force", false, "overwrite an existing file")
	for _, name := range []string{"from", "to", "out"} {
		_ = cmd.MarkFlagRequired(name) // only fails for an unknown flag name
	}
	return cmd
}

func newAnnualOverviewCmd(a *app) *cobra.Command {
	var (
		year  int
		out   string
		force bool
	)
	cmd := &cobra.Command{
		Use:   "annual-overview",
		Short: "Download the annual overview (PDF) for taxes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if year < 2015 || year > time.Now().Year() {
				return usageErr(fmt.Sprintf("--year %d is out of range", year))
			}
			if err := checkNotExists(out, force); err != nil {
				return err
			}
			c, userID, err := a.session()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			id, err := c.CreateAnnualOverview(ctx, userID, year)
			if err != nil {
				return err
			}
			var data []byte
			if err := waitFor(ctx, func() (bool, error) {
				var apiErr *bunq.APIError
				data, err = c.AnnualOverviewContent(ctx, userID, id)
				if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
					return false, nil // still being generated
				}
				return err == nil, err
			}); err != nil {
				return fmt.Errorf("annual overview %d: %w", id, err)
			}
			return a.writeOutput(out, data, fmt.Sprintf("annual overview %d", year))
		},
	}
	f := cmd.Flags()
	f.IntVar(&year, "year", time.Now().Year()-1, "year")
	f.StringVar(&out, "out", "", "output file (mode 0600)")
	f.BoolVar(&force, "force", false, "overwrite an existing file")
	_ = cmd.MarkFlagRequired("out") // only fails for an unknown flag name
	return cmd
}

// waitFor polls ready until it reports true, fails, or the timeout passes.
func waitFor(ctx context.Context, ready func() (bool, error)) error {
	ctx, cancel := context.WithTimeout(ctx, statementTimeout)
	defer cancel()
	for {
		ok, err := ready()
		if err != nil || ok {
			return err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("not ready after %s", statementTimeout)
		case <-time.After(statementPoll):
		}
	}
}

// checkNotExists refuses to overwrite path unless force is set.
func checkNotExists(path string, force bool) error {
	if force {
		return nil
	}
	_, err := os.Stat(path)
	switch {
	case err == nil:
		return fmt.Errorf("%s exists: use --force to overwrite", path)
	case errors.Is(err, fs.ErrNotExist):
		return nil
	default:
		return err
	}
}

// writeOutput writes data atomically with mode 0600 and reports it.
func (a *app) writeOutput(path string, data []byte, what string) error {
	if err := atomicfile.Write(path, data); err != nil {
		return err
	}
	fmt.Fprintf(a.errOut, "wrote %s to %s (%d bytes)\n", what, path, len(data))
	return nil
}
