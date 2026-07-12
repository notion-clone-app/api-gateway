package auth

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type MethodPolicy func(fullMethod string) bool

func UnaryClientInterceptor(validator Validator, protected MethodPolicy) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		if protected(method) {
			if _, err := Authorize(ctx, validator); err != nil {
				return status.Error(codes.Unauthenticated, "invalid bearer token")
			}
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
