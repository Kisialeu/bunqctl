// Package profile stores the non-secret settings and state of one bunq
// environment in ~/.config/bunqctl/<env>.json.
package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/Kisialeu/bunqctl/internal/atomicfile"
	"github.com/Kisialeu/bunqctl/internal/bunq"
)

// Env is a bunq environment.
type Env string

// Supported environments.
const (
	Sandbox    Env = "sandbox"
	Production Env = "prod"
)

// ParseEnv validates an environment name.
func ParseEnv(s string) (Env, error) {
	switch e := Env(s); e {
	case Sandbox, Production:
		return e, nil
	default:
		return "", fmt.Errorf("unknown environment %q (want %s or %s)", s, Sandbox, Production)
	}
}

// BaseURL returns the API host of the environment.
func (e Env) BaseURL() string {
	if e == Production {
		return bunq.ProductionURL
	}
	return bunq.SandboxURL
}

// ErrNotFound means the environment has not been initialized.
var ErrNotFound = errors.New("profile not found")

// Profile is the persisted, non-secret state of one environment.
type Profile struct {
	Env          Env      `json:"env"`
	PermittedIPs []string `json:"permitted_ips"`
	Region       string   `json:"region,omitempty"`     // X-Bunq-Region, default en_US
	Country      string   `json:"country,omitempty"`    // geolocation country, default NL
	MaxAmount    string   `json:"max_amount,omitempty"` // refuse money writes above this, e.g. "500.00"

	ServerPublicKey string    `json:"server_public_key,omitempty"`
	InstallationID  int       `json:"installation_id,omitempty"`
	DeviceID        int       `json:"device_id,omitempty"`
	UserID          int       `json:"user_id,omitempty"`
	SessionCreated  time.Time `json:"session_created,omitzero"`
}

// Initialized reports whether the handshake has completed.
func (p *Profile) Initialized() bool { return p.DeviceID != 0 && p.UserID != 0 }

// Geolocation returns the zero-valued X-Bunq-Geolocation header value.
func (p *Profile) Geolocation() string {
	country := p.Country
	if country == "" {
		country = "NL"
	}
	return "0 0 0 0 " + country
}

// ValidateIPs checks that PermittedIPs is a non-empty list of IPv4 addresses.
func ValidateIPs(ips []string) error {
	if len(ips) == 0 {
		return errors.New("no permitted IPs: a standard API key cannot get new IPs later, list all of them")
	}
	for _, s := range ips {
		ip := net.ParseIP(s)
		if ip == nil || ip.To4() == nil {
			return fmt.Errorf("permitted IP %q is not an IPv4 address", s)
		}
	}
	return nil
}

// Dir returns the configuration directory, honouring XDG_CONFIG_HOME.
func Dir() (string, error) {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "bunqctl"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home directory (set HOME or XDG_CONFIG_HOME): %w", err)
	}
	return filepath.Join(home, ".config", "bunqctl"), nil
}

// Path returns the profile file of env inside dir.
func Path(dir string, env Env) string {
	return filepath.Join(dir, string(env)+".json")
}

// Load reads the profile of env. It returns ErrNotFound if there is none.
func Load(dir string, env Env) (*Profile, error) {
	b, err := os.ReadFile(Path(dir, env))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read profile: %w", err)
	}
	var p Profile
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("decode profile %s: %w", Path(dir, env), err)
	}
	return &p, nil
}

// Save writes the profile atomically with mode 0600.
func (p *Profile) Save(dir string) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("encode profile: %w", err)
	}
	return atomicfile.Write(Path(dir, p.Env), append(b, '\n'))
}

// Delete removes the profile of env. A missing profile is not an error.
func Delete(dir string, env Env) error {
	err := os.Remove(Path(dir, env))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("delete profile: %w", err)
	}
	return nil
}
