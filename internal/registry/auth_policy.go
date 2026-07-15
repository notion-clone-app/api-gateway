package registry

import (
	"strings"

	"github.com/notion-clone-app/api-gateway/internal/auth"
	commonv1 "github.com/notion-clone-app/protos/gen/go/proto/common"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

func AuthenticationMode(fullMethod string) auth.Mode {
	serviceName, methodName, ok := splitMethod(fullMethod)
	if !ok {
		return auth.ModePublic
	}

	descriptor, err := protoregistry.GlobalFiles.FindDescriptorByName(
		protoreflect.FullName(serviceName + "." + methodName),
	)
	if err == nil {
		if method, ok := descriptor.(protoreflect.MethodDescriptor); ok {
			if mode, ok := annotatedMode(method); ok {
				return mode
			}
			return auth.ModeAccessToken
		}
	}

	for _, service := range PublicHTTPServices() {
		if service.GRPCService == serviceName && service.AuthRequired {
			return auth.ModeAccessToken
		}
	}
	return auth.ModePublic
}

func annotatedMode(method protoreflect.MethodDescriptor) (auth.Mode, bool) {
	options, ok := method.Options().(*descriptorpb.MethodOptions)
	if !ok || !proto.HasExtension(options, commonv1.E_Authorization) {
		return auth.ModePublic, false
	}
	policy, ok := proto.GetExtension(options, commonv1.E_Authorization).(*commonv1.AuthorizationPolicy)
	if !ok || policy == nil {
		return auth.ModePublic, false
	}

	switch policy.GetAuthentication() {
	case commonv1.AuthenticationMode_AUTHENTICATION_MODE_PUBLIC:
		return auth.ModePublic, true
	case commonv1.AuthenticationMode_AUTHENTICATION_MODE_ACCESS_TOKEN:
		return auth.ModeAccessToken, true
	case commonv1.AuthenticationMode_AUTHENTICATION_MODE_REFRESH_TOKEN:
		return auth.ModeRefreshToken, true
	default:
		return auth.ModePublic, false
	}
}

func splitMethod(fullMethod string) (string, string, bool) {
	parts := strings.Split(strings.TrimPrefix(fullMethod, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
