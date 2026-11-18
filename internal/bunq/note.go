package bunq

import (
	"context"
	"iter"
)

// Note is a text note on a payment.
type Note struct {
	ID      int    `json:"id"`
	Created Time   `json:"created"`
	Content string `json:"content"`
}

func paymentPath(userID, accountID, paymentID int, rest ...any) string {
	return apiPath(append([]any{"user", userID, "monetary-account", accountID, "payment", paymentID}, rest...)...)
}

// CreateNote adds a text note to a payment and returns its id.
func (c *Client) CreateNote(ctx context.Context, userID, accountID, paymentID int, text string) (int, error) {
	return create(ctx, c, paymentPath(userID, accountID, paymentID, "note-text"), map[string]string{"content": text})
}

// Notes iterates the text notes of a payment.
func (c *Client) Notes(ctx context.Context, userID, accountID, paymentID int) iter.Seq2[Note, error] {
	return listAs[Note](ctx, c, paymentPath(userID, accountID, paymentID, "note-text"), nil)
}

// UploadAttachment uploads a file (e.g. a receipt) to an account and returns
// the attachment id. The body is the raw file; bunq signs and checks it like
// any other body.
func (c *Client) UploadAttachment(ctx context.Context, userID, accountID int, contentType, description string, data []byte) (int, error) {
	return upload(ctx, c, apiPath("user", userID, "monetary-account", accountID, "attachment"),
		contentType, map[string]string{"X-Bunq-Attachment-Description": description}, data)
}

// AttachToPayment links an uploaded attachment to a payment and returns the
// note id.
func (c *Client) AttachToPayment(ctx context.Context, userID, accountID, paymentID, attachmentID int, description string) (int, error) {
	return create(ctx, c, paymentPath(userID, accountID, paymentID, "note-attachment"), map[string]any{
		"attachment_id": attachmentID,
		"description":   description,
	})
}
