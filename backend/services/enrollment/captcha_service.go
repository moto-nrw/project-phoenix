package enrollment

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

// CaptchaSettingsResolver is the narrow contract the captcha service
// needs from the platform settings service. Defined locally so this
// package doesn't import services/config (which would create a cycle
// once config wants to read enrollment-side state).
type CaptchaSettingsResolver interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
}

// CaptchaService verifies a parent-submitted captcha token against the
// configured provider. Today: Cloudflare Turnstile. The plan keeps the
// abstraction so hCaptcha or another provider can drop in later
// without changing the call sites in the submission service.
//
// Verify returns nil when the captcha is valid, or an error explaining
// why it failed. Returns nil unconditionally when captcha is disabled
// for the tenant (settings-driven via enrollment.require_captcha).
type CaptchaService interface {
	// Verify validates `token` against the configured provider for the
	// tenant in context. `remoteIP` is the parent's source IP — passed
	// through to the provider as a defense-in-depth signal.
	Verify(ctx context.Context, token, remoteIP string) error

	// IsEnabled reports whether the captcha gate is active for the
	// tenant in context. Frontend calls this so it can hide the
	// widget when the setting is off.
	IsEnabled(ctx context.Context) bool
}

// CaptchaServiceConfig is the dependency-injection bundle.
type CaptchaServiceConfig struct {
	Settings   CaptchaSettingsResolver
	Logger     *slog.Logger
	HTTPClient *http.Client
	VerifyURL  string // override for tests; defaults to Turnstile siteverify URL
}

const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

type captchaService struct {
	settings   CaptchaSettingsResolver
	logger     *slog.Logger
	httpClient *http.Client
	verifyURL  string
}

// NewCaptchaService wires a Turnstile-backed verifier. A nil HTTPClient
// falls back to a 10-second-timeout client. Empty VerifyURL falls back
// to the canonical Turnstile endpoint.
func NewCaptchaService(cfg CaptchaServiceConfig) CaptchaService {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	verifyURL := cfg.VerifyURL
	if verifyURL == "" {
		verifyURL = turnstileVerifyURL
	}
	return &captchaService{
		settings:   cfg.Settings,
		logger:     logger,
		httpClient: httpClient,
		verifyURL:  verifyURL,
	}
}

// IsEnabled checks the enrollment.require_captcha setting via the
// standard HasTenantOverride → ResolveBool → env var → default chain.
// Default true: a fresh tenant gets bot protection out of the box.
func (s *captchaService) IsEnabled(ctx context.Context) bool {
	if s.settings == nil {
		return true
	}
	if has, err := s.settings.HasTenantOverride(ctx, configModel.KeyEnrollmentRequireCaptcha); err == nil && has {
		if v, err := s.settings.ResolveBool(ctx, configModel.KeyEnrollmentRequireCaptcha); err == nil {
			return v
		}
	}
	if env := strings.TrimSpace(os.Getenv("ENROLLMENT_REQUIRE_CAPTCHA")); env != "" {
		return env == "true"
	}
	return true
}

// Verify hits the provider's siteverify endpoint with secret + token +
// remote IP. Returns nil on success.
func (s *captchaService) Verify(ctx context.Context, token, remoteIP string) error {
	if !s.IsEnabled(ctx) {
		return nil
	}

	secret := s.resolveSecret(ctx)
	if secret == "" {
		return fmt.Errorf("captcha secret key not configured")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("captcha token is required")
	}

	form := url.Values{}
	form.Set("secret", secret)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.verifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("captcha verify: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("captcha verify: http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var body struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return fmt.Errorf("captcha verify: decode response: %w", err)
	}
	if !body.Success {
		s.logger.Warn("captcha verification failed",
			slog.Any("error_codes", body.ErrorCodes),
			slog.String("remote_ip", remoteIP))
		return fmt.Errorf("captcha verification failed")
	}
	return nil
}

func (s *captchaService) resolveSecret(ctx context.Context) string {
	if s.settings != nil {
		if has, err := s.settings.HasTenantOverride(ctx, configModel.KeyEnrollmentCaptchaSecretKey); err == nil && has {
			if v, err := s.settings.ResolveString(ctx, configModel.KeyEnrollmentCaptchaSecretKey); err == nil && v != "" {
				return v
			}
		}
	}
	return strings.TrimSpace(os.Getenv("ENROLLMENT_CAPTCHA_SECRET_KEY"))
}
