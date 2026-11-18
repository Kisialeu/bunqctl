// Package secret keeps bunqctl secrets in the macOS Keychain.
package secret

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

// Names of the stored secrets.
const (
	APIKey            = "api_key"
	ClientPrivateKey  = "client_private_key"
	InstallationToken = "installation_token"
	SessionToken      = "session_token"
)

// All lists every secret name, e.g. for logout.
var All = []string{APIKey, ClientPrivateKey, InstallationToken, SessionToken}

// ErrNotFound means the secret is not stored.
var ErrNotFound = errors.New("secret not found")

// Keychain stores secrets under one Keychain service.
type Keychain struct {
	service string
}

// NewKeychain returns a store for the service "bunqctl.<env>".
func NewKeychain(env string) Keychain {
	return Keychain{service: "bunqctl." + env}
}

// Get returns the secret name.
func (k Keychain) Get(name string) (string, error) {
	v, err := keyring.Get(k.service, name)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", fmt.Errorf("%s: %w", name, ErrNotFound)
	}
	if err != nil {
		return "", fmt.Errorf("keychain get %s: %w", name, err)
	}
	return v, nil
}

// Set stores the secret name.
func (k Keychain) Set(name, value string) error {
	if err := keyring.Set(k.service, name, value); err != nil {
		return fmt.Errorf("keychain set %s: %w", name, err)
	}
	return nil
}

// Delete removes the secret name. A missing secret is not an error.
func (k Keychain) Delete(name string) error {
	err := keyring.Delete(k.service, name)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("keychain delete %s: %w", name, err)
	}
	return nil
}
