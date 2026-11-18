package bunq

import (
	"context"
	"encoding/json"
	"net/http"
)

// Installation is the result of registering a client public key.
type Installation struct {
	ID              int
	Token           string
	ServerPublicKey string
}

// CreateInstallation registers the client public key (PEM).
func (c *Client) CreateInstallation(ctx context.Context, publicKeyPEM string) (Installation, error) {
	env, err := c.do(ctx, http.MethodPost, apiPath("installation"), nil,
		map[string]string{"client_public_key": publicKeyPEM})
	if err != nil {
		return Installation{}, err
	}
	id, err := createdID(env)
	if err != nil {
		return Installation{}, err
	}
	token, err := first[struct {
		Token string `json:"token"`
	}](env, "Token")
	if err != nil {
		return Installation{}, err
	}
	key, err := first[struct {
		Key string `json:"server_public_key"`
	}](env, "ServerPublicKey")
	if err != nil {
		return Installation{}, err
	}
	return Installation{ID: id, Token: token.Token, ServerPublicKey: key.Key}, nil
}

// CreateDevice binds apiKey to this installation and the permitted IPs.
func (c *Client) CreateDevice(ctx context.Context, description, apiKey string, permittedIPs []string) (int, error) {
	return create(ctx, c, apiPath("device-server"), map[string]any{
		"description":   description,
		"secret":        apiKey,
		"permitted_ips": permittedIPs,
	})
}

// Session is an authenticated API session.
type Session struct {
	ID       int
	Token    string
	UserID   int
	UserType string // UserPerson, UserCompany, UserApiKey or UserPaymentServiceProvider
}

// CreateSession opens a session for apiKey.
func (c *Client) CreateSession(ctx context.Context, apiKey string) (Session, error) {
	env, err := c.do(ctx, http.MethodPost, apiPath("session-server"), nil,
		map[string]string{"secret": apiKey})
	if err != nil {
		return Session{}, err
	}
	id, err := createdID(env)
	if err != nil {
		return Session{}, err
	}
	token, err := first[struct {
		Token string `json:"token"`
	}](env, "Token")
	if err != nil {
		return Session{}, err
	}
	s := Session{ID: id, Token: token.Token}

	items, err := env.items()
	if err != nil {
		return Session{}, err
	}
	for _, it := range items {
		switch it.Type {
		case "UserPerson", "UserCompany", "UserApiKey", "UserPaymentServiceProvider":
			var u struct {
				ID int `json:"id"`
			}
			if err := json.Unmarshal(it.Body, &u); err != nil {
				return Session{}, err
			}
			s.UserID, s.UserType = u.ID, it.Type
		}
	}
	return s, nil
}

// CreateSandboxUser creates a sandbox person and returns its API key.
// It works only against the sandbox environment.
func (c *Client) CreateSandboxUser(ctx context.Context) (string, error) {
	env, err := c.do(ctx, http.MethodPost, apiPath("sandbox-user-person"), nil, struct{}{})
	if err != nil {
		return "", err
	}
	key, err := first[struct {
		APIKey string `json:"api_key"`
	}](env, "ApiKey")
	return key.APIKey, err
}
