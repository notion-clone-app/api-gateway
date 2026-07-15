package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc/metadata"
)

var ErrUnauthenticated = errors.New("missing or invalid bearer token")

type Claims struct {
	Subject   string
	SessionID string
	Issuer    string
	Audience  []string
	ExpiresAt int64
}

type Validator interface {
	Validate(context.Context, string) (*Claims, error)
}

type HMACValidator struct {
	issuer   string
	audience string
	now      func() time.Time
	secret   []byte
}

func NewHMACValidator(secret, issuer, audience string) (*HMACValidator, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("JWT HMAC secret must be at least 32 bytes")
	}
	return &HMACValidator{secret: []byte(secret), issuer: issuer, audience: audience, now: time.Now}, nil
}

func (v *HMACValidator) Validate(_ context.Context, token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrUnauthenticated
	}

	var header struct {
		Algorithm string `json:"alg"`
	}
	if err := decodeJSON(parts[0], &header); err != nil || header.Algorithm != "HS256" {
		return nil, ErrUnauthenticated
	}

	mac := hmac.New(sha256.New, v.secret)
	if _, err := mac.Write([]byte(parts[0] + "." + parts[1])); err != nil {
		return nil, ErrUnauthenticated
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		return nil, ErrUnauthenticated
	}

	var payload struct {
		Subject   string          `json:"sub"`
		SessionID string          `json:"sid"`
		Issuer    string          `json:"iss"`
		Audience  json.RawMessage `json:"aud"`
		ExpiresAt int64           `json:"exp"`
		NotBefore int64           `json:"nbf"`
	}
	decodeErr := decodeJSON(parts[1], &payload)
	if decodeErr != nil {
		return nil, ErrUnauthenticated
	}

	now := v.now().Unix()
	if payload.Subject == "" || payload.SessionID == "" || payload.ExpiresAt == 0 || now >= payload.ExpiresAt || (payload.NotBefore != 0 && now < payload.NotBefore) {
		return nil, ErrUnauthenticated
	}
	if v.issuer != "" && payload.Issuer != v.issuer {
		return nil, ErrUnauthenticated
	}

	audience, err := parseAudience(payload.Audience)
	if err != nil || (v.audience != "" && !contains(audience, v.audience)) {
		return nil, ErrUnauthenticated
	}

	return &Claims{
		Subject: payload.Subject, SessionID: payload.SessionID, ExpiresAt: payload.ExpiresAt,
		Issuer: payload.Issuer, Audience: audience,
	}, nil
}

func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return metadata.AppendToOutgoingContext(
		ctx,
		"x-user-id", claims.Subject,
		"x-session-id", claims.SessionID,
		"x-access-expires-at", fmt.Sprintf("%d", claims.ExpiresAt),
	)
}

func BearerToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md, ok = metadata.FromOutgoingContext(ctx)
		if !ok {
			return "", ErrUnauthenticated
		}
	}
	values := md.Get("authorization")
	if len(values) != 1 {
		return "", ErrUnauthenticated
	}
	parts := strings.SplitN(values[0], " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", ErrUnauthenticated
	}
	return strings.TrimSpace(parts[1]), nil
}

func Authorize(ctx context.Context, validator Validator) (*Claims, error) {
	token, err := BearerToken(ctx)
	if err != nil {
		return nil, err
	}
	return validator.Validate(ctx, token)
}

func decodeJSON(part string, target any) error {
	data, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func parseAudience(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return []string{single}, nil
	}
	var multiple []string
	if err := json.Unmarshal(raw, &multiple); err != nil {
		return nil, err
	}
	return multiple, nil
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
