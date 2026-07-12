package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsGRPCRequest(t *testing.T) {
	request := httptest.NewRequest(stdhttp.MethodPost, "http://gateway/auth.Auth/Login", strings.NewReader("request"))
	request.ProtoMajor = 2
	request.Header.Set("Content-Type", "application/grpc+proto")

	if !isGRPCRequest(request) {
		t.Fatal("isGRPCRequest() = false")
	}
}

func TestIsGRPCRequestRejectsHTTPJSON(t *testing.T) {
	request := httptest.NewRequest(stdhttp.MethodPost, "http://gateway/v1/auth/login", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")

	if isGRPCRequest(request) {
		t.Fatal("isGRPCRequest() = true")
	}
}

func TestRegisterDocs(t *testing.T) {
	router := stdhttp.NewServeMux()
	registerDocs(router, `{"openapi":"3.1.0"}`)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodGet, "/openapi.json", nil)
	router.ServeHTTP(response, request)

	if response.Code != stdhttp.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("content type = %q", response.Header().Get("Content-Type"))
	}
}
