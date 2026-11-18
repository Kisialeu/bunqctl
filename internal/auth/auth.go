// Package auth runs the bunq handshake (installation, device, session) and
// provides authenticated API clients that renew their session on demand.
package auth

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Kisialeu/bunqctl/internal/bunq"
	"github.com/Kisialeu/bunqctl/internal/profile"
	"github.com/Kisialeu/bunqctl/internal/secret"
)

// ErrNotInitialized means the handshake has not been completed.
var ErrNotInitialized = errors.New("not initialized: run 'bunqctl auth init'")

// deviceDescription is how the device shows up in the bunq app.
const deviceDescription = "bunqctl"

// Store holds secrets by name.
type Store interface {
	Get(name string) (string, error)
	Set(name, value string) error
	Delete(name string) error
}

// Manager owns the authentication state of one environment.
type Manager struct {
	dir       string
	prof      *profile.Profile
	store     Store
	userAgent string
	baseURL   string // the environment's URL; tests point it at a fake server
	logf      func(format string, args ...any)

	mu      sync.Mutex
	session string
}

// New returns a Manager. prof is updated and saved to dir as state changes.
func New(dir string, prof *profile.Profile, store Store, userAgent string, logf func(string, ...any)) *Manager {
	return &Manager{dir: dir, prof: prof, store: store, userAgent: userAgent, baseURL: prof.Env.BaseURL(), logf: logf}
}

// Profile returns the managed profile.
func (m *Manager) Profile() *profile.Profile { return m.prof }

func (m *Manager) config() bunq.Config {
	return bunq.Config{
		BaseURL:     m.baseURL,
		UserAgent:   m.userAgent,
		Region:      m.prof.Region,
		Geolocation: m.prof.Geolocation(),
		Logf:        m.logf,
	}
}

// PublicClient returns a client without authentication, e.g. for creating
// sandbox users.
func (m *Manager) PublicClient() *bunq.Client { return bunq.New(m.config()) }

// Init runs the full handshake for apiKey. It uses the setup endpoints, which
// bunq limits to about 10 calls per day.
func (m *Manager) Init(ctx context.Context, apiKey string) error {
	if err := profile.ValidateIPs(m.prof.PermittedIPs); err != nil {
		return err
	}

	key, err := bunq.GenerateKey()
	if err != nil {
		return err
	}
	pub, err := bunq.EncodePublicKey(&key.PublicKey)
	if err != nil {
		return err
	}
	inst, err := m.PublicClient().CreateInstallation(ctx, pub)
	if err != nil {
		return fmt.Errorf("installation: %w", err)
	}
	serverKey, err := bunq.DecodePublicKey(inst.ServerPublicKey)
	if err != nil {
		return fmt.Errorf("installation: %w", err)
	}

	priv, err := bunq.EncodePrivateKey(key)
	if err != nil {
		return err
	}
	for name, v := range map[string]string{
		secret.APIKey:            apiKey,
		secret.ClientPrivateKey:  priv,
		secret.InstallationToken: inst.Token,
	} {
		if err := m.store.Set(name, v); err != nil {
			return err
		}
	}
	m.prof.ServerPublicKey = inst.ServerPublicKey
	m.prof.InstallationID = inst.ID
	m.prof.DeviceID, m.prof.UserID = 0, 0
	if err := m.prof.Save(m.dir); err != nil {
		return err
	}

	deviceID, err := m.installationClient(key, serverKey, inst.Token).
		CreateDevice(ctx, deviceDescription, apiKey, m.prof.PermittedIPs)
	if err != nil {
		return fmt.Errorf("device registration: %w", err)
	}
	m.prof.DeviceID = deviceID
	if err := m.prof.Save(m.dir); err != nil {
		return err
	}

	_, err = m.Renew(ctx)
	return err
}

// Client returns an authenticated client that renews its session on a 401.
func (m *Manager) Client() (*bunq.Client, error) {
	if !m.prof.Initialized() {
		return nil, ErrNotInitialized
	}
	key, serverKey, err := m.keys()
	if err != nil {
		return nil, err
	}
	cfg := m.config()
	cfg.PrivateKey, cfg.ServerKey, cfg.Auth = key, serverKey, m
	return bunq.New(cfg), nil
}

// Token returns the session token. It implements bunq.TokenSource.
func (m *Manager) Token(context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.session != "" {
		return m.session, nil
	}
	t, err := m.store.Get(secret.SessionToken)
	if err != nil {
		return "", err
	}
	m.session = t
	return t, nil
}

// Renew opens a new session and stores it. It implements bunq.TokenSource.
func (m *Manager) Renew(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, serverKey, err := m.keys()
	if err != nil {
		return "", err
	}
	instToken, err := m.store.Get(secret.InstallationToken)
	if err != nil {
		return "", err
	}
	apiKey, err := m.store.Get(secret.APIKey)
	if err != nil {
		return "", err
	}

	s, err := m.installationClient(key, serverKey, instToken).CreateSession(ctx, apiKey)
	if err != nil {
		return "", fmt.Errorf("session: %w", err)
	}
	if s.UserType != "UserPerson" {
		return "", fmt.Errorf("session: user type %q is not supported, only personal accounts are", s.UserType)
	}
	if err := m.store.Set(secret.SessionToken, s.Token); err != nil {
		return "", err
	}
	m.session = s.Token
	m.prof.UserID = s.UserID
	m.prof.SessionCreated = time.Now().UTC()
	if err := m.prof.Save(m.dir); err != nil {
		return "", err
	}
	return s.Token, nil
}

// Logout removes all secrets and the profile of the environment.
func (m *Manager) Logout() error {
	for _, name := range secret.All {
		if err := m.store.Delete(name); err != nil {
			return err
		}
	}
	return profile.Delete(m.dir, m.prof.Env)
}

func (m *Manager) installationClient(key *rsa.PrivateKey, serverKey *rsa.PublicKey, token string) *bunq.Client {
	cfg := m.config()
	cfg.PrivateKey, cfg.ServerKey, cfg.Auth = key, serverKey, bunq.StaticToken(token)
	return bunq.New(cfg)
}

func (m *Manager) keys() (*rsa.PrivateKey, *rsa.PublicKey, error) {
	pem, err := m.store.Get(secret.ClientPrivateKey)
	if err != nil {
		return nil, nil, err
	}
	key, err := bunq.DecodePrivateKey(pem)
	if err != nil {
		return nil, nil, err
	}
	serverKey, err := bunq.DecodePublicKey(m.prof.ServerPublicKey)
	if err != nil {
		return nil, nil, err
	}
	return key, serverKey, nil
}
