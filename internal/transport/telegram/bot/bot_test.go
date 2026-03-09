package bot

import (
	"encoding/base64"
	"testing"

	"imsub/internal/platform/config"
)

func TestNewSecureToken(t *testing.T) {
	t.Parallel()

	token, err := NewSecureToken(24)
	if err != nil {
		t.Fatalf("NewSecureToken(24) returned unexpected error: %v", err)
	}
	if got, want := len(token), 32; got != want {
		t.Errorf("len(NewSecureToken(24)) = %d, want %d", got, want)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("DecodeString() returned unexpected error: %v", err)
	}
	if got, want := len(decoded), 24; got != want {
		t.Errorf("len(decodedToken) = %d, want %d", got, want)
	}
}

func TestOAuthStartURLEscapesState(t *testing.T) {
	t.Parallel()

	c := &Bot{cfg: config.Config{PublicBaseURL: "https://example.com"}}
	state := "a/b c"
	if got, want := c.oauthStartURL(state), "https://example.com/auth/start/a%2Fb%20c"; got != want {
		t.Errorf("(*Bot).oauthStartURL(%q) = %q, want %q", state, got, want)
	}
}
