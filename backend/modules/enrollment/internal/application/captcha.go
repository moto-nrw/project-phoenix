package application

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// Captcha verifies a parent-submitted captcha token against the configured
// provider. Today: Cloudflare Turnstile. The abstraction keeps hCaptcha or
// another provider droppable without changing the call sites in the
// submission flow.
type Captcha struct {
	settings CaptchaSettings
	provider CaptchaSiteVerify
	logger   *slog.Logger
}

// NewCaptcha binds captcha verification to the tenant settings and the
// provider. A nil logger falls back to slog.Default().
func NewCaptcha(settings CaptchaSettings, provider CaptchaSiteVerify, logger *slog.Logger) *Captcha {
	if logger == nil {
		logger = slog.Default()
	}
	return &Captcha{settings: settings, provider: provider, logger: logger}
}

// IsEnabled reports the enrollment.require_captcha setting of the tenant in
// context. Default false: a fresh tenant has no Turnstile keys configured
// yet.
func (s *Captcha) IsEnabled(ctx context.Context) bool {
	return s.settings != nil && s.settings.CaptchaRequired(ctx)
}

// Verify validates token against the configured provider for the tenant in
// context. remoteIP is the parent's source IP - passed through to the
// provider as a defense-in-depth signal. Returns nil on success and when
// captcha is disabled.
func (s *Captcha) Verify(ctx context.Context, token, remoteIP string) error {
	if !s.IsEnabled(ctx) {
		return nil
	}

	secret := s.settings.CaptchaSecretKey(ctx)
	if secret == "" {
		return fmt.Errorf("captcha secret key not configured")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("captcha token is required")
	}

	success, errorCodes, err := s.provider.SiteVerify(ctx, secret, token, remoteIP)
	if err != nil {
		return err
	}
	if !success {
		s.logger.Warn("captcha verification failed",
			slog.Any("error_codes", errorCodes),
			slog.String("remote_ip", remoteIP))
		return fmt.Errorf("captcha verification failed")
	}
	return nil
}

// SiteKey returns the public site key for the tenant in context, or "" when
// unset. Safe to expose on a public endpoint — it's the same value that
// lives in the rendered widget markup.
func (s *Captcha) SiteKey(ctx context.Context) string {
	if s.settings == nil {
		return ""
	}
	return s.settings.CaptchaSiteKey(ctx)
}
