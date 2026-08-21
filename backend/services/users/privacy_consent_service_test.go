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

func TestPrivacyConsentService_Revoke(t *testing.T) {
	t.Parallel()

	now := time.Now()
	svc := NewPrivacyConsentService(nil, nil)
	consent := &userModels.PrivacyConsent{Accepted: true, AcceptedAt: &now}

	svc.Revoke(consent)
	if consent.Accepted {
		t.Errorf("Revoke() failed to set Accepted to false")
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

func TestPrivacyConsentService_IsValid(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	future := now.AddDate(0, 0, 30)
	past := now.AddDate(0, 0, -30)
	svc := NewPrivacyConsentService(nil, nil)

	tests := []struct {
		name     string
		pc       *userModels.PrivacyConsent
		expected bool
	}{
		{"valid", &userModels.PrivacyConsent{Accepted: true, AcceptedAt: &now, ExpiresAt: &future}, true},
		{"not accepted", &userModels.PrivacyConsent{Accepted: false, ExpiresAt: &future}, false},
		{"expired", &userModels.PrivacyConsent{Accepted: true, ExpiresAt: &past}, false},
		{"no expiration", &userModels.PrivacyConsent{Accepted: true, AcceptedAt: &now}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := svc.IsValid(tt.pc, now); got != tt.expected {
				t.Errorf("IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPrivacyConsentService_IsExpired(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	future := now.AddDate(0, 0, 30)
	past := now.AddDate(0, 0, -30)
	svc := NewPrivacyConsentService(nil, nil)

	tests := []struct {
		name     string
		pc       *userModels.PrivacyConsent
		expected bool
	}{
		{"not expired", &userModels.PrivacyConsent{ExpiresAt: &future}, false},
		{"expired", &userModels.PrivacyConsent{ExpiresAt: &past}, true},
		{"no expiration", &userModels.PrivacyConsent{ExpiresAt: nil}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := svc.IsExpired(tt.pc, now); got != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPrivacyConsentService_GetTimeToExpiry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	future := now.AddDate(0, 0, 30)
	past := now.AddDate(0, 0, -30)
	svc := NewPrivacyConsentService(nil, nil)

	t.Run("future expiry returns positive", func(t *testing.T) {
		got := svc.GetTimeToExpiry(&userModels.PrivacyConsent{ExpiresAt: &future}, now)
		if got == nil || *got <= 0 {
			t.Errorf("GetTimeToExpiry() = %v, want > 0", got)
		}
	})

	t.Run("past expiry returns zero", func(t *testing.T) {
		got := svc.GetTimeToExpiry(&userModels.PrivacyConsent{ExpiresAt: &past}, now)
		if got == nil || *got != 0 {
			t.Errorf("GetTimeToExpiry() = %v, want 0", got)
		}
	})

	t.Run("no expiry returns nil", func(t *testing.T) {
		if got := svc.GetTimeToExpiry(&userModels.PrivacyConsent{ExpiresAt: nil}, now); got != nil {
			t.Errorf("GetTimeToExpiry() = %v, want nil", got)
		}
	})
}

func TestPrivacyConsentService_ResolveDataRetentionDays(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("consent value wins when set", func(t *testing.T) {
		svc := NewPrivacyConsentService(fakeConsentSettings{hasOverride: true, overrideVal: 7}, nil)
		got := svc.ResolveDataRetentionDays(ctx, &userModels.PrivacyConsent{DataRetentionDays: 15})
		if got != 15 {
			t.Errorf("ResolveDataRetentionDays() = %v, want 15", got)
		}
	})

	t.Run("nil settings falls back to constant default", func(t *testing.T) {
		svc := NewPrivacyConsentService(nil, nil)
		got := svc.ResolveDataRetentionDays(ctx, &userModels.PrivacyConsent{DataRetentionDays: 0})
		if got != userModels.DefaultDataRetentionDays {
			t.Errorf("ResolveDataRetentionDays() = %v, want %v", got, userModels.DefaultDataRetentionDays)
		}
	})

	t.Run("no override falls back to constant default", func(t *testing.T) {
		svc := NewPrivacyConsentService(fakeConsentSettings{hasOverride: false}, nil)
		got := svc.ResolveDataRetentionDays(ctx, &userModels.PrivacyConsent{DataRetentionDays: 0})
		if got != userModels.DefaultDataRetentionDays {
			t.Errorf("ResolveDataRetentionDays() = %v, want %v", got, userModels.DefaultDataRetentionDays)
		}
	})

	t.Run("tenant override applied", func(t *testing.T) {
		svc := NewPrivacyConsentService(fakeConsentSettings{hasOverride: true, overrideVal: 21}, nil)
		got := svc.ResolveDataRetentionDays(ctx, &userModels.PrivacyConsent{DataRetentionDays: 0})
		if got != 21 {
			t.Errorf("ResolveDataRetentionDays() = %v, want 21", got)
		}
	})

	t.Run("nil consent uses default", func(t *testing.T) {
		svc := NewPrivacyConsentService(fakeConsentSettings{hasOverride: false}, nil)
		got := svc.ResolveDataRetentionDays(ctx, nil)
		if got != userModels.DefaultDataRetentionDays {
			t.Errorf("ResolveDataRetentionDays(nil) = %v, want %v", got, userModels.DefaultDataRetentionDays)
		}
	})
}

func TestPrivacyConsentService_KeyConstant(t *testing.T) {
	t.Parallel()

	if configModel.KeyPrivacyConsentRetentionDays != "gdpr.privacy_consent_retention_days" {
		t.Errorf("unexpected setting key: %s", configModel.KeyPrivacyConsentRetentionDays)
	}
}
