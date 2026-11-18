package auth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/Kisialeu/bunqctl/internal/bunqtest"
	"github.com/Kisialeu/bunqctl/internal/profile"
	"github.com/Kisialeu/bunqctl/internal/secret"
)

const (
	testAPIKey    = "api-key"
	testInstToken = "installation-token"
	testUserID    = 42
	testDeviceID  = 10
)

// fakeAPI emulates the bunq handshake and one authenticated endpoint.
type fakeAPI struct {
	srv      *bunqtest.Server
	userType string

	mu       sync.Mutex
	sessions int
	current  string
}

func newFakeAPI(t *testing.T) *fakeAPI {
	f := &fakeAPI{userType: "UserPerson"}
	f.srv = bunqtest.New(t, f.handle(t))
	return f
}

func (f *fakeAPI) handle(t *testing.T) bunqtest.Handler {
	return func(r *http.Request, body []byte) (int, string) {
		f.mu.Lock()
		defer f.mu.Unlock()
		auth := r.Header.Get("X-Bunq-Client-Authentication")
		switch r.Method + " " + r.URL.Path {
		case "POST /v1/installation":
			var req struct {
				Key string `json:"client_public_key"`
			}
			if err := json.Unmarshal(body, &req); err != nil {
				t.Errorf("installation body: %v", err)
			}
			f.srv.VerifyRequestsWith(parsePublicKey(t, req.Key))
			key, _ := json.Marshal(f.srv.PublicKeyPEM())
			return 200, fmt.Sprintf(`{"Response":[{"Id":{"id":1}},{"Token":{"token":%q}},{"ServerPublicKey":{"server_public_key":%s}}]}`, testInstToken, key)
		case "POST /v1/device-server":
			var req struct {
				Secret string   `json:"secret"`
				IPs    []string `json:"permitted_ips"`
			}
			if err := json.Unmarshal(body, &req); err != nil {
				t.Errorf("device body: %v", err)
			}
			if auth != testInstToken || req.Secret != testAPIKey || len(req.IPs) == 0 {
				return 400, `{"Error":[{"error_description":"bad device request"}]}`
			}
			return 200, fmt.Sprintf(`{"Response":[{"Id":{"id":%d}}]}`, testDeviceID)
		case "POST /v1/session-server":
			if auth != testInstToken {
				return 401, `{"Error":[{"error_description":"bad installation token"}]}`
			}
			f.sessions++
			f.current = fmt.Sprintf("session-%d", f.sessions)
			return 200, fmt.Sprintf(`{"Response":[{"Id":{"id":%d}},{"Token":{"token":%q}},{%q:{"id":%d}}]}`,
				f.sessions, f.current, f.userType, testUserID)
		case fmt.Sprintf("GET /v1/user/%d", testUserID):
			if auth != f.current {
				return 403, `{"Error":[{"error_description":"Insufficient authentication."}]}`
			}
			return 200, fmt.Sprintf(`{"Response":[{"UserPerson":{"id":%d,"display_name":"Test"}}]}`, testUserID)
		}
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		return 404, `{"Error":[{"error_description":"not found"}]}`
	}
}

func parsePublicKey(t *testing.T, s string) *rsa.PublicKey {
	t.Helper()
	block, _ := pem.Decode([]byte(s))
	if block == nil {
		t.Fatal("client public key is not PEM")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return key.(*rsa.PublicKey)
}

func newTestManager(f *fakeAPI, dir string, store Store) *Manager {
	prof := &profile.Profile{Env: profile.Sandbox, PermittedIPs: []string{"203.0.113.1"}}
	m := New(dir, prof, store, "test", nil)
	m.baseURL = f.srv.URL
	return m
}

// reload returns a fresh Manager over the saved state, like a new process.
func reload(t *testing.T, f *fakeAPI, dir string, store Store) *Manager {
	t.Helper()
	prof, err := profile.Load(dir, profile.Sandbox)
	if err != nil {
		t.Fatal(err)
	}
	m := New(dir, prof, store, "test", nil)
	m.baseURL = f.srv.URL
	return m
}

func Test_Init_storesStateAndAuthenticates(t *testing.T) {
	f, dir, store := newFakeAPI(t), t.TempDir(), &secret.Memory{}
	if err := newTestManager(f, dir, store).Init(context.Background(), testAPIKey); err != nil {
		t.Fatal(err)
	}

	for _, name := range secret.All {
		if v, err := store.Get(name); err != nil || v == "" {
			t.Errorf("secret %s not stored: %v", name, err)
		}
	}
	m := reload(t, f, dir, store)
	if p := m.Profile(); p.UserID != testUserID || p.DeviceID != testDeviceID || p.ServerPublicKey == "" {
		t.Errorf("profile = %+v", p)
	}

	c, err := m.Client()
	if err != nil {
		t.Fatal(err)
	}
	u, err := c.User(context.Background(), testUserID)
	if err != nil {
		t.Fatal(err)
	}
	if u.DisplayName != "Test" {
		t.Errorf("user = %+v", u)
	}
}

func Test_Client_renewsInvalidSession(t *testing.T) {
	f, dir, store := newFakeAPI(t), t.TempDir(), &secret.Memory{}
	if err := newTestManager(f, dir, store).Init(context.Background(), testAPIKey); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(secret.SessionToken, "invalid"); err != nil {
		t.Fatal(err)
	}

	c, err := reload(t, f, dir, store).Client()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.User(context.Background(), testUserID); err != nil {
		t.Fatalf("request after renewal: %v", err)
	}
	if f.sessions != 2 {
		t.Errorf("sessions = %d, want 2 (init + renewal)", f.sessions)
	}
	if got, _ := store.Get(secret.SessionToken); got != f.current {
		t.Errorf("stored session %q, want %q", got, f.current)
	}
}

func Test_Init_rejectsMissingIPs(t *testing.T) {
	f := newFakeAPI(t)
	m := newTestManager(f, t.TempDir(), &secret.Memory{})
	m.prof.PermittedIPs = nil
	if err := m.Init(context.Background(), testAPIKey); err == nil {
		t.Fatal("Init succeeded without permitted IPs")
	}
}

func Test_Init_rejectsCompanyUser(t *testing.T) {
	f := newFakeAPI(t)
	f.userType = "UserCompany"
	err := newTestManager(f, t.TempDir(), &secret.Memory{}).Init(context.Background(), testAPIKey)
	if err == nil {
		t.Fatal("Init accepted a company user")
	}
}

func Test_Client_requiresInit(t *testing.T) {
	m := newTestManager(newFakeAPI(t), t.TempDir(), &secret.Memory{})
	if _, err := m.Client(); !errors.Is(err, ErrNotInitialized) {
		t.Errorf("err = %v, want ErrNotInitialized", err)
	}
}

func Test_Logout_removesEverything(t *testing.T) {
	f, dir, store := newFakeAPI(t), t.TempDir(), &secret.Memory{}
	m := newTestManager(f, dir, store)
	if err := m.Init(context.Background(), testAPIKey); err != nil {
		t.Fatal(err)
	}
	if err := m.Logout(); err != nil {
		t.Fatal(err)
	}
	for _, name := range secret.All {
		if _, err := store.Get(name); !errors.Is(err, secret.ErrNotFound) {
			t.Errorf("secret %s still stored", name)
		}
	}
	if _, err := profile.Load(dir, profile.Sandbox); !errors.Is(err, profile.ErrNotFound) {
		t.Errorf("profile still present: %v", err)
	}
}
