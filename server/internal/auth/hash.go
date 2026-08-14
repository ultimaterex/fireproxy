package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
)

// NewToken returns plaintext token prefix + 32 random bytes as hex.
func NewToken(prefix string) (plaintext string, err error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b[:]), nil
}

// HashToken returns hex-encoded SHA-256 of plaintext.
func HashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// VerifyToken compares plaintext against a hex SHA-256 hash in constant time.
func VerifyToken(plaintext, hashHex string) bool {
	want, err := hex.DecodeString(hashHex)
	if err != nil {
		return false
	}
	sum := sha256.Sum256([]byte(plaintext))
	if len(want) != len(sum) {
		return false
	}
	return subtle.ConstantTimeCompare(want, sum[:]) == 1
}
