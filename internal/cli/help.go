package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

// The texts shown by --help live here, in one place, keyed by command path
// ("pay draft"). help_test.go fails when a command has no entry, an example
// stops being a valid command line, or a flag has no description.

const rootLong = `bunqctl reads and manages your bunq accounts from the terminal.

Environments
  sandbox (default)   fake money, safe for experiments
  prod                your real account: --env prod, or BUNQCTL_ENV=prod

First steps
  bunqctl auth init --permitted-ip <your public IPv4>
  bunqctl accounts list

Safety
  - Payments to other people are drafts: you approve them in the bunq app.
  - move and sweep pay directly, but only between your own accounts.
  - Every command that changes something has --dry-run and asks to confirm.
  - In prod, commands that move money make you type the amount, even with
    --yes, so they cannot run unattended.
  - Keys and tokens live in the macOS Keychain and are never printed.

Output
  A table by default, --json for scripts. Results go to stdout; summaries,
  messages and errors go to stderr.

Exit codes
  0 success   1 error or refused write   2 wrong usage
  3 not initialized (run auth init)   4 cancelled

Documentation
  bunqctl docs                 list the built-in guides
  bunqctl docs safety          read one`

const (
	safetyWrite = "Safety: this changes data at bunq. --dry-run prints the request without sending it. You are asked to confirm unless you pass --yes."
	safetyMoney = "Safety: this moves money. --dry-run prints the request without sending it. In prod you must type the amount to confirm, even with --yes, so it cannot run unattended. It is refused above max-amount (config set max-amount), and when an identical request was sent in the last 10 minutes (--force overrides)."
)

// helpEntry is the help of one command. safety is appended to long.
type helpEntry struct {
	long, example, safety string
}

func group(long string) helpEntry              { return helpEntry{long: long} }
func read(long string, ex ...string) helpEntry { return helpEntry{long, examples(ex), ""} }
func write(long string, ex ...string) helpEntry {
	return helpEntry{long, examples(ex), safetyWrite}
}
func spend(long string, ex ...string) helpEntry {
	return helpEntry{long, examples(ex), safetyMoney}
}

func examples(lines []string) string { return "  " + strings.Join(lines, "\n  ") }

// commandGroups are the sections of the top-level command list.
var commandGroups = []*cobra.Group{
	{ID: "setup", Title: "Setup:"},
	{ID: "read", Title: "Read and export:"},
	{ID: "money", Title: "Money:"},
	{ID: "paid", Title: "Getting paid:"},
	{ID: "sandbox", Title: "Sandbox:"},
	{ID: "help", Title: "Help:"},
}

var groupOf = map[string]string{
	"auth": "setup", "config": "setup",
	"user": "read", "accounts": "read", "tx": "read", "events": "read", "insights": "read",
	"limits": "read", "cards": "read", "categories": "read", "auto-allocate": "read",
	"export": "read", "statement": "read", "annual-overview": "read",
	"pay": "money", "move": "money", "sweep": "money", "schedule": "money",
	"request": "paid", "bunqme": "paid",
	"sandbox": "sandbox",
	"docs":    "help",
}

// applyHelp sets Long and Example from the help table and puts the top-level
// commands into their groups.
func applyHelp(root *cobra.Command) {
	root.Long = rootLong
	root.AddGroup(commandGroups...)
	root.SetHelpCommandGroupID("help")
	root.SetCompletionCommandGroupID("help")
	for _, c := range root.Commands() {
		c.GroupID = groupOf[c.Name()]
	}
	walkCommands(root, func(c *cobra.Command) {
		if c == root {
			return
		}
		e, ok := helpTable[helpKey(c)]
		if !ok {
			return
		}
		c.Long = e.long
		if e.safety != "" {
			c.Long += "\n\n" + e.safety
		}
		c.Example = e.example
	})
}

// helpKey is the command path without the program name: "pay draft".
func helpKey(c *cobra.Command) string {
	return strings.TrimPrefix(c.CommandPath(), c.Root().Name()+" ")
}

func walkCommands(c *cobra.Command, fn func(*cobra.Command)) {
	fn(c)
	for _, sub := range c.Commands() {
		walkCommands(sub, fn)
	}
}

var helpTable = map[string]helpEntry{
	// Setup
	"auth": group(`Register this machine with bunq and inspect the connection.

Start with 'auth init'. The other commands show or renew what it set up.`),

	"auth init": read(`Register this machine with bunq: installation, device and session.

In the sandbox a test user is created for you. In prod the API key from the
bunq app is read from a hidden prompt, or from stdin with --api-key-stdin.

A standard API key cannot get new IPs later, so give every public IPv4 you
will use with --permitted-ip. bunqctl refuses to continue when your current
IP is not in the list. bunq allows only about 10 setup calls per day and this
command uses 2, so do not run it in a loop.

Keys and tokens go to the macOS Keychain, the rest to
~/.config/bunqctl/<env>.json. A second run for the same environment is
refused unless you pass --force.`,
		`bunqctl auth init --permitted-ip 203.0.113.10`,
		`bunqctl --env prod auth init --permitted-ip 203.0.113.10 --permitted-ip 203.0.113.11`,
		`pbpaste | bunqctl --env prod auth init --permitted-ip 203.0.113.10 --api-key-stdin`),

	"auth status": read(`Show the environment, user and device ids, the permitted IPs next to your current
public IP, and the session age. Never prints secrets. The public IP is looked
up at api.ipify.org.

--json prints: env, user_id, device_id, permitted_ips, current_ip,
session_created.`,
		`bunqctl auth status`, `bunqctl --env prod auth status --json`),

	"auth session": read(`Open a new session and store its token. Sessions renew themselves when bunq
rejects the token, so you rarely need this.`,
		`bunqctl auth session`),

	"auth logout": read(`Delete the Keychain entries and the profile of the selected environment, after
confirmation. The device stays registered at bunq: if you stop using the API
key, revoke it in the bunq app as well.`,
		`bunqctl auth logout`, `bunqctl --env prod auth logout`),

	"config": group(`Local settings of an environment. They live in ~/.config/bunqctl/<env>.json and
are not sent to bunq.`),

	"config show": read(`Show the local settings: permitted IPs, region, country and max-amount.

--json prints: env, permitted_ips, region, country, max_amount.`,
		`bunqctl config show`, `bunqctl --env prod config show --json`),

	"config set": read(`Set a local setting. The only one is max-amount: refuse any money write above
this amount (for a batch, the total). Use none to remove the limit.

It protects against mistakes, not against someone who can edit the profile
file; bunq does not enforce it.`,
		`bunqctl --env prod config set max-amount 500.00`, `bunqctl config set max-amount none`),

	// Read
	"user": read(`Show the logged-in user: id and name.

--json prints: id, display_name.`,
		`bunqctl user`, `bunqctl --env prod user --json`),

	"accounts": group(`Accounts and balances. The "primary account" used when --account is omitted in
other commands is your first active current account.`),

	"accounts list": read(`List your accounts with IBAN, balance and status. Only active accounts are
shown; --all adds closed and blocked ones.

--json prints: id, type, description, iban, balance, currency, status. The
type is the bunq account type without its prefix: Bank, Savings, Joint, ...`,
		`bunqctl accounts list`, `bunqctl accounts list --all --json`),

	"accounts show": read(`Show one account by id or IBAN. An IBAN may contain spaces and any letter case.`,
		`bunqctl accounts show 3851717`, `bunqctl accounts show "NL07 BUNQ 2108 2146 23"`),

	"accounts savings": read(`List savings accounts with their goal and progress.

--json adds goal and progress to the fields of 'accounts list'.`,
		`bunqctl accounts savings`),

	"tx": group(`Transactions of an account, and notes and receipts attached to them.`),

	"tx list": read(`List the transactions of one account, newest first. Without --account the
primary account is used.

--from and --to are local days, --to included. --min and --max compare the
absolute amount, so --min 10.00 finds both +12.00 and -12.00. --search looks
in the description and the counterparty name, ignoring case. The default
--limit is 50; use --limit 0 for everything.

--json prints: id, account_id, date, amount, currency, counterparty,
counterparty_iban, description, type, balance_after. Amounts are strings with
two decimals, negative for outgoing payments.`,
		`bunqctl tx list`,
		`bunqctl tx list --from 2026-11-01 --search coffee`,
		`bunqctl tx list --min 100.00 --limit 0 --json`),

	"tx note": write(`Add a text note to a payment. Find the payment id with 'tx list'.`,
		`bunqctl tx note 36765845 "Lunch with the team"`),

	"tx notes": read(`List the text notes of a payment.

--json prints: id, date, content.`,
		`bunqctl tx notes 36765845`),

	"tx attach": write(`Upload a receipt and attach it to a payment. PNG, JPEG, GIF or PDF, at most
10 MB; the type is detected from the content, not from the file name. The
description defaults to the file name. Attachments are included in PDF
statements made with 'statement --attachments'.`,
		`bunqctl tx attach 36765845 ~/Downloads/receipt.pdf`,
		`bunqctl tx attach 36765845 receipt.png --description "Team lunch"`),

	"events": read(`Show the activity feed: payments, card transactions, requests and more,
newest first. The default --limit is 25.

--json prints: id, date, action, type, account_id, and when present amount,
currency, counterparty, description.`,
		`bunqctl events`, `bunqctl events --limit 10 --json`),

	"insights": read(`Show spending per bunq category for a period. The default period is this
month until now. bunq indexes new transactions with a delay, so very recent
payments may be missing.

--json prints: category, total, currency, transactions.`,
		`bunqctl insights`, `bunqctl insights --from 2026-01-01 --to 2026-12-31`),

	"limits": read(`Show account, card and monthly spending limits. Values that bunq does not
return show as "-".

--json prints: limit_monetary_account, limit_monetary_account_remaining,
limit_amount_monthly, spent_amount_monthly, limit_card_debit_maestro,
limit_card_debit_mastercard, limit_card_replacement.`,
		`bunqctl limits`),

	"cards": group(`Payment cards. bunqctl lists cards and freezes or unfreezes them. It never
blocks a card permanently (lost, stolen, cancelled): use the bunq app.`),

	"cards list": read(`List cards with type, last four digits, name, expiry, status and the linked
account.

--json prints: id, type, product_type, status, order_status, last_digits,
name_on_card, expiry_date, account_iban.`,
		`bunqctl cards list`, `bunqctl cards list --json`),

	"cards freeze": write(`Block a card temporarily (status DEACTIVATED). Reversible with 'cards
unfreeze'. LOST, STOLEN and CANCELLED are permanent at bunq and are not
available here.

After the change the card is read back. If it did not change, for example
because it is not activated yet, the command fails and shows the real status.`,
		`bunqctl --env prod cards freeze 516206`),

	"cards unfreeze": write(`Unblock a frozen card (status ACTIVE). The card is read back afterwards; if it
did not change, the command fails and shows the real status.`,
		`bunqctl --env prod cards unfreeze 516206`),

	"categories": group(`Transaction categories: bunq's built-in ones and up to nine of your own.`),

	"categories list": read(`List built-in and user-defined categories. Unused user-defined slots are
shown as INACTIVE.

--json prints: category, type, status, description, color, icon.`,
		`bunqctl categories list`),

	"categories create": write(`Create a user-defined category. It takes the next free slot
(USER_DEFINED_1 to USER_DEFINED_9) and the name becomes its description. When
all nine slots are active the command fails.

--color is one of YELLOW, DEEP_YELLOW, ORANGE, HOT_PINK, RED, INDIGO, PURPLE,
DEEP_PURPLE, BLUE (default), LIGHT_BLUE, DEEP_BLUE, CYAN, TURQUOISE, GREEN,
FUCHSIA, DEEP_GREEN, GRAY, DARK_GRAY, CALM_GRAY, BLACK. --icon is an icon name
(default GENERAL); see the icon field of 'categories list --json'.`,
		`bunqctl categories create "Board games"`,
		`bunqctl categories create Coffee --icon FOOD_AND_DRINK --color DEEP_YELLOW`),

	"auto-allocate": group(`Rules that split incoming payments. Rules are created in the bunq app.`),

	"auto-allocate list": read(`List your auto-allocate rules. Read-only.

--json prints: id, type, status, trigger_amount.`,
		`bunqctl auto-allocate list`),

	"export": read(`Write all transactions of a period to a JSON file, newest first, in the schema
of 'tx list --json' (plus account_id). There is no row limit.

--account all exports every account, including closed ones. The file is
written atomically with mode 0600 and never overwritten without --force.`,
		`bunqctl export --from 2026-01-01 --to 2026-12-31 --out 2026.json`,
		`bunqctl export --account all --from 2026-11-01 --to 2026-11-30 --out nov.json`),

	"statement": read(`Download an official statement that bunq generates for one account. bunqctl
requests it, waits up to 2 minutes and saves the file (mode 0600). An existing
file is never overwritten without --force.

--format is pdf (default), csv or mt940. --regional applies to csv: eu is
semicolon-separated (default), uk is comma-separated. --attachments adds
receipts to a PDF. For several accounts run it once per --account, or use
'export --account all' for one JSON file.`,
		`bunqctl statement --from 2026-01-01 --to 2026-12-31 --out 2026.pdf`,
		`bunqctl statement --format csv --regional eu --from 2026-11-01 --to 2026-11-30 --out nov.csv`,
		`bunqctl statement --format mt940 --from 2026-11-01 --to 2026-11-30 --out nov.mt940`),

	"annual-overview": read(`Download the annual overview as a PDF, useful for taxes. The default year is
last year. bunq refuses years in which you had no active account. An
existing file is never overwritten without --force.`,
		`bunqctl annual-overview --year 2026 --out overview-2026.pdf`),

	// Money
	"pay": group(`Payments to other people. They are drafts: bunqctl creates them and you
approve them in the bunq app. bunqctl can never approve a draft.`),

	"pay draft": spend(`Create a draft payment that you approve in the bunq app. Nothing is paid until
you approve it there.

Give the recipient as --to-iban with --to-name (an external account), or as
--to-account (one of your own accounts). Descriptions to external IBANs are
limited to 140 characters. The source is --account, by default the primary
account.

Afterwards 'pay status <draft-id>' shows the state and 'pay cancel
<draft-id>' withdraws it.`,
		`bunqctl pay draft --to-iban NL32BUNQ2025313705 --to-name "Landlord" --amount 950.00 --description "Rent December"`,
		`bunqctl pay draft --to-account 3851794 --amount 50.00 --description "To savings" --dry-run`),

	"pay batch": spend(`Create one draft with 1 to 350 payments from a JSON file. You approve it once in
the bunq app.

The file is an array of objects with to_iban, to_name, amount and description:
  [{"to_iban": "NL32BUNQ2025313705", "to_name": "Landlord", "amount": "950.00", "description": "Rent"}]

The whole file is checked first (IBAN checksums, amounts, required fields,
description length); nothing is sent if one entry is invalid. max-amount
applies to the total.`,
		`bunqctl pay batch bills.json --dry-run`, `bunqctl pay batch bills.json`),

	"pay status": read(`Show the state of a draft payment: PENDING until you decide in the bunq app,
then ACCEPTED or REJECTED; EXPIRED when nobody did.

--json prints: id, status, entries, total.`,
		`bunqctl pay status 9948`, `bunqctl pay status 9948 --json`),

	"pay cancel": write(`Reject a pending draft payment. bunq has no "cancelled" state for drafts, so it
ends as REJECTED. bunqctl never sends ACCEPTED: only the bunq app can approve.`,
		`bunqctl pay cancel 9948`),

	"move": spend(`Pay from one of your accounts to another one of yours, directly: no draft and
no approval in the app. Both accounts must be yours, and they must differ.
--from defaults to the primary account.`,
		`bunqctl move --to 3851794 --amount 50.00`,
		`bunqctl move --from 3851794 --to 3851717 --amount 20.00 --description "Back to current"`),

	"sweep": spend(`Move money to another account of yours, directly (no draft). Two modes:

  --keep X                      move everything above X on the source account
  --percent N --since D --until D
                                move N% of the money other people paid you on
                                the source account in that period

Money sent between your own accounts does not count as income. --percent runs
once per period: the payment description "bunqctl sweep N% D..D" is the
marker, and a second run reports "period already swept". When there is nothing
to move it prints "nothing to sweep" and exits 0.`,
		`bunqctl sweep --to 3851794 --keep 500.00`,
		`bunqctl sweep --to 3851794 --percent 10 --since 2026-11-01 --until 2026-11-30 --dry-run`),

	"schedule": group(`Scheduled payments (standing orders). They run at bunq without any approval in
the app, so creating one is like making the payment.`),

	"schedule list": read(`List the standing orders of an account.

--json prints: id, status, amount, counterparty, counterparty_iban,
description, every, start, end.`,
		`bunqctl schedule list`, `bunqctl schedule list --account 3851717 --json`),

	"schedule create": spend(`Create a standing order from an account.

--start is local midnight of that day and may not be in the past. --every is
once, daily, weekly, monthly (default) or yearly; --interval 2 with --every
weekly means every two weeks. --end is the last day it runs.

Give the recipient as --to-iban with --to-name, or as --to-account (one of
your own accounts).

A standing order runs at bunq repeatedly and without approval in the app.
Cancel it with 'schedule cancel <id>'.`,
		`bunqctl schedule create --to-account 3851794 --amount 100.00 --start 2026-12-01 --description "Monthly savings"`,
		`bunqctl schedule create --to-iban NL32BUNQ2025313705 --to-name Gym --amount 29.99 --every monthly --start 2026-12-05 --end 2027-12-05 --description Gym --dry-run`),

	"schedule cancel": write(`Delete a standing order. Find its id with 'schedule list'.`,
		`bunqctl schedule cancel 34208`),

	// Getting paid
	"request": group(`Ask other people to pay you.`),

	"request create": write(`Ask someone to pay you. --to is an email address, a phone number (+31...) or
an IBAN, in which case --name is required. Requests are created with bunq.me
enabled, which bunq needs for payers without a bunq account. The money arrives
in --account, by default the primary account.`,
		`bunqctl request create --to friend@example.com --amount 12.50 --description "Pizza"`,
		`bunqctl request create --to NL32BUNQ2025313705 --name "Sugar Daddy" --amount 5.00 --description "Tickets"`),

	"request list": read(`List the payment requests you sent from an account.

--json prints: id, date, status, amount, counterparty, description.`,
		`bunqctl request list`),

	"request revoke": write(`Withdraw a pending payment request. Requests that were already answered cannot
be revoked.`,
		`bunqctl request revoke 2470738`),

	"bunqme": group(`Shareable bunq.me payment links: anyone can pay, with or without bunq.`),

	"bunqme create": write(`Create a payment link and print its URL (--json prints the whole record).
The money arrives in --account, by default the primary account.`,
		`bunqctl bunqme create --amount 20.00 --description "Concert ticket"`),

	"bunqme list": read(`List the payment links of an account. Statuses include WAITING_FOR_PAYMENT and
CANCELLED.

--json prints: id, date, status, amount, description, url.`,
		`bunqctl bunqme list`),

	"bunqme cancel": write(`Close a payment link. Find its id with 'bunqme list'.`,
		`bunqctl bunqme cancel 19054`),

	// Sandbox
	"sandbox": group(`Helpers that exist only in the sandbox. With --env prod they exit with code 2.`),

	"sandbox fund": read(`Request test money from sugardaddy@bunq.com: 500.00 EUR by default, which is
also the maximum per request. bunq books it a few seconds later, so wait
before spending it.`,
		`bunqctl sandbox fund`, `bunqctl sandbox fund --amount 100.00`),

	"sandbox spend": read(`Make a direct test payment to create transactions. The default recipient is
sugardaddy@bunq.com. An IBAN needs --name. Sandbox only: in prod use 'pay
draft'.`,
		`bunqctl sandbox spend --amount 12.34`,
		`bunqctl sandbox spend --amount 5.00 --to NL32BUNQ2025313705 --name "Test Shop" --description Coffee`),

	// Help
	"docs": read(`Show the documentation that is built into this binary, so it works offline and
always matches the installed version.

Without a topic it lists the guides. With one it prints it as plain text; pipe
it to a pager for long ones.`,
		`bunqctl docs`, `bunqctl docs safety`, `bunqctl docs use-cases | less`),
}
