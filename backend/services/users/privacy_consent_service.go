package users

import (
	"context"
	"log/slog"
	"time"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/config"
)

// ConsentSettings is the narrow settings surface the privacy-consent service
// needs to resolve the per-tenant default data-retention window.
type ConsentSettings interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// PrivacyConsentService owns the GDPR privacy-consent business decisions that
// used to live on the model (issue #586, Rule 12). All time decisions take an
// explicit `now` so they are deterministic and testable; the data-retention
// default is resolved from the per-tenant settings registry.
type PrivacyConsentService struct {
	settings ConsentSettings
	logger   *slog.Logger
}

// NewPrivacyConsentService creates a privacy-consent service. settings may be
// nil (the constant default is then always used).
func NewPrivacyConsentService(settings ConsentSettings, logger *slog.Logger) *PrivacyConsentService {
	if logger == nil {
		logger = slog.Default()
	}
	return &PrivacyConsentService{settings: settings, logger: logger}
}

// Accept marks a consent accepted at `now` and derives the expiry date from the
// configured duration. The caller is responsible for persisting the result.
func (s *PrivacyConsentService) Accept(consent *userModels.PrivacyConsent, now time.Time) {
	consent.Accepted = true
	consent.AcceptedAt = &now
	s.DeriveExpiry(consent)
}

// Revoke flips a consent back to not-accepted. The caller persists the result.
func (s *PrivacyConsentService) Revoke(consent *userModels.PrivacyConsent) {
	consent.Accepted = false
}

// DeriveExpiry stamps ExpiresAt = AcceptedAt + DurationDays when a positive
// duration is set and no expiry is present yet. This is the consent-lifetime
// derivation that previously hid inside the model's Validate()/Accept().
func (s *PrivacyConsentService) DeriveExpiry(consent *userModels.PrivacyConsent) {
	if consent.DurationDays == nil || *consent.DurationDays <= 0 {
		return
	}
	if consent.ExpiresAt != nil || consent.AcceptedAt == nil {
		return
	}
	expiresAt := consent.AcceptedAt.AddDate(0, 0, *consent.DurationDays)
	consent.ExpiresAt = &expiresAt
}

// IsValid reports whether a consent is accepted and not expired as of `now`.
func (s *PrivacyConsentService) IsValid(consent *userModels.PrivacyConsent, now time.Time) bool {
	if !consent.Accepted {
		return false
	}
	if consent.ExpiresAt != nil && now.After(*consent.ExpiresAt) {
		return false
	}
	return true
}

// IsExpired reports whether a consent's expiry has passed as of `now`.
func (s *PrivacyConsentService) IsExpired(consent *userModels.PrivacyConsent, now time.Time) bool {
	if consent.ExpiresAt == nil {
		return false
	}
	return now.After(*consent.ExpiresAt)
}

// GetTimeToExpiry returns the duration from `now` until the consent expires, or
// nil when no expiry is set. An already-expired consent returns a zero
// duration.
func (s *PrivacyConsentService) GetTimeToExpiry(consent *userModels.PrivacyConsent, now time.Time) *time.Duration {
	if consent.ExpiresAt == nil {
		return nil
	}
	if now.After(*consent.ExpiresAt) {
		zero := time.Duration(0)
		return &zero
	}
	d := consent.ExpiresAt.Sub(now)
	return &d
}

// ResolveDataRetentionDays returns the retention window for a consent: the
// consent's own value when set, otherwise the per-tenant default resolved from
// settings (falling back to the package default). Resolution follows the
// HasTenantOverride pattern so an unset override falls through to the constant.
func (s *PrivacyConsentService) ResolveDataRetentionDays(ctx context.Context, consent *userModels.PrivacyConsent) int {
	if consent != nil && consent.DataRetentionDays > 0 {
		return consent.DataRetentionDays
	}
	return s.DefaultDataRetentionDays(ctx)
}

// DefaultDataRetentionDays resolves the per-tenant default retention window,
// falling back to the package constant when no override exists or settings are
// unavailable.
func (s *PrivacyConsentService) DefaultDataRetentionDays(ctx context.Context) int {
	days := config.ResolveIntOrDefault(ctx, s.settings, configModel.KeyPrivacyConsentRetentionDays, 0, s.logger)
	if days <= 0 {
		return userModels.DefaultDataRetentionDays
	}
	return days
}
