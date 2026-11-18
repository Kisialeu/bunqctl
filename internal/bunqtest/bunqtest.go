// Package bunqtest provides a fake bunq API server for tests. It signs every
// response like bunq does and can verify request signatures. It uses only the
// standard library so that any package, including bunq itself, can use it.
package bunqtest

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// Handler answers one request with a status and a JSON body.
type Handler func(r *http.Request, body []byte) (status int, response string)

// Server is a fake bunq API.
type Server struct {
	*httptest.Server
	Key *rsa.PrivateKey // signs responses

	mu        sync.Mutex
	clientKey *rsa.PublicKey
}

// serverKey is shared by all servers: RSA generation is slow.
var serverKey = sync.OnceValues(func() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
})

// New starts a server that answers with h and stops it at test cleanup.
func New(t testing.TB, h Handler) *Server {
	t.Helper()
	key, err := serverKey()
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Key: key}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		if pub := s.requestKey(); pub != nil && len(body) > 0 {
			if err := verify(pub, body, r.Header.Get("X-Bunq-Client-Signature")); err != nil {
				t.Errorf("%s %s: request signature: %v", r.Method, r.URL.Path, err)
			}
		}
		status, resp := h(r, body)
		sig, err := sign(key, []byte(resp))
		if err != nil {
			t.Errorf("sign response: %v", err)
		}
		w.Header().Set("X-Bunq-Server-Signature", sig)
		w.WriteHeader(status)
		io.WriteString(w, resp)
	}))
	t.Cleanup(s.Close)
	return s
}

// VerifyRequestsWith makes the server check every request body signature
// against pub.
func (s *Server) VerifyRequestsWith(pub *rsa.PublicKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clientKey = pub
}

func (s *Server) requestKey() *rsa.PublicKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clientKey
}

// PublicKeyPEM returns the server public key as bunq sends it on installation.
func (s *Server) PublicKeyPEM() string {
	der, err := x509.MarshalPKIXPublicKey(&s.Key.PublicKey)
	if err != nil {
		panic(err) // an rsa.PublicKey always marshals
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func sign(key *rsa.PrivateKey, body []byte) (string, error) {
	sum := sha256.Sum256(body)
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	return base64.StdEncoding.EncodeToString(sig), err
}

func verify(pub *rsa.PublicKey, body []byte, sigB64 string) error {
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig)
}
