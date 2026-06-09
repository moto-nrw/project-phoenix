package platform

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/platform"
)

func TestOperatorInvitationTokenValidity(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	used := now.Add(-time.Minute)

	tests := []struct {
		name        string
		token       *platform.OperatorInvitationToken
		wantExpired bool
		wantValid   bool
	}{
		{"nil", nil, false, false},
		{"valid", &platform.OperatorInvitationToken{ExpiresAt: future}, false, true},
		{"used", &platform.OperatorInvitationToken{ExpiresAt: future, UsedAt: &used}, false, false},
		{"expired", &platform.OperatorInvitationToken{ExpiresAt: past}, true, false},
		{"expired and used", &platform.OperatorInvitationToken{ExpiresAt: past, UsedAt: &used}, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := OperatorInvitationTokenExpired(tt.token, now); got != tt.wantExpired {
				t.Errorf("OperatorInvitationTokenExpired() = %v, want %v", got, tt.wantExpired)
			}
			if got := OperatorInvitationTokenValid(tt.token, now); got != tt.wantValid {
				t.Errorf("OperatorInvitationTokenValid() = %v, want %v", got, tt.wantValid)
			}
		})
	}
}

func TestOperatorEmailChangeTokenExpired(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)

	if OperatorEmailChangeTokenExpired(nil, now) {
		t.Error("nil token must not be reported expired")
	}
	if !OperatorEmailChangeTokenExpired(&platform.OperatorEmailChangeToken{Expiry: now.Add(-time.Minute)}, now) {
		t.Error("past expiry must be expired")
	}
	if OperatorEmailChangeTokenExpired(&platform.OperatorEmailChangeToken{Expiry: now.Add(time.Minute)}, now) {
		t.Error("future expiry must not be expired")
	}
}

func TestOperatorMFAEmailChallengeExpired(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)

	if OperatorMFAEmailChallengeExpired(nil, now) {
		t.Error("nil challenge must not be reported expired")
	}
	if !OperatorMFAEmailChallengeExpired(&platform.OperatorMFAEmailChallenge{ExpiresAt: now.Add(-time.Second)}, now) {
		t.Error("past TTL must be expired")
	}
	if OperatorMFAEmailChallengeExpired(&platform.OperatorMFAEmailChallenge{ExpiresAt: now.Add(time.Second)}, now) {
		t.Error("future TTL must not be expired")
	}
}

func TestOperatorMFATrustedDeviceActive(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	revoked := now.Add(-time.Minute)

	tests := []struct {
		name        string
		dev         *platform.OperatorMFATrustedDevice
		wantExpired bool
		wantActive  bool
	}{
		{"nil", nil, false, false},
		{"active", &platform.OperatorMFATrustedDevice{ExpiresAt: future}, false, true},
		{"revoked", &platform.OperatorMFATrustedDevice{ExpiresAt: future, RevokedAt: &revoked}, false, false},
		{"expired", &platform.OperatorMFATrustedDevice{ExpiresAt: past}, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := OperatorMFATrustedDeviceExpired(tt.dev, now); got != tt.wantExpired {
				t.Errorf("OperatorMFATrustedDeviceExpired() = %v, want %v", got, tt.wantExpired)
			}
			if got := OperatorMFATrustedDeviceActive(tt.dev, now); got != tt.wantActive {
				t.Errorf("OperatorMFATrustedDeviceActive() = %v, want %v", got, tt.wantActive)
			}
		})
	}
}
