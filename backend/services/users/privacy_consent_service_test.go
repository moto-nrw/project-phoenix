package users

import (
	"context"
	"testing"
	"time"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// fakeConsentSettings is a stub ConsentSettings for unit testing the
// privacy-consent service without a database.
type fakeConsentSettings struct {
	hasOverride bool
	overrideVal int
	hasErr      error
	resolveErr  error
}

func (f fakeConsentSettings) HasTenantOverride(_ context.Context, _ string) (bool, error) {
	return f.hasOverride, f.hasErr
}

func (f fakeConsentSettings) ResolveInt(_ context.Context, _ string) (int, error) {
	return f.overrideVal, f.resolveErr
}

func TestPrivacyConsentService_Accept(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	days30 := 30
	svc := NewPrivacyConsentService(nil, nil)

	consent := &userModels.PrivacyConsent{
		StudentID:     1,
		PolicyVersion: "1.0",
		Accepted:      false,
		DurationDays:  &days30,
	}

	svc.Accept(consent, now)

	if !consent.Accepted {
		t.Errorf("Accept() failed to set Accepted to true")
	}
	if consent.AcceptedAt == nil || !consent.AcceptedAt.Equal(now) {
		t.Errorf("Accept() failed to set AcceptedAt to now, got %v", consent.AcceptedAt)
	}
	if consent.ExpiresAt == nil {
		t.Fatalf("Accept() failed to derive ExpiresAt")
	}
	want := now.AddDate(0, 0, days30)
	if !consent.ExpiresAt.Equal(want) {
		t.Errorf("Accept() derived ExpiresAt = %v, want %v", consent.ExpiresAt, want)
	}
}

func TestPrivacyConsentService_Accept_NoDuration(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	svc := NewPrivacyConsentService(nil, nil)

	consent := &userModels.PrivacyConsent{StudentID: 1, PolicyVersion: "1.0"}
	svc.Accept(consent, now)

	if consent.ExpiresAt != nil {
		t.Errorf("Accept() with no duration should not set ExpiresAt, got %v", consent.ExpiresAt)
	}
}

func TestPrivacyConsentService_DeriveExpiry(t *testing.T) {
	t.Parallel()

	accepted := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	days10 := 10
	svc := NewPrivacyConsentService(nil, nil)

	t.Run("derives from duration", func(t *testing.T) {
		consent := &userModels.PrivacyConsent{AcceptedAt: &accepted, DurationDays: &days10}
		svc.DeriveExpiry(consent)
		if consent.ExpiresAt == nil || !consent.ExpiresAt.Equal(accepted.AddDate(0, 0, days10)) {
			t.Errorf("DeriveExpiry() = %v, want %v", consent.ExpiresAt, accepted.AddDate(0, 0, days10))
		}
	})

	t.Run("no duration leaves expiry unset", func(t *testing.T) {
		consent := &userModels.PrivacyConsent{AcceptedAt: &accepted}
		svc.DeriveExpiry(consent)
		if consent.ExpiresAt != nil {
			t.Errorf("DeriveExpiry() should leave ExpiresAt nil, got %v", consent.ExpiresAt)
		}
	})

	t.Run("existing expiry preserved", func(t *testing.T) {
		existing := accepted.AddDate(0, 0, 99)
		consent := &userModels.PrivacyConsent{AcceptedAt: &accepted, DurationDays: &days10, ExpiresAt: &existing}
		svc.DeriveExpiry(consent)
		if !consent.ExpiresAt.Equal(existing) {
			t.Errorf("DeriveExpiry() overwrote existing expiry, got %v", consent.ExpiresAt)
		}
	})
}

func TestPrivacyConsentService_KeyConstant(t *testing.T) {
	t.Parallel()

	if configModel.KeyPrivacyConsentRetentionDays != "gdpr.privacy_consent_retention_days" {
		t.Errorf("unexpected setting key: %s", configModel.KeyPrivacyConsentRetentionDays)
	}
}

// The tenant default is what the student privacy response falls back to when a
// consent carries no retention window of its own.
func TestPrivacyConsentService_DefaultDataRetentionDays(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("nil settings falls back to constant default", func(t *testing.T) {
		svc := NewPrivacyConsentService(nil, nil)
		if got := svc.DefaultDataRetentionDays(ctx); got != userModels.DefaultDataRetentionDays {
			t.Errorf("DefaultDataRetentionDays() = %v, want %v", got, userModels.DefaultDataRetentionDays)
		}
	})

	t.Run("no override falls back to constant default", func(t *testing.T) {
		svc := NewPrivacyConsentService(fakeConsentSettings{hasOverride: false}, nil)
		if got := svc.DefaultDataRetentionDays(ctx); got != userModels.DefaultDataRetentionDays {
			t.Errorf("DefaultDataRetentionDays() = %v, want %v", got, userModels.DefaultDataRetentionDays)
		}
	})

	t.Run("tenant override applied", func(t *testing.T) {
		svc := NewPrivacyConsentService(fakeConsentSettings{hasOverride: true, overrideVal: 21}, nil)
		if got := svc.DefaultDataRetentionDays(ctx); got != 21 {
			t.Errorf("DefaultDataRetentionDays() = %v, want 21", got)
		}
	})
}
