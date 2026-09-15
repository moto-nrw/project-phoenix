package enrollment

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config"
)

// CaptchaSettingsResolver is the narrow contract the captcha service
// needs from the platform settings service, keeping the DI surface small
// and test stubs cheap.
type CaptchaSettingsResolver interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
}

// CaptchaServiceConfig is the dependency-injection bundle.
type CaptchaServiceConfig struct {
	Settings   CaptchaSettingsResolver
	Logger     *slog.Logger
	HTTPClient *http.Client
	VerifyURL  string // override for tests; defaults to Turnstile siteverify URL

	RequireCaptcha bool
	SecretKey      string
	SiteKey        string
}

const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// CaptchaService verifies a parent-submitted captcha token against the
// configured provider. Today: Cloudflare Turnstile. The plan keeps the
// abstraction so hCaptcha or another provider can drop in later
// without changing the call sites in the submission service.
//
// Verify returns nil when the captcha is valid, or an error explaining
// why it failed. Returns nil unconditionally when captcha is disabled
// for the tenant (settings-driven via enrollment.require_captcha).
type CaptchaService struct {
	settings   CaptchaSettingsResolver
	logger     *slog.Logger
	httpClient *http.Client
	verifyURL  string
	fallbacks  captchaFallbacks
}

type captchaFallbacks struct {
	requireCaptcha bool
	secretKey      string
	siteKey        string
}

// NewCaptchaService wires a Turnstile-backed verifier. A nil HTTPClient
// falls back to a 10-second-timeout client. Empty VerifyURL falls back
// to the canonical Turnstile endpoint.
func NewCaptchaService(cfg CaptchaServiceConfig) *CaptchaService {
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
	return &CaptchaService{
		settings:   cfg.Settings,
		logger:     logger,
		httpClient: httpClient,
		verifyURL:  verifyURL,
		fallbacks: captchaFallbacks{
			requireCaptcha: cfg.RequireCaptcha,
			secretKey:      strings.TrimSpace(cfg.SecretKey),
			siteKey:        strings.TrimSpace(cfg.SiteKey),
		},
	}
}

// IsEnabled checks the enrollment.require_captcha setting via the
// standard tenant override, env var, registry default chain.
// Default false: a fresh tenant has no Turnstile keys configured yet.
func (s *CaptchaService) IsEnabled(ctx context.Context) bool {
	if s.settings == nil {
		return false
	}
	return config.ResolveBoolOrDefault(ctx, s.settings, configModel.KeyEnrollmentRequireCaptcha, s.fallbacks.requireCaptcha, nil)
}

// Verify validates `token` against the configured provider for the tenant in
// context by hitting the provider's siteverify endpoint with secret + token +
// remote IP. `remoteIP` is the parent's source IP - passed through to the
// provider as a defense-in-depth signal. Returns nil on success.
func (s *CaptchaService) Verify(ctx context.Context, token, remoteIP string) error {
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

func (s *CaptchaService) resolveSecret(ctx context.Context) string {
	return config.ResolveStringOrDefault(ctx, s.settings, configModel.KeyEnrollmentCaptchaSecretKey, s.fallbacks.secretKey, nil)
}

// SiteKey returns the public Cloudflare Turnstile site key for the tenant in
// context, or "" when unset, via the same tenant override → env var
// (ENROLLMENT_CAPTCHA_SITE_KEY) fallback chain used by the secret key. Safe
// to expose on a public endpoint — it's the same value that lives in the
// rendered widget markup.
func (s *CaptchaService) SiteKey(ctx context.Context) string {
	return config.ResolveStringOrDefault(ctx, s.settings, configModel.KeyEnrollmentCaptchaSiteKey, s.fallbacks.siteKey, nil)
}
