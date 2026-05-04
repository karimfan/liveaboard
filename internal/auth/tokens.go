package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// NewToken returns a 64-char hex string suitable for an email link
// (verification, password reset, invitation, change-email) plus its
// sha256 hash for storage in the corresponding table.
func NewToken() (raw string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	raw = hex.EncodeToString(b)
	return raw, HashToken(raw), nil
}

// HashToken returns sha256(token). The plaintext token only ever
// exists in the URL we send by email; only the hash hits the DB.
func HashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}
