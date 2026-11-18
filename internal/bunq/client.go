// Package bunq is a small client for the bunq public API.
//
// The transport (headers, signing, response verification, retries) lives in
// this file. Each API resource lives in its own file and is built on the
// generic helpers in response.go, so adding an endpoint is usually one method.
package bunq

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Base URLs of the bunq environments.
const (
	SandboxURL    = "https://public-api.sandbox.bunq.com"
	ProductionURL = "https://api.bunq.com"
)

const (
	defaultTimeout    = 30 * time.Second
	defaultInterval   = 500 * time.Millisecond // 60 requests per 30 seconds
	maxRateRetries    = 5
	maxResponseBytes  = 16 << 20
	headerAuth        = "X-Bunq-Client-Authentication"
	headerSignature   = "X-Bunq-Client-Signature"
	headerServerSig   = "X-Bunq-Server-Signature"
	headerRequestID   = "X-Bunq-Client-Request-Id"
	headerResponseID  = "X-Bunq-Client-Response-Id"
	headerLanguage    = "X-Bunq-Language"
	headerRegion      = "X-Bunq-Region"
	headerGeolocation = "X-Bunq-Geolocation"
)

// TokenSource provides the value of the X-Bunq-Client-Authentication header.
type TokenSource interface {
	// Token returns the current token.
	Token(ctx context.Context) (string, error)
	// Renew replaces an expired token. It is called once after a 401 or 403.
	Renew(ctx context.Context) (string, error)
}

// StaticToken is a TokenSource that never renews, used for the installation token.
type StaticToken string

// Token returns the token itself.
func (t StaticToken) Token(context.Context) (string, error) { return string(t), nil }

// Renew always fails: an installation token cannot be renewed.
func (t StaticToken) Renew(context.Context) (string, error) {
	return "", errors.New("installation token cannot be renewed")
}

// Config configures a Client. Only BaseURL and UserAgent are required.
type Config struct {
	BaseURL     string
	UserAgent   string
	Region      string // e.g. "en_US"
	Geolocation string // e.g. "0 0 0 0 NL"

	PrivateKey *rsa.PrivateKey // signs request bodies; nil disables signing
	ServerKey  *rsa.PublicKey  // verifies response bodies; nil disables verification
	Auth       TokenSource     // nil sends no authentication header

	HTTPClient *http.Client                     // defaults to a client with a 30s timeout
	Logf       func(format string, args ...any) // request log without secrets; nil disables
}

// Client calls the bunq API. It is safe for concurrent use.
type Client struct {
	cfg     Config
	http    *http.Client
	limiter *limiter
}

// New returns a Client for cfg.
func New(cfg Config) *Client {
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: defaultTimeout}
	}
	if cfg.Region == "" {
		cfg.Region = "en_US"
	}
	if cfg.Geolocation == "" {
		cfg.Geolocation = "0 0 0 0 NL"
	}
	return &Client{cfg: cfg, http: hc, limiter: newLimiter(defaultInterval)}
}

// request is one API call. body is sent as is; contentType defaults to JSON.
type request struct {
	method      string
	path        string // absolute from the host, e.g. "/v1/user/1"
	query       url.Values
	body        []byte
	contentType string
	header      map[string]string // extra headers
}

// do sends a JSON request and returns the decoded envelope. in may be nil.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, in any) (*envelope, error) {
	r := request{method: method, path: path, query: query}
	if in != nil {
		var err error
		if r.body, err = json.Marshal(in); err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
	}
	raw, err := c.roundTrip(ctx, r)
	if err != nil {
		return nil, err
	}
	return decodeEnvelope(raw)
}

// roundTrip sends r with retries and returns the verified response body.
func (c *Client) roundTrip(ctx context.Context, r request) ([]byte, error) {
	renewed := false
	for attempt := 0; ; attempt++ {
		raw, resp, err := c.send(ctx, r)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 300 {
			if err := c.verify(resp, raw); err != nil {
				return nil, err
			}
			return raw, nil
		}

		apiErr := newAPIError(resp, raw)
		switch {
		case authFailed(resp.StatusCode) && !renewed && c.cfg.Auth != nil:
			renewed = true
			if _, err := c.cfg.Auth.Renew(ctx); err != nil {
				return nil, fmt.Errorf("%w (renew session: %v)", apiErr, err)
			}
		case resp.StatusCode == http.StatusTooManyRequests && attempt < maxRateRetries:
			if err := sleep(ctx, backoff(apiErr, attempt)); err != nil {
				return nil, err
			}
		default:
			return nil, apiErr
		}
	}
}

func (c *Client) send(ctx context.Context, r request) ([]byte, *http.Response, error) {
	if err := c.limiter.wait(ctx); err != nil {
		return nil, nil, err
	}

	u := c.cfg.BaseURL + r.path
	if len(r.query) > 0 {
		u += "?" + r.query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, r.method, u, bytes.NewReader(r.body))
	if err != nil {
		return nil, nil, fmt.Errorf("build request: %w", err)
	}
	if err := c.setHeaders(ctx, req, r); err != nil {
		return nil, nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("%s %s: %w", r.method, r.path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("%s %s: read response: %w", r.method, r.path, err)
	}
	c.logf("%s %s -> %d (response id %s)", r.method, r.path, resp.StatusCode, resp.Header.Get(headerResponseID))
	return raw, resp, nil
}

func (c *Client) setHeaders(ctx context.Context, req *http.Request, r request) error {
	h := req.Header
	h.Set("User-Agent", c.cfg.UserAgent)
	h.Set("Cache-Control", "no-cache")
	h.Set(headerLanguage, "en_US")
	h.Set(headerRegion, c.cfg.Region)
	h.Set(headerGeolocation, c.cfg.Geolocation)
	h.Set(headerRequestID, newRequestID())
	if len(r.body) > 0 {
		ct := r.contentType
		if ct == "" {
			ct = "application/json"
		}
		h.Set("Content-Type", ct)
	}
	for k, v := range r.header {
		h.Set(k, v)
	}
	if c.cfg.Auth != nil {
		token, err := c.cfg.Auth.Token(ctx)
		if err != nil {
			return fmt.Errorf("authentication token: %w", err)
		}
		h.Set(headerAuth, token)
	}
	if c.cfg.PrivateKey != nil && len(r.body) > 0 {
		sig, err := sign(c.cfg.PrivateKey, r.body)
		if err != nil {
			return err
		}
		h.Set(headerSignature, sig)
	}
	return nil
}

func (c *Client) verify(resp *http.Response, raw []byte) error {
	if c.cfg.ServerKey == nil {
		return nil
	}
	sig := resp.Header.Get(headerServerSig)
	if sig == "" {
		return errors.New("response is not signed by the server")
	}
	if err := verify(c.cfg.ServerKey, raw, sig); err != nil {
		return fmt.Errorf("response signature: %w", err)
	}
	return nil
}

func (c *Client) logf(format string, args ...any) {
	if c.cfg.Logf != nil {
		c.cfg.Logf(format, args...)
	}
}

// authFailed reports whether a status may mean an expired session. bunq
// answers an invalid session token with 403 "Insufficient authentication",
// not 401, so both trigger one renewal.
func authFailed(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// newRequestID returns a random UUID v4.
func newRequestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never returns an error
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
