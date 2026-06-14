package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

type mockTransport struct{}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := httptest.NewRecorder()
	resp.WriteHeader(http.StatusOK)
	resp.WriteString("mock response")
	return resp.Result(), nil
}

func TestDynamicRegistry_InterceptRoute_Validation(t *testing.T) {
	registry := NewDynamicRegistry("", []string{"auth-service"})

	tests := []struct {
		name           string
		requestURL     string
		expectedStatus int
	}{
		{
			name:           "Valid Service Name",
			requestURL:     "/api/v1/auth-service/login",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid Service Name Pattern",
			requestURL:     "/api/v1/malicious-hack/payload",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Too Short Path",
			requestURL:     "/api/v1",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tt.requestURL, nil)
			rr := httptest.NewRecorder()

			r := chi.NewRouter()
			r.HandleFunc("/api/*", registry.InterceptRoute())

			if tt.name == "Valid Service Name" {
				p, _ := registry.GetOrCreateProxy("auth-service")
				p.Transport = &mockTransport{}
			}

			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
		})
	}
}
