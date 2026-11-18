# bunqctl

A personal command-line client for the [bunq](https://www.bunq.com) API. Check balances, search and export transactions, download statements, pay through drafts, move money between your own accounts, and script all of it.

bunqctl is an unofficial tool and is not affiliated with bunq. It can move real money: read [Safety](#safety) first, and try everything in the sandbox.

- Go 1.27, macOS (secrets are kept in the macOS Keychain)
- One small binary, no runtime dependencies
- The sandbox is the default environment; production has to be asked for

## Quick start

```
go build -o bin/bunqctl ./cmd/bunqctl

# sandbox: fake money, nothing to lose
bunqctl auth init --permitted-ip "$(curl -fsS https://api.ipify.org)"
bunqctl sandbox fund
bunqctl accounts list
bunqctl tx list
```

`auth init` registers this machine with bunq and creates a sandbox user. It prints `initialized sandbox: user N, device N` when it works. For your real account see [Production](#production).

## What it can do

| Area | Commands |
|---|---|
| Setup | `auth init`, `auth status`, `auth session`, `auth logout`, `config show`, `config set max-amount` |
| Read | `user`, `accounts list/show/savings`, `tx list`, `events`, `insights`, `limits`, `cards list`, `categories list`, `auto-allocate list` |
| Export | `export` (JSON), `statement` (PDF, CSV, MT940), `annual-overview` (PDF) |
| Annotate | `tx note`, `tx notes`, `tx attach`, `categories create` |
| Pay | `pay draft`, `pay batch`, `pay status`, `pay cancel` (drafts you approve in the bunq app) |
| Own accounts | `move`, `sweep` (keep a floor, or save a share of income) |
| Standing orders | `schedule list/create/cancel` |
| Get paid | `request create/list/revoke`, `bunqme create/list/cancel` |
| Cards | `cards freeze`, `cards unfreeze` |
| Sandbox | `sandbox fund`, `sandbox spend` |

Every command has `--help` with a description, examples and a safety note. List commands take `--json` for scripts.

```
bunqctl tx list --from 2026-11-01 --search coffee
bunqctl insights
bunqctl export --account all --from 2026-01-01 --to 2026-12-31 --out 2026.json
bunqctl statement --format mt940 --from 2026-11-01 --to 2026-11-30 --out nov.mt940
bunqctl pay draft --to-iban NL32BUNQ2025313705 --to-name "Landlord" --amount 950.00 --description "Rent" --dry-run
bunqctl sweep --to 3851794 --percent 10 --since 2026-11-01 --until 2026-11-30 --dry-run
```

## Safety

- **Payments to other people are drafts.** bunqctl creates them and you approve them in the bunq app. It can never approve one.
- **`move` and `sweep` pay directly, but only between your own accounts.**
- **Every command that changes something has `--dry-run`** and asks you to confirm.
- **In production, commands that move money make you type the amount**, even with `--yes`. They cannot run unattended.
- **`max-amount`** refuses any money write above a limit you set, and an identical payment sent twice within 10 minutes is refused.
- **Cards can be frozen and unfrozen**, never blocked permanently.
- **Requests are signed, and every response is verified** against bunq's public key.
- **Secrets stay in the Keychain.** The API key is read from a hidden prompt or stdin, and no command prints a key or token.

`schedule create` is the exception to "you approve it": a standing order runs at bunq without any approval. See `bunqctl docs safety` for where the protection ends.

## Production

1. Create a standard API key in the bunq app: Profile, Security & Settings, Developers, API keys.
2. List every public IP you will use. A standard key cannot get new IPs later.
3. Register once, with the VPN connected, and set a payment ceiling:

```
bunqctl --env prod auth init --permitted-ip 203.0.113.10 --permitted-ip 203.0.113.11
bunqctl --env prod config set max-amount 500.00
bunqctl --env prod accounts list
```

bunq allows only about 10 setup calls per day, and `auth init` uses 2. Details: [docs/getting-started.txt](docs/getting-started.txt).

## Documentation

The guides are plain text, so they read well in a terminal. They are also built into the binary: `bunqctl docs` lists them and `bunqctl docs safety` prints one, offline.

| Guide | Content |
|---|---|
| [getting-started](docs/getting-started.txt) | Sandbox and production setup, where state is stored, maintenance |
| [commands](docs/commands.txt) | The command tree and a reference of every command |
| [use-cases](docs/use-cases.txt) | Recipes: balances, spending, bookkeeping, tax, bills, savings, receipts, cards |
| [safety](docs/safety.txt) | The write guard, what protects your money, where protection ends |
| [automation](docs/automation.txt) | Scripts and cron: what can run unattended, exit codes |
| [troubleshooting](docs/troubleshooting.txt) | Messages, causes and fixes |
| [api-notes](docs/api-notes.txt) | bunq endpoints used and how the real API behaves |
| [building](docs/building.txt) | Compile, release build, install, Docker |
| [architecture](docs/architecture.txt) | Code layout, how to add a command, how to write the docs |

## Build

```
go build -o bin/bunqctl ./cmd/bunqctl                                   # development build
go build -trimpath -ldflags "-s -w -X main.version=1.0.0" -o bin/bunqctl ./cmd/bunqctl
```

Or in Docker (no Go needed on the host). This builds the macOS binary for Apple Silicon:

```
docker build --target artifacts --output type=local,dest=dist \
  --build-arg GOOS=darwin --build-arg GOARCH=arm64 .
```

Universal binaries, installation and the other Docker targets are in [docs/building.txt](docs/building.txt).

## Tests

```
go vet ./... && go test -race ./...          # unit tests: no network, no Keychain
docker build --target test .                 # the same checks in a container, without -race
go test -tags integration -run Integration -v ./internal/cli/   # against the bunq sandbox
```

The integration test creates its own sandbox user with in-memory secrets and a temporary config directory, so your own state is untouched. It uses 2 of bunq's roughly 10 daily setup calls and never runs against production.

## Project layout

```
cmd/bunqctl/        entry point
internal/bunq/      API client: signing, verification, retries, one file per resource
internal/auth/      handshake and session renewal
internal/cli/       commands, the write guard, help texts
internal/profile/   per-environment state in ~/.config/bunqctl
internal/secret/    macOS Keychain
internal/output/    tables and JSON
docs/               the guides (plain text, embedded in the binary)
Dockerfile          test, build, artifacts and runtime stages
```

To add a command, see [docs/architecture.txt](docs/architecture.txt): one API method, one command, and a help entry that the tests insist on.

## Limitations

- macOS only. The code compiles elsewhere, but only macOS is built and tested.
- One personal user per environment: no business or joint accounts, no OAuth or PSD2 access to other people's accounts.
- No webhooks or other real-time features; bunqctl only talks to bunq when you run it.
- Payments are EUR.
- Approving a draft in the app, IPv6 in the permitted IPs, and unattended Keychain access were not tested. [docs/api-notes.txt](docs/api-notes.txt) lists what is unverified.

## License

MIT, see [LICENSE](LICENSE). Copyright (c) 2026 [Kisialeu](https://github.com/Kisialeu).
