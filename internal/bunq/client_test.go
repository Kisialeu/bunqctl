package bunq

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/Kisialeu/bunqctl/internal/bunqtest"
)

// clientKey is generated once: RSA generation is slow.
var clientKey = sync.OnceValue(func() *rsa.PrivateKey {
	key, err := GenerateKey()
	if err != nil {
		panic(err)
	}
	return key
})

// fakeBunq starts a signing fake server that also verifies request signatures.
func fakeBunq(t *testing.T, h bunqtest.Handler) *bunqtest.Server {
	t.Helper()
	srv := bunqtest.New(t, h)
	srv.VerifyRequestsWith(&clientKey().PublicKey)
	return srv
}

func newTestClient(srv *bunqtest.Server, auth TokenSource) *Client {
	c := New(Config{
		BaseURL: srv.URL, UserAgent: "test",
		PrivateKey: clientKey(), ServerKey: &srv.Key.PublicKey, Auth: auth,
	})
	c.limiter = newLimiter(0)
	return c
}

type fakeTokens struct {
	token   string
	renewed int
}

func (f *fakeTokens) Token(context.Context) (string, error) { return f.token, nil }
func (f *fakeTokens) Renew(context.Context) (string, error) {
	f.renewed++
	f.token = fmt.Sprintf("session-%d", f.renewed)
	return f.token, nil
}

func Test_sign_verifyRoundTrip(t *testing.T) {
	key := clientKey()
	body := []byte(`{"a":1}`)
	sig, err := sign(key, body)
	if err != nil {
		t.Fatal(err)
	}
	if err := verify(&key.PublicKey, body, sig); err != nil {
		t.Errorf("verify own signature: %v", err)
	}
	if err := verify(&key.PublicKey, []byte(`{"a":2}`), sig); err == nil {
		t.Error("verify accepted a tampered body")
	}
}

func Test_keys_pemRoundTrip(t *testing.T) {
	key := clientKey()
	priv, err := EncodePrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodePrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(key) {
		t.Error("private key changed in round trip")
	}
	pub, err := EncodePublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	gotPub, err := DecodePublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	if !gotPub.Equal(&key.PublicKey) {
		t.Error("public key changed in round trip")
	}
}

func Test_do_sendsHeadersAndVerifiesResponse(t *testing.T) {
	srv := fakeBunq(t, func(r *http.Request, _ []byte) (int, string) {
		for _, h := range []string{"User-Agent", "Cache-Control", headerRequestID, headerRegion, headerGeolocation, headerAuth} {
			if r.Header.Get(h) == "" {
				t.Errorf("missing header %s", h)
			}
		}
		return 200, `{"Response":[{"UserPerson":{"id":7,"display_name":"Test"}}]}`
	})
	u, err := newTestClient(srv, StaticToken("t")).User(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != 7 || u.DisplayName != "Test" {
		t.Errorf("got %+v", u)
	}
}

func Test_do_rejectsBadServerSignature(t *testing.T) {
	srv := bunqtest.New(t, func(*http.Request, []byte) (int, string) {
		return 200, `{"Response":[{"UserPerson":{"id":1}}]}`
	})
	c := newTestClient(srv, nil)
	c.cfg.ServerKey = &clientKey().PublicKey // a key that did not sign the response
	if _, err := c.User(context.Background(), 1); err == nil {
		t.Fatal("accepted a response with a bad signature")
	}
}

func Test_do_renewsSessionOnce(t *testing.T) {
	for _, status := range []int{401, 403} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			srv := fakeBunq(t, func(r *http.Request, _ []byte) (int, string) {
				if r.Header.Get(headerAuth) == "expired" {
					return status, `{"Error":[{"error_description":"Insufficient authentication."}]}`
				}
				return 200, `{"Response":[{"UserPerson":{"id":1}}]}`
			})
			tokens := &fakeTokens{token: "expired"}
			if _, err := newTestClient(srv, tokens).User(context.Background(), 1); err != nil {
				t.Fatal(err)
			}
			if tokens.renewed != 1 {
				t.Errorf("renewed %d times, want 1", tokens.renewed)
			}
		})
	}
}

func Test_do_stopsAfterSecond401(t *testing.T) {
	srv := fakeBunq(t, func(*http.Request, []byte) (int, string) {
		return 401, `{"Error":[{"error_description":"nope"}]}`
	})
	tokens := &fakeTokens{token: "x"}
	_, err := newTestClient(srv, tokens).User(context.Background(), 1)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 401 {
		t.Fatalf("got %v, want 401 APIError", err)
	}
	if tokens.renewed != 1 {
		t.Errorf("renewed %d times, want 1", tokens.renewed)
	}
}

func Test_do_retriesAfter429(t *testing.T) {
	calls := 0
	srv := fakeBunq(t, func(*http.Request, []byte) (int, string) {
		calls++
		if calls == 1 {
			return 429, `{"Error":[{"error_description":"Too many requests. You can do a maximum of 100 calls per 1 second to this endpoint."}]}`
		}
		return 200, `{"Response":[{"UserPerson":{"id":1}}]}`
	})
	if _, err := newTestClient(srv, nil).User(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func Test_backoff(t *testing.T) {
	limited := &APIError{Messages: []string{"You can do a maximum of 3 calls per 3 seconds to this endpoint."}}
	tests := []struct {
		name    string
		err     *APIError
		attempt int
		want    time.Duration
	}{
		{"parsed limit", limited, 0, time.Second},
		{"doubles", limited, 2, 4 * time.Second},
		{"capped", limited, 10, maxBackoff},
		{"unknown message", &APIError{}, 0, defaultRateWindow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := backoff(tt.err, tt.attempt); got != tt.want {
				t.Errorf("backoff = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_list_followsOlderURL(t *testing.T) {
	srv := fakeBunq(t, func(r *http.Request, _ []byte) (int, string) {
		if r.URL.Query().Get("count") != "200" {
			t.Errorf("count = %q, want 200", r.URL.Query().Get("count"))
		}
		if r.URL.Query().Get("older_id") == "" {
			return 200, `{"Response":[{"MonetaryAccountBank":{"id":2}},{"MonetaryAccountSavings":{"id":1}}],
				"Pagination":{"older_url":"/v1/user/1/monetary-account?count=200&older_id=1"}}`
		}
		return 200, `{"Response":[{"MonetaryAccountJoint":{"id":0}}],"Pagination":{"older_url":null}}`
	})
	got, err := collectAll(newTestClient(srv, nil).MonetaryAccounts(context.Background(), 1))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"MonetaryAccountBank", "MonetaryAccountSavings", "MonetaryAccountJoint"}
	if len(got) != len(want) {
		t.Fatalf("got %d accounts, want %d", len(got), len(want))
	}
	for i, a := range got {
		if a.Type != want[i] {
			t.Errorf("account %d type = %s, want %s", i, a.Type, want[i])
		}
	}
}

func Test_Pointers_objectOrArray(t *testing.T) {
	for _, in := range []string{
		`{"type":"IBAN","value":"NL01"}`,
		`[{"type":"EMAIL","value":"a@b"},{"type":"IBAN","value":"NL01"}]`,
	} {
		var p Pointers
		if err := json.Unmarshal([]byte(in), &p); err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got := p.Value("IBAN"); got != "NL01" {
			t.Errorf("%s: IBAN = %q", in, got)
		}
	}
}

func collectAll[T any](seq func(func(T, error) bool)) ([]T, error) {
	var out []T
	for v, err := range seq {
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}
