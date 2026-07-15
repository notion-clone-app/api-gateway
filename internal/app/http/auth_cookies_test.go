package http

import (
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/notion-clone-app/api-gateway/internal/config"
	ssov1 "github.com/notion-clone-app/protos/gen/go/proto/sso"
)

func TestAuthCookiesWriteLoginResponse(t *testing.T) {
	cookies := newTestAuthCookies(t)
	response := &ssov1.LoginResponse{
		AccessToken:      "access-value",
		RefreshToken:     "refresh-value",
		AccessExpiresAt:  200,
		RefreshExpiresAt: 300,
	}
	recorder := httptest.NewRecorder()

	if err := cookies.writeResponse(context.Background(), recorder, response); err != nil {
		t.Fatal(err)
	}

	setCookies := recorder.Header().Values("Set-Cookie")
	if len(setCookies) != 2 {
		t.Fatalf("Set-Cookie headers = %d", len(setCookies))
	}
	assertCookieContains(t, setCookies[0], "access_token=access-value", "Path=/", "Max-Age=100", "HttpOnly", "SameSite=Lax")
	assertCookieContains(t, setCookies[1], "refresh_token=refresh-value", "Path=/v1/auth/refresh", "Max-Age=200", "HttpOnly", "SameSite=Lax")
	if response.AccessToken != "" || response.RefreshToken != "" {
		t.Fatal("tokens were not removed from the HTTP response")
	}
	if response.AccessExpiresAt != 200 || response.RefreshExpiresAt != 300 {
		t.Fatal("expiration timestamps were unexpectedly changed")
	}
}

func TestAuthCookiesWriteRegisterResponse(t *testing.T) {
	cookies := newTestAuthCookies(t)
	response := &ssov1.RegisterResponse{
		AccessToken:      "access-value",
		RefreshToken:     "refresh-value",
		AccessExpiresAt:  200,
		RefreshExpiresAt: 300,
	}

	if err := cookies.writeResponse(context.Background(), httptest.NewRecorder(), response); err != nil {
		t.Fatal(err)
	}
	if response.AccessToken != "" || response.RefreshToken != "" {
		t.Fatal("tokens were not removed from the HTTP response")
	}
}

func TestAuthCookiesWriteRefreshResponse(t *testing.T) {
	cookies := newTestAuthCookies(t)
	response := &ssov1.RefreshResponse{
		AccessToken: "next-access", RefreshToken: "next-refresh",
		AccessExpiresAt: 1_100, RefreshExpiresAt: 1_200,
	}
	recorder := httptest.NewRecorder()
	if err := cookies.writeResponse(context.Background(), recorder, response); err != nil {
		t.Fatal(err)
	}
	setCookies := recorder.Header().Values("Set-Cookie")
	assertCookieContains(t, setCookies[0], "access_token=next-access", "Path=/")
	assertCookieContains(t, setCookies[1], "refresh_token=next-refresh", "Path=/v1/auth/refresh")
	if response.AccessToken != "" || response.RefreshToken != "" {
		t.Fatal("refresh tokens leaked into response body")
	}
}

func TestAuthCookiesClearLogoutResponse(t *testing.T) {
	cookies := newTestAuthCookies(t)
	recorder := httptest.NewRecorder()
	if err := cookies.writeResponse(context.Background(), recorder, &ssov1.LogoutResponse{}); err != nil {
		t.Fatal(err)
	}
	setCookies := recorder.Header().Values("Set-Cookie")
	assertCookieContains(t, setCookies[0], "access_token=", "Path=/", "Max-Age=0")
	assertCookieContains(t, setCookies[1], "refresh_token=", "Path=/v1/auth/refresh", "Max-Age=0")
}

func TestAuthCookiesRequestMetadata(t *testing.T) {
	cookies := newTestAuthCookies(t)
	request := httptest.NewRequest(stdhttp.MethodGet, "/v1/documents", stdhttp.NoBody)
	request.AddCookie(&stdhttp.Cookie{
		Name: "access_token", Value: "access-value", Path: "/",
		HttpOnly: true, Secure: true, SameSite: stdhttp.SameSiteStrictMode,
	})
	request.AddCookie(&stdhttp.Cookie{
		Name: "refresh_token", Value: "refresh-value", Path: "/v1/auth/refresh",
		HttpOnly: true, Secure: true, SameSite: stdhttp.SameSiteStrictMode,
	})

	result := cookies.requestMetadata(context.Background(), request)
	if got := result.Get("authorization"); len(got) != 1 || got[0] != "Bearer access-value" {
		t.Fatalf("authorization metadata = %v", got)
	}
	if got := result.Get(refreshTokenMetadata); len(got) != 1 || got[0] != "refresh-value" {
		t.Fatalf("refresh metadata = %v", got)
	}
}

func TestAuthCookiesRejectExpiredToken(t *testing.T) {
	cookies := newTestAuthCookies(t)
	response := &ssov1.LoginResponse{
		AccessToken:      "access-value",
		RefreshToken:     "refresh-value",
		AccessExpiresAt:  99,
		RefreshExpiresAt: 300,
	}

	if err := cookies.writeResponse(context.Background(), httptest.NewRecorder(), response); err == nil {
		t.Fatal("writeResponse() expected an expiration error")
	}
}

func TestAuthCookiesRejectInsecureSameSiteNone(t *testing.T) {
	_, err := newAuthCookies(&config.CookieConfig{
		AccessName:  "access_token",
		RefreshName: "refresh_token",
		AccessPath:  "/",
		RefreshPath: "/v1/auth/refresh",
		SameSite:    "none",
	})
	if err == nil {
		t.Fatal("newAuthCookies() expected an error")
	}
}

func newTestAuthCookies(t *testing.T) *authCookies {
	t.Helper()
	cookies, err := newAuthCookies(&config.CookieConfig{
		AccessName:  "access_token",
		RefreshName: "refresh_token",
		AccessPath:  "/",
		RefreshPath: "/v1/auth/refresh",
		SameSite:    "lax",
	})
	if err != nil {
		t.Fatal(err)
	}
	cookies.now = func() time.Time { return time.Unix(100, 0) }
	return cookies
}

func assertCookieContains(t *testing.T, cookie string, expected ...string) {
	t.Helper()
	for _, value := range expected {
		if !strings.Contains(cookie, value) {
			t.Fatalf("cookie %q does not contain %q", cookie, value)
		}
	}
}
