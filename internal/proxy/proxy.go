package proxy

import (
	"errors"
	"io"

	"github.com/notion-clone-app/api-gateway/internal/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Route struct {
	Connection *grpc.ClientConn
}

type Handler struct {
	routes    map[string]Route
	validator auth.Validator
	policy    auth.MethodPolicy
}

func NewHandler(routes map[string]Route, validator auth.Validator, policy auth.MethodPolicy) *Handler {
	return &Handler{routes: routes, validator: validator, policy: policy}
}

func Codec() grpc.ServerOption { return grpc.ForceServerCodec(rawCodec{}) }

func (h *Handler) Handle(_ any, downstream grpc.ServerStream) error {
	method, ok := grpc.MethodFromServerStream(downstream)
	if !ok {
		return status.Error(codes.Internal, "cannot determine gRPC method")
	}
	service, ok := serviceFromMethod(method)
	if !ok {
		return status.Error(codes.Unimplemented, "invalid gRPC method")
	}
	route, ok := h.routes[service]
	if !ok {
		return status.Error(codes.Unimplemented, "gRPC service is not exposed")
	}
	ctx := downstream.Context()
	if h.policy(method) == auth.ModeAccessToken {
		claims, err := auth.Authorize(ctx, h.validator)
		if err != nil {
			return status.Error(codes.Unauthenticated, "invalid bearer token")
		}
		ctx = auth.WithClaims(ctx, claims)
	}

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		outgoing, _ := metadata.FromOutgoingContext(ctx)
		merged := md.Copy()
		for key, values := range outgoing {
			merged.Set(key, values...)
		}
		ctx = metadata.NewOutgoingContext(ctx, merged)
	}
	upstream, err := route.Connection.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true, ClientStreams: true}, method, grpc.ForceCodec(rawCodec{}))
	if err != nil {
		return status.Errorf(codes.Unavailable, "open upstream stream: %v", err)
	}

	requestErr := make(chan error, 1)
	responseErr := make(chan error, 1)
	go copyRequests(downstream, upstream, requestErr)
	go copyResponses(downstream, upstream, responseErr)

	for i := 0; i < 2; i++ {
		select {
		case err := <-requestErr:
			if err != nil {
				return err
			}
		case err := <-responseErr:
			downstream.SetTrailer(upstream.Trailer())
			return err
		}
	}
	return nil
}

func copyRequests(downstream grpc.ServerStream, upstream grpc.ClientStream, done chan<- error) {
	for {
		message := new(frame)
		if err := downstream.RecvMsg(message); err != nil {
			if errors.Is(err, io.EOF) {
				done <- upstream.CloseSend()
			} else {
				done <- err
			}
			return
		}
		if err := upstream.SendMsg(message); err != nil {
			done <- err
			return
		}
	}
}

func copyResponses(downstream grpc.ServerStream, upstream grpc.ClientStream, done chan<- error) {
	header, err := upstream.Header()
	if err != nil {
		done <- err
		return
	}
	if err := downstream.SendHeader(header); err != nil {
		done <- err
		return
	}
	for {
		message := new(frame)
		if err := upstream.RecvMsg(message); err != nil {
			if errors.Is(err, io.EOF) {
				done <- nil
			} else {
				done <- err
			}
			return
		}
		if err := downstream.SendMsg(message); err != nil {
			done <- err
			return
		}
	}
}

func serviceFromMethod(method string) (string, bool) {
	if len(method) < 3 || method[0] != '/' {
		return "", false
	}
	for i := 1; i < len(method); i++ {
		if method[i] == '/' {
			return method[1:i], i < len(method)-1
		}
	}
	return "", false
}
