package auth

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Mode uint8

const (
	ModePublic Mode = iota
	ModeAccessToken
	ModeRefreshToken
)

type MethodPolicy func(fullMethod string) Mode

func UnaryClientInterceptor(validator Validator, protected MethodPolicy) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		if protected(method) == ModeAccessToken {
			claims, err := Authorize(ctx, validator)
			if err != nil {
				return status.Error(codes.Unauthenticated, "invalid bearer token")
			}
			ctx = WithClaims(ctx, claims)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
