package flows

import (
	"encoding/base64"
	"strings"
	"testing"

	"imsub/internal/core"
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
		t.Fatalf("base64.RawURLEncoding.DecodeString(NewSecureToken(24)) returned unexpected error: %v", err)
	}
	if got, want := len(decoded), 24; got != want {
		t.Errorf("len(decodedToken) = %d, want %d", got, want)
	}
}

func TestOAuthStartURLEscapesState(t *testing.T) {
	t.Parallel()

	c := &Controller{
		cfg: config.Config{
			PublicBaseURL: "https://example.com",
		},
	}
	state := "a/b c"
	if got, want := c.oauthStartURL(state), "https://example.com/auth/start/a%2Fb%20c"; got != want {
		t.Errorf("(*Controller).oauthStartURL(%q) = %q, want %q", state, got, want)
	}
}

func TestCreatorGroupLineEscapesHTML(t *testing.T) {
	t.Parallel()

	line := CreatorGroupLines("en", `name<&>`, []core.ManagedGroup{{GroupName: `group "x"`}})
	if !strings.Contains(line, "name&lt;&amp;&gt;") {
		t.Errorf("CreatorGroupLines(%q, creatorName, groups) = %q, want escaped creator name", "en", line)
	}
	if !strings.Contains(line, "group &#34;x&#34;") {
		t.Errorf("CreatorGroupLines(%q, creatorName, groups) = %q, want escaped group name", "en", line)
	}
}
