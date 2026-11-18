package bunq

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
)

// KeyBits is the RSA key size required by bunq.
const KeyBits = 2048

// GenerateKey returns a new RSA key pair for an installation.
func GenerateKey() (*rsa.PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, KeyBits)
	if err != nil {
		return nil, fmt.Errorf("generate rsa key: %w", err)
	}
	return key, nil
}

// EncodePrivateKey returns key as a PKCS #8 PEM block.
func EncodePrivateKey(key *rsa.PrivateKey) (string, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", fmt.Errorf("encode private key: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), nil
}

// DecodePrivateKey parses a PKCS #8 PEM block.
func DecodePrivateKey(s string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(s))
	if block == nil {
		return nil, errors.New("decode private key: no pem block")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("decode private key: not an rsa key")
	}
	return rsaKey, nil
}

// EncodePublicKey returns key as a PKIX ("BEGIN PUBLIC KEY") PEM block.
func EncodePublicKey(key *rsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return "", fmt.Errorf("encode public key: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), nil
}

// DecodePublicKey parses a PKIX PEM block, such as the bunq server public key.
func DecodePublicKey(s string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(s))
	if block == nil {
		return nil, errors.New("decode public key: no pem block")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("decode public key: not an rsa key")
	}
	return rsaKey, nil
}

// sign returns the base64 SHA-256 PKCS #1 v1.5 signature of body.
func sign(key *rsa.PrivateKey, body []byte) (string, error) {
	sum := sha256.Sum256(body)
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", fmt.Errorf("sign request: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// verify checks a base64 SHA-256 PKCS #1 v1.5 signature of body.
func verify(key *rsa.PublicKey, body []byte, sigB64 string) error {
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	sum := sha256.Sum256(body)
	return rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig)
}
