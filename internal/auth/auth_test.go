package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"
)

func TestHMACValidator(t *testing.T) {
	const secret = "01234567890123456789012345678901"
	validator, err := NewHMACValidator(secret, "issuer", "mobile")
	if err != nil {
		t.Fatal(err)
	}
	validator.now = func() time.Time { return time.Unix(100, 0) }

	token := signedToken(t, secret, map[string]any{
		"sub": "user-1", "iss": "issuer", "aud": "mobile", "exp": 200,
	})
	claims, err := validator.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if claims.Subject != "user-1" {
		t.Fatalf("Subject = %q", claims.Subject)
	}
}

func TestAuthorizeRejectsMissingBearerToken(t *testing.T) {
	validator, _ := NewHMACValidator("01234567890123456789012345678901", "", "")
	if _, err := Authorize(context.Background(), validator); err == nil {
		t.Fatal("Authorize() expected an error")
	}
}

func TestBearerTokenFromMetadata(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token-value"))
	token, err := BearerToken(ctx)
	if err != nil || token != "token-value" {
		t.Fatalf("BearerToken() = %q, %v", token, err)
	}
}

func TestBearerTokenFromOutgoingGatewayMetadata(t *testing.T) {
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer gateway-token"))
	token, err := BearerToken(ctx)
	if err != nil || token != "gateway-token" {
		t.Fatalf("BearerToken() = %q, %v", token, err)
	}
}

func signedToken(t *testing.T, secret string, claims map[string]any) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	encodedHeader := base64.RawURLEncoding.EncodeToString(header)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	unsigned := encodedHeader + "." + encodedPayload
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
