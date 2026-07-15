package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notion-clone-app/api-gateway/internal/config"
)

func TestCORSPreflight(t *testing.T) {
	nextCalled := false
	next := stdhttp.HandlerFunc(func(stdhttp.ResponseWriter, *stdhttp.Request) {
		nextCalled = true
	})
	handler, err := newCORSMiddleware(config.CORSConfig{
		AllowedOrigins: []string{"http://localhost:5173"},
	}, next)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(stdhttp.MethodOptions, "/v1/auth/register", stdhttp.NoBody)
	request.Header.Set("Origin", "http://localhost:5173")
	request.Header.Set("Access-Control-Request-Method", stdhttp.MethodPost)
	request.Header.Set("Access-Control-Request-Headers", "content-type,authorization")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != stdhttp.StatusNoContent {
		t.Fatalf("status = %d", response.Code)
	}
	if nextCalled {
		t.Fatal("preflight request reached the gateway handler")
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("allow origin = %q", got)
	}
	if got := response.Header().Get("Access-Control-Allow-Headers"); !stringsContainFold(got, "Authorization") {
		t.Fatalf("allow headers = %q", got)
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	handler, err := newCORSMiddleware(config.CORSConfig{
		AllowedOrigins: []string{"https://app.example.com"},
	}, stdhttp.NotFoundHandler())
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(stdhttp.MethodGet, "/v1/documents", stdhttp.NoBody)
	request.Header.Set("Origin", "https://evil.example.com")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != stdhttp.StatusForbidden {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestCORSAddsHeadersToResponse(t *testing.T) {
	handler, err := newCORSMiddleware(config.CORSConfig{
		AllowedOrigins: []string{"https://app.example.com"},
	}, stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		w.WriteHeader(stdhttp.StatusOK)
	}))
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(stdhttp.MethodGet, "/v1/documents", stdhttp.NoBody)
	request.Header.Set("Origin", "https://app.example.com")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("allow origin = %q", got)
	}
}

func stringsContainFold(value, expected string) bool {
	for item := range strings.SplitSeq(value, ",") {
		if strings.EqualFold(strings.TrimSpace(item), expected) {
			return true
		}
	}
	return false
}
