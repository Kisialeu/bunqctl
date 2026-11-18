package cli

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Kisialeu/bunqctl/internal/profile"
	"github.com/Kisialeu/bunqctl/internal/publicip"
)

func newAuthCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Manage authentication with bunq"}
	cmd.AddCommand(newAuthInitCmd(a), newAuthStatusCmd(a), newAuthSessionCmd(a), newAuthLogoutCmd(a))
	return cmd
}

func newAuthInitCmd(a *app) *cobra.Command {
	var (
		ips         []string
		region      string
		country     string
		apiKeyStdin bool
		force       bool
		skipIPCheck bool
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Register this machine with bunq (installation, device, session)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			prof, err := profile.Load(a.dir, a.env)
			switch {
			case errors.Is(err, profile.ErrNotFound):
				prof = &profile.Profile{Env: a.env}
			case err != nil:
				return err
			case !force:
				return fmt.Errorf("%s is already initialized: use --force to start over", a.env)
			}
			if len(ips) > 0 {
				prof.PermittedIPs = ips
			}
			if region != "" {
				prof.Region = region
			}
			if country != "" {
				prof.Country = country
			}
			if err := profile.ValidateIPs(prof.PermittedIPs); err != nil {
				return usageErr(err.Error() + " (--permitted-ip)")
			}
			if !skipIPCheck {
				ip, err := publicip.Lookup(ctx)
				if err != nil {
					return fmt.Errorf("%w (use --skip-ip-check to skip)", err)
				}
				if !slices.Contains(prof.PermittedIPs, ip) {
					return fmt.Errorf("current IP %s is not in the permitted IPs: check the VPN", ip)
				}
			}

			a.warnProduction()
			if force {
				if err := a.confirm(fmt.Sprintf("Start over and register a new device for %s?", a.env), ""); err != nil {
					return err
				}
			}

			m := a.newManager(prof)
			var apiKey string
			if a.env == profile.Sandbox {
				if apiKey, err = m.PublicClient().CreateSandboxUser(ctx); err != nil {
					return fmt.Errorf("create sandbox user: %w", err)
				}
			} else if apiKey, err = a.readAPIKey(apiKeyStdin); err != nil {
				return err
			}

			if err := m.Init(ctx, apiKey); err != nil {
				return err
			}
			fmt.Fprintf(a.errOut, "initialized %s: user %d, device %d\n", a.env, prof.UserID, prof.DeviceID)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringSliceVar(&ips, "permitted-ip", nil, "IPv4 address the API key may be used from (repeatable)")
	f.StringVar(&region, "region", "", "X-Bunq-Region, e.g. nl_NL (default en_US)")
	f.StringVar(&country, "country", "", "ISO country code for X-Bunq-Geolocation (default NL)")
	f.BoolVar(&apiKeyStdin, "api-key-stdin", false, "read the API key from stdin (production)")
	f.BoolVar(&force, "force", false, "re-initialize an existing environment")
	f.BoolVar(&skipIPCheck, "skip-ip-check", false, "do not check the current public IP")
	return cmd
}

func (a *app) readAPIKey(fromStdin bool) (string, error) {
	var key string
	if fromStdin {
		b, err := io.ReadAll(io.LimitReader(a.in, 4096))
		if err != nil {
			return "", fmt.Errorf("read api key: %w", err)
		}
		key = strings.TrimSpace(string(b))
	} else {
		var err error
		if key, err = a.readSecret("bunq API key: "); err != nil {
			return "", err
		}
	}
	if key == "" {
		return "", usageErr("empty API key")
	}
	return key, nil
}

func newAuthStatusCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the authentication state (no secrets)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			m, err := a.manager()
			if err != nil {
				return err
			}
			p := m.Profile()
			ip, err := publicip.Lookup(cmd.Context())
			if err != nil {
				ip = "unknown (" + err.Error() + ")"
			}
			status := struct {
				Env            profile.Env `json:"env"`
				UserID         int         `json:"user_id"`
				DeviceID       int         `json:"device_id"`
				PermittedIPs   []string    `json:"permitted_ips"`
				CurrentIP      string      `json:"current_ip"`
				SessionCreated time.Time   `json:"session_created"`
			}{p.Env, p.UserID, p.DeviceID, p.PermittedIPs, ip, p.SessionCreated}

			return a.printer().Fields(status, [][2]string{
				{"Environment", string(p.Env)},
				{"User", strconv.Itoa(p.UserID)},
				{"Device", strconv.Itoa(p.DeviceID)},
				{"Permitted IPs", strings.Join(p.PermittedIPs, ", ")},
				{"Current IP", ip},
				{"Session age", time.Since(p.SessionCreated).Round(time.Second).String()},
			})
		},
	}
}

func newAuthSessionCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "session",
		Short: "Open a new session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			m, err := a.manager()
			if err != nil {
				return err
			}
			if _, err := m.Renew(cmd.Context()); err != nil {
				return err
			}
			fmt.Fprintln(a.errOut, "new session opened")
			return nil
		},
	}
}

func newAuthLogoutCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Delete the stored keys, tokens and profile of the environment",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			m, err := a.manager()
			if err != nil {
				return err
			}
			a.warnProduction()
			if err := a.confirm(fmt.Sprintf("Delete all bunqctl state of %s?", a.env), ""); err != nil {
				return err
			}
			if err := m.Logout(); err != nil {
				return err
			}
			fmt.Fprintf(a.errOut, "logged out of %s\n", a.env)
			return nil
		},
	}
}
