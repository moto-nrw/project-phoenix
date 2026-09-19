package studentpresence_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// The consent lifecycle moved here with users.privacy_consents' application
// logic (#3349); these cases came from the retired people-directory service.

func TestAcceptPrivacyConsent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	days30 := 30

	accepted := studentpresence.AcceptPrivacyConsent(studentpresence.PrivacyConsent{
		StudentID:     1,
		PolicyVersion: "1.0",
		DurationDays:  &days30,
	}, now)

	if !accepted.Accepted {
		t.Errorf("AcceptPrivacyConsent() failed to set Accepted to true")
	}
	if accepted.AcceptedAt == nil || !accepted.AcceptedAt.Equal(now) {
		t.Errorf("AcceptPrivacyConsent() failed to set AcceptedAt to now, got %v", accepted.AcceptedAt)
	}
	if accepted.ExpiresAt == nil {
		t.Fatalf("AcceptPrivacyConsent() failed to derive ExpiresAt")
	}
	want := now.AddDate(0, 0, days30)
	if !accepted.ExpiresAt.Equal(want) {
		t.Errorf("AcceptPrivacyConsent() derived ExpiresAt = %v, want %v", accepted.ExpiresAt, want)
	}
}

func TestAcceptPrivacyConsentWithoutDuration(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	accepted := studentpresence.AcceptPrivacyConsent(
		studentpresence.PrivacyConsent{StudentID: 1, PolicyVersion: "1.0"}, now)

	if accepted.ExpiresAt != nil {
		t.Errorf("AcceptPrivacyConsent() with no duration should not set ExpiresAt, got %v", accepted.ExpiresAt)
	}
}

func TestDerivePrivacyConsentExpiry(t *testing.T) {
	t.Parallel()

	accepted := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	days10 := 10

	t.Run("derives from duration", func(t *testing.T) {
		got := studentpresence.DerivePrivacyConsentExpiry(
			studentpresence.PrivacyConsent{AcceptedAt: &accepted, DurationDays: &days10})
		if got.ExpiresAt == nil || !got.ExpiresAt.Equal(accepted.AddDate(0, 0, days10)) {
			t.Errorf("DerivePrivacyConsentExpiry() = %v, want %v", got.ExpiresAt, accepted.AddDate(0, 0, days10))
		}
	})

	t.Run("no duration leaves expiry unset", func(t *testing.T) {
		got := studentpresence.DerivePrivacyConsentExpiry(
			studentpresence.PrivacyConsent{AcceptedAt: &accepted})
		if got.ExpiresAt != nil {
			t.Errorf("DerivePrivacyConsentExpiry() should leave ExpiresAt nil, got %v", got.ExpiresAt)
		}
	})

	t.Run("existing expiry preserved", func(t *testing.T) {
		existing := accepted.AddDate(0, 0, 99)
		got := studentpresence.DerivePrivacyConsentExpiry(studentpresence.PrivacyConsent{
			AcceptedAt: &accepted, DurationDays: &days10, ExpiresAt: &existing,
		})
		if !got.ExpiresAt.Equal(existing) {
			t.Errorf("DerivePrivacyConsentExpiry() overwrote existing expiry, got %v", got.ExpiresAt)
		}
	})

	t.Run("unaccepted consent keeps expiry unset", func(t *testing.T) {
		got := studentpresence.DerivePrivacyConsentExpiry(
			studentpresence.PrivacyConsent{DurationDays: &days10})
		if got.ExpiresAt != nil {
			t.Errorf("DerivePrivacyConsentExpiry() without AcceptedAt should leave ExpiresAt nil, got %v", got.ExpiresAt)
		}
	})
}

func TestDataRetentionDaysOrDefault(t *testing.T) {
	t.Parallel()

	if got := studentpresence.DataRetentionDaysOrDefault(0); got != studentpresence.DefaultPrivacyConsentRetentionDays {
		t.Errorf("DataRetentionDaysOrDefault(0) = %v, want %v", got, studentpresence.DefaultPrivacyConsentRetentionDays)
	}
	if got := studentpresence.DataRetentionDaysOrDefault(-3); got != studentpresence.DefaultPrivacyConsentRetentionDays {
		t.Errorf("DataRetentionDaysOrDefault(-3) = %v, want %v", got, studentpresence.DefaultPrivacyConsentRetentionDays)
	}
	if got := studentpresence.DataRetentionDaysOrDefault(21); got != 21 {
		t.Errorf("DataRetentionDaysOrDefault(21) = %v, want 21", got)
	}
}
