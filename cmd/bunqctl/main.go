// Command bunqctl is a personal command-line client for the bunq API.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/Kisialeu/bunqctl/internal/cli"
)

// Set at build time: -ldflags "-X main.version=1.2.3 -X main.commit=abc1234".
var (
	version = "dev"
	commit  = ""
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := cli.Execute(ctx, versionString())
	stop()
	os.Exit(code)
}

// versionString is what --version prints: the version, plus the commit when
// the build recorded one ("unknown" means it could not).
func versionString() string {
	if commit == "" || commit == "unknown" {
		return version
	}
	return version + " (" + commit + ")"
}
