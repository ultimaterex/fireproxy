package tplink

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	ErrNoSecretsKey  = errors.New("FIREPROXY_SECRETS_KEY not set")
	ErrBadSecretsKey = errors.New("FIREPROXY_SECRETS_KEY must be 64 hex characters")
	ErrBadCipher     = errors.New("invalid ciphertext")
)

// KeyFromEnv parses FIREPROXY_SECRETS_KEY as exactly 64 hex characters (32 raw bytes).
func KeyFromEnv(env string) ([]byte, error) {
	env = strings.TrimSpace(env)
	if env == "" {
		return nil, ErrNoSecretsKey
	}
	if len(env) != 64 {
		return nil, ErrBadSecretsKey
	}
	b, err := hex.DecodeString(env)
	if err != nil || len(b) != 32 {
		return nil, ErrBadSecretsKey
	}
	return b, nil
}

// Encrypt returns base64(nonce|ciphertext) using AES-GCM.
func Encrypt(key []byte, plaintext string) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

// Decrypt reverses Encrypt.
func Decrypt(key []byte, blob string) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("key must be 32 bytes")
	}
	raw, err := base64.StdEncoding.DecodeString(blob)
	if err != nil {
		return "", ErrBadCipher
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return "", ErrBadCipher
	}
	plain, err := gcm.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", ErrBadCipher
	}
	return string(plain), nil
}
