package registry

import (
	"testing"

	"github.com/notion-clone-app/api-gateway/internal/auth"
)

func TestAuthenticationModeFromProtoOptions(t *testing.T) {
	tests := map[string]auth.Mode{
		"/auth.Auth/Login":    auth.ModePublic,
		"/auth.Auth/Register": auth.ModePublic,
		"/auth.Auth/Refresh":  auth.ModeRefreshToken,
		"/auth.Auth/Me":       auth.ModeAccessToken,
		"/auth.Auth/Logout":   auth.ModeAccessToken,
	}
	for method, expected := range tests {
		if got := AuthenticationMode(method); got != expected {
			t.Errorf("AuthenticationMode(%q) = %v, want %v", method, got, expected)
		}
	}
}

func TestAuthenticationModeFallsBackToServicePolicy(t *testing.T) {
	if got := AuthenticationMode("/marketing.Marketing/ListDocuments"); got != auth.ModeAccessToken {
		t.Fatalf("AuthenticationMode() = %v, want access token", got)
	}
}
