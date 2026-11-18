// Package cli implements the bunqctl commands.
//
// Every command group lives in its own file and is registered in newRootCmd.
// Commands share the app value for flags, output, and authenticated clients.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/Kisialeu/bunqctl/internal/auth"
	"github.com/Kisialeu/bunqctl/internal/bunq"
	"github.com/Kisialeu/bunqctl/internal/output"
	"github.com/Kisialeu/bunqctl/internal/profile"
	"github.com/Kisialeu/bunqctl/internal/secret"
)

// Exit codes.
const (
	exitOK        = 0
	exitError     = 1
	exitUsage     = 2
	exitAuth      = 3
	exitCancelled = 4
)

var (
	errCancelled = errors.New("cancelled")
	errUsage     = errors.New("usage")
)

// app is the state shared by all commands.
type app struct {
	version string
	in      io.Reader
	out     io.Writer
	errOut  io.Writer
	store   func(profile.Env) auth.Store

	envName string
	json    bool
	verbose bool

	env         profile.Env
	dir         string
	started     bool // a command's RunE was reached
	setupFailed bool // the environment could not be prepared: not the user's usage mistake
}

func newApp(version string, in io.Reader, out, errOut io.Writer) *app {
	return &app{
		version: version, in: in, out: out, errOut: errOut,
		store: func(env profile.Env) auth.Store { return secret.NewKeychain(string(env)) },
	}
}

// Execute runs bunqctl with os.Args and returns the process exit code.
func Execute(ctx context.Context, version string) int {
	a := newApp(version, os.Stdin, os.Stdout, os.Stderr)
	err := a.run(ctx, os.Args[1:])
	if err != nil {
		fmt.Fprintln(a.errOut, "error:", err)
	}
	return exitCode(err)
}

// run executes one command line. Errors raised by cobra before a command
// runs (unknown command, missing required flag, wrong arguments) are usage
// errors.
func (a *app) run(ctx context.Context, args []string) error {
	root := newRootCmd(a)
	a.markStart(root)
	root.SetArgs(args)
	err := root.ExecuteContext(ctx)
	if err != nil && !a.started && !a.setupFailed && !errors.Is(err, errUsage) {
		return usageErr(err.Error())
	}
	return err
}

// markStart wraps every RunE in the tree to record that it was reached.
func (a *app) markStart(cmd *cobra.Command) {
	if run := cmd.RunE; run != nil {
		cmd.RunE = func(c *cobra.Command, args []string) error {
			a.started = true
			return run(c, args)
		}
	}
	for _, sub := range cmd.Commands() {
		a.markStart(sub)
	}
}

func exitCode(err error) int {
	switch {
	case err == nil:
		return exitOK
	case errors.Is(err, errUsage):
		return exitUsage
	case errors.Is(err, auth.ErrNotInitialized), errors.Is(err, secret.ErrNotFound):
		return exitAuth
	case errors.Is(err, errCancelled), errors.Is(err, context.Canceled):
		return exitCancelled
	default:
		return exitError
	}
}

func newRootCmd(a *app) *cobra.Command {
	root := &cobra.Command{
		Use:           "bunqctl",
		Short:         "Personal command-line client for the bunq API",
		Version:       a.version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(*cobra.Command, []string) error {
			return a.setup()
		},
	}
	root.SetIn(a.in)
	root.SetOut(a.out)
	root.SetErr(a.errOut)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usageErr(err.Error())
	})

	f := root.PersistentFlags()
	f.StringVar(&a.envName, "env", envDefault(), "environment: sandbox or prod (env BUNQCTL_ENV)")
	f.BoolVar(&a.json, "json", false, "print JSON instead of a table")
	f.BoolVarP(&a.verbose, "verbose", "v", false, "log API requests to stderr (no secrets)")

	root.AddCommand(
		newAuthCmd(a),
		newSandboxCmd(a),
		newUserCmd(a),
		newAccountsCmd(a),
		newTxCmd(a),
		newExportCmd(a),
		newEventsCmd(a),
		newInsightsCmd(a),
		newStatementCmd(a),
		newAnnualOverviewCmd(a),
		newLimitsCmd(a),
		newCardsCmd(a),
		newCategoriesCmd(a),
		newAutoAllocateCmd(a),
		newMoveCmd(a),
		newSweepCmd(a),
		newPayCmd(a),
		newScheduleCmd(a),
		newRequestCmd(a),
		newBunqMeCmd(a),
		newConfigCmd(a),
		newDocsCmd(a),
	)
	applyHelp(root)
	return root
}

func envDefault() string {
	if e := os.Getenv("BUNQCTL_ENV"); e != "" {
		return e
	}
	return string(profile.Sandbox)
}

func (a *app) setup() error {
	env, err := profile.ParseEnv(a.envName)
	if err != nil {
		return usageErr(err.Error())
	}
	dir, err := profile.Dir()
	if err != nil {
		a.setupFailed = true
		return err
	}
	a.env, a.dir = env, dir
	return nil
}

func (a *app) printer() output.Printer { return output.Printer{W: a.out, JSON: a.json} }

func (a *app) logf(format string, args ...any) {
	if a.verbose {
		fmt.Fprintf(a.errOut, "> "+format+"\n", args...)
	}
}

func (a *app) userAgent() string { return "bunqctl/" + a.version }

// manager returns the auth manager for an existing profile.
func (a *app) manager() (*auth.Manager, error) {
	prof, err := profile.Load(a.dir, a.env)
	if errors.Is(err, profile.ErrNotFound) {
		return nil, auth.ErrNotInitialized
	}
	if err != nil {
		return nil, err
	}
	return a.newManager(prof), nil
}

func (a *app) newManager(prof *profile.Profile) *auth.Manager {
	return auth.New(a.dir, prof, a.store(a.env), a.userAgent(), a.logf)
}

// session returns an authenticated client and the user id.
func (a *app) session() (*bunq.Client, int, error) {
	m, err := a.manager()
	if err != nil {
		return nil, 0, err
	}
	c, err := m.Client()
	if err != nil {
		return nil, 0, err
	}
	return c, m.Profile().UserID, nil
}

// requireEnv fails unless the selected environment is env.
func (a *app) requireEnv(env profile.Env) error {
	if a.env != env {
		return usageErr(fmt.Sprintf("this command needs --env %s", env))
	}
	return nil
}

// warnProduction prints a banner before a command that changes something.
func (a *app) warnProduction() {
	if a.env == profile.Production {
		fmt.Fprintln(a.errOut, "*** PRODUCTION ***")
	}
}

func usageErr(msg string) error { return fmt.Errorf("%w: %s", errUsage, msg) }
