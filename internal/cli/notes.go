package cli

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/Kisialeu/bunqctl/internal/bunq"
	"github.com/Kisialeu/bunqctl/internal/output"
)

// maxAttachmentBytes keeps uploads to receipt-sized files.
const maxAttachmentBytes = 10 << 20

// attachmentTypes are the file types bunqctl uploads.
var attachmentTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "application/pdf": true}

// notesCmds returns the tx subcommands that annotate a payment.
func notesCmds(a *app) []*cobra.Command {
	var (
		noteAccount string
		noteWF      *writeFlags
	)
	note := &cobra.Command{
		Use:   "note <payment-id> <text>",
		Short: "Add a text note to a transaction",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			c, userID, m, err := a.withAccount(cmd, noteAccount)
			if err != nil {
				return err
			}
			ch := change{op: "note", body: map[string]string{"content": args[1]}, summary: [][2]string{{"Payment", args[0]}, {"Note", args[1]}}}
			if ok, err := a.approve(ch, noteWF); err != nil || !ok {
				return err
			}
			noteID, err := c.CreateNote(cmd.Context(), userID, m.ID, id, args[1])
			if err != nil {
				return err
			}
			fmt.Fprintf(a.errOut, "added note %d to payment %d\n", noteID, id)
			return nil
		},
	}
	accountFlag(note, &noteAccount)
	noteWF = addWriteFlags(note, false)

	var notesAccount string
	notes := &cobra.Command{
		Use:   "notes <payment-id>",
		Short: "List the text notes of a transaction",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			c, userID, m, err := a.withAccount(cmd, notesAccount)
			if err != nil {
				return err
			}
			items, err := collect(c.Notes(cmd.Context(), userID, m.ID, id))
			if err != nil {
				return err
			}
			type noteView struct {
				ID      int       `json:"id"`
				Date    time.Time `json:"date"`
				Content string    `json:"content"`
			}
			return output.List(a.printer(), items,
				func(n bunq.Note) noteView { return noteView{n.ID, n.Created.Time, n.Content} },
				[]output.Column[bunq.Note]{
					{Header: "DATE", Value: func(n bunq.Note) string { return n.Created.Local().Format("2006-01-02 15:04") }},
					{Header: "NOTE", Value: func(n bunq.Note) string { return n.Content }},
					{Header: "ID", Value: func(n bunq.Note) string { return strconv.Itoa(n.ID) }},
				})
		},
	}
	accountFlag(notes, &notesAccount)

	var (
		attachAccount, description string
		attachWF                   *writeFlags
	)
	attach := &cobra.Command{
		Use:   "attach <payment-id> <file>",
		Short: "Attach a receipt (PNG, JPEG, GIF or PDF) to a transaction",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			data, contentType, err := readAttachment(args[1])
			if err != nil {
				return err
			}
			if description == "" {
				description = filepath.Base(args[1])
			}
			c, userID, m, err := a.withAccount(cmd, attachAccount)
			if err != nil {
				return err
			}
			ch := change{
				op:   "attach",
				body: map[string]any{"file": args[1], "content_type": contentType, "bytes": len(data), "description": description},
				summary: [][2]string{
					{"Payment", args[0]}, {"File", args[1]}, {"Type", contentType}, {"Size", strconv.Itoa(len(data)) + " bytes"},
				},
			}
			if ok, err := a.approve(ch, attachWF); err != nil || !ok {
				return err
			}
			attachmentID, err := c.UploadAttachment(cmd.Context(), userID, m.ID, contentType, description, data)
			if err != nil {
				return fmt.Errorf("upload: %w", err)
			}
			noteID, err := c.AttachToPayment(cmd.Context(), userID, m.ID, id, attachmentID, description)
			if err != nil {
				return err
			}
			fmt.Fprintf(a.errOut, "attached %s to payment %d (attachment %d, note %d)\n", filepath.Base(args[1]), id, attachmentID, noteID)
			return nil
		},
	}
	accountFlag(attach, &attachAccount)
	attach.Flags().StringVar(&description, "description", "", "attachment description (default: file name)")
	attachWF = addWriteFlags(attach, false)

	return []*cobra.Command{note, notes, attach}
}

func readAttachment(path string) ([]byte, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", err
	}
	if info.Size() > maxAttachmentBytes {
		return nil, "", usageErr(fmt.Sprintf("%s is larger than %d MB", path, maxAttachmentBytes>>20))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	contentType := http.DetectContentType(data)
	if !attachmentTypes[contentType] {
		return nil, "", usageErr(fmt.Sprintf("%s is %s; only PNG, JPEG, GIF and PDF are supported", path, contentType))
	}
	return data, contentType, nil
}
