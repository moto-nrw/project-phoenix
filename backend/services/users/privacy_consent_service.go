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
