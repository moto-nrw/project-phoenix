package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// Only the devtools command adapters enable this cache. Separate seed/simulate
// processes rotate the same verified session instead of sending another code.
// One file per server and credentials follows the account across tenant switches.
type tenantSessionCache struct {
	dir      string
	mu       sync.Mutex
	sessions map[string]tenantSessionKey
	loaded   map[string]bool
}

type tenantSessionKey struct{ file, slug string }
type tenantSessionContextKey struct{}

type storedTenantSession struct {
	TenantSlug   string `json:"tenant_slug"`
	RefreshToken string `json:"refresh_token"`
}

func newTenantSessionCache(dir string) *tenantSessionCache {
	return &tenantSessionCache{dir: dir, sessions: make(map[string]tenantSessionKey), loaded: make(map[string]bool)}
}

func (a *Adapter) loginWithTenantSession(ctx context.Context, email, password, slug string) (AuthRef, error) {
	if err := assertDevOnlyTarget(a.baseURL, os.Getenv("APP_ENV")); err != nil {
		return AuthRef{}, err
	}
	encoded, err := json.Marshal([]string{a.baseURL, email, password})
	if err != nil {
		return AuthRef{}, err
	}
	key := tenantSessionKey{file: filepath.Join(a.tenantSessions.dir, securityruntime.Fingerprint(encoded)+".json"), slug: slug}
	// The seed run's existing login cache owns live access tokens. Restore each
	// account only once here, so a hidden refresh cannot retire one of those
	// live tokens when the seeder signs into a different school later.
	a.tenantSessions.mu.Lock()
	alreadyLoaded := a.tenantSessions.loaded[key.file]
	a.tenantSessions.loaded[key.file] = true
	a.tenantSessions.mu.Unlock()
	if alreadyLoaded {
		return a.loginTenant(context.WithValue(ctx, tenantSessionContextKey{}, key), email, password, slug)
	}
	raw, err := os.ReadFile(key.file)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return AuthRef{}, fmt.Errorf("read devtools session cache: %w", err)
	}
	var saved storedTenantSession
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &saved); err != nil {
			return AuthRef{}, fmt.Errorf("invalid devtools session cache (remove %s)", key.file)
		}
	}
	if saved.RefreshToken != "" {
		key.slug = saved.TenantSlug
		refreshCtx := context.WithValue(ctx, tenantSessionContextKey{}, key)
		refreshed, _, refreshErr := a.Raw(refreshCtx, AuthRef{Kind: AuthBearer, Token: saved.RefreshToken}, http.MethodPost, "/auth/refresh", nil, nil)
		if refreshErr == nil {
			token, parseErr := parseLoginToken(refreshed)
			if parseErr != nil || token == "" {
				return AuthRef{}, fmt.Errorf("devtools session refresh returned no access token")
			}
			auth := AuthRef{Kind: AuthBearer, Label: saved.TenantSlug, Token: token}
			if strings.TrimSpace(slug) != "" && slug != saved.TenantSlug {
				switched, _, switchErr := a.Raw(refreshCtx, auth, http.MethodPost, "/auth/switch-tenant", map[string]string{"tenant_slug": slug}, nil)
				if switchErr != nil {
					return AuthRef{}, fmt.Errorf("switch cached devtools session: %w", switchErr)
				}
				auth.Token, err = parseLoginToken(switched)
				if err != nil || auth.Token == "" {
					return AuthRef{}, fmt.Errorf("devtools tenant switch returned no access token")
				}
				auth.Label = slug
			}
			return auth, nil
		}
		var apiErr *APIError
		if !errors.As(refreshErr, &apiErr) || (apiErr.StatusCode != http.StatusUnauthorized && apiErr.StatusCode != http.StatusForbidden) {
			return AuthRef{}, fmt.Errorf("refresh devtools session: %w", refreshErr)
		}
		// Expired/revoked sessions and newly required MFA must use the real login.
		if err := os.Remove(key.file); err != nil && !errors.Is(err, os.ErrNotExist) {
			return AuthRef{}, fmt.Errorf("remove expired devtools session: %w", err)
		}
	}
	key.slug = slug
	return a.loginTenant(context.WithValue(ctx, tenantSessionContextKey{}, key), email, password, slug)
}

func (c *tenantSessionCache) record(ctx context.Context, auth AuthRef, method, path string, body any, raw []byte) error {
	if method != http.MethodPost {
		return nil
	}
	key, known := ctx.Value(tenantSessionContextKey{}).(tenantSessionKey)
	switch path {
	case "/auth/login", "/auth/mfa/verify", "/auth/mfa/enroll/confirm", "/auth/refresh":
	case "/auth/switch-tenant":
		c.mu.Lock()
		key, known = c.sessions[auth.Token]
		c.mu.Unlock()
		var request struct {
			TenantSlug string `json:"tenant_slug"`
		}
		encoded, err := json.Marshal(body)
		if err != nil || json.Unmarshal(encoded, &request) != nil || request.TenantSlug == "" {
			return fmt.Errorf("cannot cache devtools tenant switch")
		}
		key.slug = request.TenantSlug
	default:
		return nil
	}
	if !known {
		return nil
	}
	var tokens struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	var envelope Envelope
	if json.Unmarshal(raw, &envelope) == nil && len(envelope.Data) > 0 {
		raw = envelope.Data
	}
	if err := json.Unmarshal(raw, &tokens); err != nil || tokens.AccessToken == "" || tokens.RefreshToken == "" {
		// Enrollment/challenge responses are not authenticated sessions.
		return nil
	}
	if err := c.save(key, tokens.RefreshToken); err != nil {
		return err
	}
	c.mu.Lock()
	delete(c.sessions, auth.Token)
	c.sessions[tokens.AccessToken] = key
	c.mu.Unlock()
	return nil
}

func (c *tenantSessionCache) save(key tenantSessionKey, refreshToken string) error {
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return fmt.Errorf("create devtools session cache: %w", err)
	}
	file, err := os.CreateTemp(c.dir, ".session-*")
	if err != nil {
		return fmt.Errorf("create devtools session: %w", err)
	}
	defer func() { _ = os.Remove(file.Name()) }()
	encodeErr := json.NewEncoder(file).Encode(storedTenantSession{TenantSlug: key.slug, RefreshToken: refreshToken})
	closeErr := file.Close()
	if err := errors.Join(encodeErr, closeErr); err != nil {
		return fmt.Errorf("write devtools session: %w", err)
	}
	if err := os.Rename(file.Name(), key.file); err != nil {
		return fmt.Errorf("replace devtools session: %w", err)
	}
	return nil
}
