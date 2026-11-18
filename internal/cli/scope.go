package cli

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/Kisialeu/bunqctl/internal/bunq"
	"github.com/Kisialeu/bunqctl/internal/iban"
)

// maxExternalDescription is bunq's description limit for external IBANs.
const maxExternalDescription = 140

// withAccount opens a session and resolves ref (empty: first active account).
func (a *app) withAccount(cmd *cobra.Command, ref string) (*bunq.Client, int, bunq.MonetaryAccount, error) {
	c, userID, err := a.session()
	if err != nil {
		return nil, 0, bunq.MonetaryAccount{}, err
	}
	m, err := a.accountOrPrimary(cmd, c, userID, ref)
	return c, userID, m, err
}

// accountOrPrimary resolves ref. When ref is empty it returns the primary
// account: the first active bank (current) account, else the first active
// account of any type. bunq lists newest first, so "first active" alone
// would pick a newly opened savings account.
func (a *app) accountOrPrimary(cmd *cobra.Command, c *bunq.Client, userID int, ref string) (bunq.MonetaryAccount, error) {
	if ref != "" {
		return findAccount(cmd.Context(), c, userID, ref)
	}
	var fallback *bunq.MonetaryAccount
	for m, err := range c.MonetaryAccounts(cmd.Context(), userID) {
		if err != nil {
			return bunq.MonetaryAccount{}, err
		}
		if m.Status != statusActive {
			continue
		}
		if m.Type == "MonetaryAccountBank" {
			return m, nil
		}
		if fallback == nil {
			fallback = &m
		}
	}
	if fallback == nil {
		return bunq.MonetaryAccount{}, errors.New("no active account")
	}
	return *fallback, nil
}

// accountFlag registers the common --account flag.
func accountFlag(cmd *cobra.Command, ref *string) {
	cmd.Flags().StringVar(ref, "account", "", "account id or IBAN (default: primary account)")
}

// payee is a payment counterparty given either as an external IBAN with a
// name or as one of the user's own accounts.
type payee struct {
	iban, name, account string
}

func (p *payee) register(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVar(&p.iban, "to-iban", "", "recipient IBAN")
	f.StringVar(&p.name, "to-name", "", "recipient name (required with --to-iban)")
	f.StringVar(&p.account, "to-account", "", "one of your own accounts (id or IBAN)")
	cmd.MarkFlagsMutuallyExclusive("to-iban", "to-account")
	cmd.MarkFlagsOneRequired("to-iban", "to-account")
}

// pointer resolves the payee. external is false for the user's own accounts.
func (p payee) pointer(cmd *cobra.Command, c *bunq.Client, userID int) (ptr bunq.Pointer, external bool, err error) {
	if p.account != "" {
		m, err := findAccount(cmd.Context(), c, userID, p.account)
		if err != nil {
			return bunq.Pointer{}, false, err
		}
		return a2p(cmd, c, userID, m)
	}
	n := iban.Normalize(p.iban)
	if err := iban.Validate(n); err != nil {
		return bunq.Pointer{}, false, usageErr(err.Error())
	}
	if p.name == "" {
		return bunq.Pointer{}, false, usageErr("--to-name is required with --to-iban")
	}
	return bunq.Pointer{Type: "IBAN", Value: n, Name: p.name}, true, nil
}

// a2p turns an own account into an IBAN pointer named after the user, the
// account holder.
func a2p(cmd *cobra.Command, c *bunq.Client, userID int, m bunq.MonetaryAccount) (bunq.Pointer, bool, error) {
	if m.IBAN() == "" {
		return bunq.Pointer{}, false, fmt.Errorf("account %d has no IBAN", m.ID)
	}
	u, err := c.User(cmd.Context(), userID)
	if err != nil {
		return bunq.Pointer{}, false, err
	}
	return bunq.Pointer{Type: "IBAN", Value: m.IBAN(), Name: u.DisplayName}, false, nil
}

func checkDescription(desc string, external bool) error {
	if external && len([]rune(desc)) > maxExternalDescription {
		return usageErr(fmt.Sprintf("description is limited to %d characters for external IBANs", maxExternalDescription))
	}
	return nil
}

// parseID parses a positional object id.
func parseID(s string) (int, error) {
	id, err := strconv.Atoi(s)
	if err != nil || id <= 0 {
		return 0, usageErr(fmt.Sprintf("id %q must be a positive number", s))
	}
	return id, nil
}

// eur builds an EUR amount; bunqctl handles EUR accounts only.
func eur(c interface{ String() string }) bunq.Amount {
	return bunq.Amount{Value: c.String(), Currency: "EUR"}
}
