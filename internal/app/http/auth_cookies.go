package http

import (
	"context"
	"fmt"
	stdhttp "net/http"
	"strings"
	"time"

	"github.com/notion-clone-app/api-gateway/internal/config"
	ssov1 "github.com/notion-clone-app/protos/gen/go/proto/sso"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

const refreshTokenMetadata = "x-refresh-token"

type authCookies struct {
	accessName  string
	refreshName string
	accessPath  string
	refreshPath string
	domain      string
	now         func() time.Time
	sameSite    stdhttp.SameSite
	secure      bool
}

func newAuthCookies(cfg config.CookieConfig) (*authCookies, error) {
	sameSite, err := parseSameSite(cfg.SameSite)
	if err != nil {
		return nil, err
	}

	cookies := &authCookies{
		accessName:  strings.TrimSpace(cfg.AccessName),
		refreshName: strings.TrimSpace(cfg.RefreshName),
		accessPath:  cfg.AccessPath,
		refreshPath: cfg.RefreshPath,
		domain:      strings.TrimSpace(cfg.Domain),
		sameSite:    sameSite,
		secure:      cfg.Secure,
		now:         time.Now,
	}

	if cookies.accessName == "" || cookies.refreshName == "" {
		return nil, fmt.Errorf("access and refresh cookie names are required")
	}
	if cookies.accessName == cookies.refreshName {
		return nil, fmt.Errorf("access and refresh cookie names must differ")
	}
	if cookies.sameSite == stdhttp.SameSiteNoneMode && !cookies.secure {
		return nil, fmt.Errorf("same_site=none requires secure=true")
	}
	if cookies.accessPath == "" || cookies.accessPath[0] != '/' {
		return nil, fmt.Errorf("access cookie path must start with /")
	}
	if cookies.refreshPath == "" || cookies.refreshPath[0] != '/' {
		return nil, fmt.Errorf("refresh cookie path must start with /")
	}
	if err := cookies.validatePrefixes(); err != nil {
		return nil, err
	}
	if err := cookies.validateCookieNames(); err != nil {
		return nil, err
	}

	return cookies, nil
}

func (c *authCookies) writeResponse(
	_ context.Context,
	w stdhttp.ResponseWriter,
	message proto.Message,
) error {
	var accessToken, refreshToken string
	var accessExpiresAt, refreshExpiresAt int64

	switch response := message.(type) {
	case *ssov1.RegisterResponse:
		accessToken = response.GetAccessToken()
		refreshToken = response.GetRefreshToken()
		accessExpiresAt = response.GetAccessExpiresAt()
		refreshExpiresAt = response.GetRefreshExpiresAt()
		response.AccessToken = ""
		response.RefreshToken = ""
	case *ssov1.LoginResponse:
		accessToken = response.GetAccessToken()
		refreshToken = response.GetRefreshToken()
		accessExpiresAt = response.GetAccessExpiresAt()
		refreshExpiresAt = response.GetRefreshExpiresAt()
		response.AccessToken = ""
		response.RefreshToken = ""
	case *ssov1.RefreshResponse:
		accessToken = response.GetAccessToken()
		refreshToken = response.GetRefreshToken()
		accessExpiresAt = response.GetAccessExpiresAt()
		refreshExpiresAt = response.GetRefreshExpiresAt()
		response.AccessToken = ""
		response.RefreshToken = ""
	case *ssov1.LogoutResponse:
		c.clearResponse(w)
		return nil
	default:
		return nil
	}

	accessCookie, err := c.tokenCookie(
		c.accessName,
		accessToken,
		c.accessPath,
		accessExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("create access cookie: %w", err)
	}
	refreshCookie, err := c.tokenCookie(
		c.refreshName,
		refreshToken,
		c.refreshPath,
		refreshExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("create refresh cookie: %w", err)
	}

	stdhttp.SetCookie(w, accessCookie)
	stdhttp.SetCookie(w, refreshCookie)
	return nil
}

func (c *authCookies) clearResponse(w stdhttp.ResponseWriter) {
	for name, path := range map[string]string{
		c.accessName:  c.accessPath,
		c.refreshName: c.refreshPath,
	} {
		stdhttp.SetCookie(w, &stdhttp.Cookie{
			Name: name, Value: "", Path: path, Domain: c.domain,
			Expires: time.Unix(1, 0), MaxAge: -1, HttpOnly: true,
			Secure: c.secure, SameSite: c.sameSite,
		})
	}
}

func (c *authCookies) requestMetadata(_ context.Context, r *stdhttp.Request) metadata.MD {
	result := metadata.MD{}

	if r.Header.Get("Authorization") == "" {
		if cookie, err := r.Cookie(c.accessName); err == nil && cookie.Value != "" {
			result.Set("authorization", "Bearer "+cookie.Value)
		}
	}
	if cookie, err := r.Cookie(c.refreshName); err == nil && cookie.Value != "" {
		result.Set(refreshTokenMetadata, cookie.Value)
	}

	return result
}

func (c *authCookies) tokenCookie(name, value, path string, expiresAt int64) (*stdhttp.Cookie, error) {
	if value == "" {
		return nil, fmt.Errorf("token is empty")
	}

	expires := time.Unix(expiresAt, 0)
	maxAge := int(expires.Sub(c.now()).Seconds())
	if expiresAt <= 0 || maxAge <= 0 {
		return nil, fmt.Errorf("token expiration must be a future Unix timestamp")
	}

	cookie := &stdhttp.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		Domain:   c.domain,
		Expires:  expires,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   c.secure,
		SameSite: c.sameSite,
	}
	if err := cookie.Valid(); err != nil {
		return nil, err
	}
	return cookie, nil
}

func (c *authCookies) validatePrefixes() error {
	for _, name := range []string{c.accessName, c.refreshName} {
		if strings.HasPrefix(name, "__Secure-") && !c.secure {
			return fmt.Errorf("cookie %q requires secure=true", name)
		}
		if strings.HasPrefix(name, "__Host-") {
			if !c.secure {
				return fmt.Errorf("cookie %q requires secure=true", name)
			}
			if c.domain != "" {
				return fmt.Errorf("cookie %q requires an empty domain", name)
			}
		}
	}
	if strings.HasPrefix(c.accessName, "__Host-") && c.accessPath != "/" {
		return fmt.Errorf("cookie %q requires path=/", c.accessName)
	}
	if strings.HasPrefix(c.refreshName, "__Host-") && c.refreshPath != "/" {
		return fmt.Errorf("cookie %q requires path=/", c.refreshName)
	}
	return nil
}

func (c *authCookies) validateCookieNames() error {
	for name, path := range map[string]string{
		c.accessName:  c.accessPath,
		c.refreshName: c.refreshPath,
	} {
		cookie := &stdhttp.Cookie{Name: name, Value: "validation", Path: path}
		if err := cookie.Valid(); err != nil {
			return fmt.Errorf("invalid cookie %q: %w", name, err)
		}
	}
	return nil
}

func parseSameSite(value string) (stdhttp.SameSite, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return stdhttp.SameSiteStrictMode, nil
	case "lax":
		return stdhttp.SameSiteLaxMode, nil
	case "none":
		return stdhttp.SameSiteNoneMode, nil
	default:
		return stdhttp.SameSiteDefaultMode, fmt.Errorf(
			"same_site must be strict, lax, or none",
		)
	}
}
