package bot

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// NewSecureToken returns a cryptographically random, URL-safe base64 string
// generated from size random bytes.
func NewSecureToken(size int) (string, error) {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
