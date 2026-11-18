package bunq

import "context"

// Statement formats.
const (
	StatementCSV   = "CSV"
	StatementMT940 = "MT940"
	StatementPDF   = "PDF"
)

// NewStatement asks bunq to generate an account statement.
type NewStatement struct {
	Format         string `json:"statement_format"`
	DateStart      string `json:"date_start"` // YYYY-MM-DD
	DateEnd        string `json:"date_end"`
	RegionalFormat string `json:"regional_format,omitempty"` // CSV only: UK_US or EUROPEAN
	Attachments    bool   `json:"include_attachment,omitempty"`
}

// Statement is a generated account statement.
type Statement struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
	Format string `json:"statement_format"`
}

func statementPath(userID, accountID int, rest ...any) string {
	return apiPath(append([]any{"user", userID, "monetary-account", accountID, "customer-statement"}, rest...)...)
}

// CreateStatement starts generating a statement and returns its id.
func (c *Client) CreateStatement(ctx context.Context, userID, accountID int, s NewStatement) (int, error) {
	return create(ctx, c, statementPath(userID, accountID), s)
}

// Statement returns the state of a statement.
func (c *Client) Statement(ctx context.Context, userID, accountID, id int) (Statement, error) {
	return get[Statement](ctx, c, statementPath(userID, accountID, id))
}

// StatementContent downloads a generated statement.
func (c *Client) StatementContent(ctx context.Context, userID, accountID, id int) ([]byte, error) {
	return download(ctx, c, statementPath(userID, accountID, id, "content"))
}

// CreateAnnualOverview starts generating the annual overview of year.
func (c *Client) CreateAnnualOverview(ctx context.Context, userID, year int) (int, error) {
	return create(ctx, c, apiPath("user", userID, "export-annual-overview"), map[string]int{"year": year})
}

// AnnualOverviewContent downloads a generated annual overview (PDF).
func (c *Client) AnnualOverviewContent(ctx context.Context, userID, id int) ([]byte, error) {
	return download(ctx, c, apiPath("user", userID, "export-annual-overview", id, "content"))
}
