package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGatewayMarshalsGRPCErrorDetails(t *testing.T) {
	grpcStatus, err := status.New(codes.InvalidArgument, "invalid registration data").WithDetails(
		&errdetails.BadRequest{FieldViolations: []*errdetails.BadRequest_FieldViolation{{
			Field: "password", Reason: "INVALID_LENGTH", Description: "password is too short",
		}}},
	)
	if err != nil {
		t.Fatalf("attach error details: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/auth/register", http.NoBody)
	runtime.HTTPError(context.Background(), runtime.NewServeMux(), &runtime.JSONPb{}, recorder, request, grpcStatus.Err())

	body := recorder.Body.String()
	if strings.Contains(body, "failed to marshal error message") {
		t.Fatalf("gateway failed to marshal details: %s", body)
	}
	if !strings.Contains(body, "INVALID_LENGTH") || !strings.Contains(body, "password") {
		t.Fatalf("gateway dropped structured error details: %s", body)
	}
}
