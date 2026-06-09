package auth

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/auth"
)

func TestTokenExpired(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)

	if TokenExpired(nil, now) {
		t.Error("nil token must not be reported expired")
	}
	if !TokenExpired(&auth.Token{Expiry: now.Add(-time.Minute)}, now) {
		t.Error("past expiry must be expired")
	}
	if TokenExpired(&auth.Token{Expiry: now.Add(time.Minute)}, now) {
		t.Error("future expiry must not be expired")
	}
	// zero expiry is before now
	if !TokenExpired(&auth.Token{Expiry: time.Time{}}, now) {
		t.Error("zero expiry must be expired")
	}
}

func TestPasswordResetTokenValidity(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	tests := []struct {
		name        string
		token       *auth.PasswordResetToken
		wantExpired bool
		wantValid   bool
	}{
		{"nil", nil, false, false},
		{"valid", &auth.PasswordResetToken{Expiry: future, Used: false}, false, true},
		{"used", &auth.PasswordResetToken{Expiry: future, Used: true}, false, false},
		{"expired", &auth.PasswordResetToken{Expiry: past, Used: false}, true, false},
		{"expired and used", &auth.PasswordResetToken{Expiry: past, Used: true}, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PasswordResetTokenExpired(tt.token, now); got != tt.wantExpired {
				t.Errorf("PasswordResetTokenExpired() = %v, want %v", got, tt.wantExpired)
			}
			if got := PasswordResetTokenValid(tt.token, now); got != tt.wantValid {
				t.Errorf("PasswordResetTokenValid() = %v, want %v", got, tt.wantValid)
			}
		})
	}
}

func TestInvitationTokenValidity(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	used := now.Add(-time.Minute)

	tests := []struct {
		name        string
		token       *auth.InvitationToken
		wantExpired bool
		wantValid   bool
	}{
		{"nil", nil, false, false},
		{"valid", &auth.InvitationToken{ExpiresAt: future}, false, true},
		{"used", &auth.InvitationToken{ExpiresAt: future, UsedAt: &used}, false, false},
		{"expired", &auth.InvitationToken{ExpiresAt: past}, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InvitationTokenExpired(tt.token, now); got != tt.wantExpired {
				t.Errorf("InvitationTokenExpired() = %v, want %v", got, tt.wantExpired)
			}
			if got := InvitationTokenValid(tt.token, now); got != tt.wantValid {
				t.Errorf("InvitationTokenValid() = %v, want %v", got, tt.wantValid)
			}
		})
	}
}

func TestGuardianInvitationValidity(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	accepted := now.Add(-time.Minute)

	tests := []struct {
		name        string
		inv         *auth.GuardianInvitation
		wantExpired bool
		wantValid   bool
	}{
		{"nil", nil, false, false},
		{"valid", &auth.GuardianInvitation{ExpiresAt: future}, false, true},
		{"accepted", &auth.GuardianInvitation{ExpiresAt: future, AcceptedAt: &accepted}, false, false},
		{"expired", &auth.GuardianInvitation{ExpiresAt: past}, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GuardianInvitationExpired(tt.inv, now); got != tt.wantExpired {
				t.Errorf("GuardianInvitationExpired() = %v, want %v", got, tt.wantExpired)
			}
			if got := GuardianInvitationValid(tt.inv, now); got != tt.wantValid {
				t.Errorf("GuardianInvitationValid() = %v, want %v", got, tt.wantValid)
			}
		})
	}
}

func TestMFAEmailChallengeExpired(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)

	if MFAEmailChallengeExpired(nil, now) {
		t.Error("nil challenge must not be reported expired")
	}
	if !MFAEmailChallengeExpired(&auth.MFAEmailChallenge{ExpiresAt: now.Add(-time.Second)}, now) {
		t.Error("past TTL must be expired")
	}
	if MFAEmailChallengeExpired(&auth.MFAEmailChallenge{ExpiresAt: now.Add(time.Second)}, now) {
		t.Error("future TTL must not be expired")
	}
}

func TestPasswordResetRateLimitExpired(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)

	if PasswordResetRateLimitExpired(nil, now) {
		t.Error("nil record must not be reported expired")
	}
	// window started just over an hour ago -> expired
	if !PasswordResetRateLimitExpired(&auth.PasswordResetRateLimit{WindowStart: now.Add(-PasswordResetRateLimitWindow - time.Second)}, now) {
		t.Error("elapsed window must be expired")
	}
	// window started well within the hour -> not expired
	if PasswordResetRateLimitExpired(&auth.PasswordResetRateLimit{WindowStart: now.Add(-time.Minute)}, now) {
		t.Error("fresh window must not be expired")
	}
}

func TestResetPasswordResetRateLimit(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	rec := &auth.PasswordResetRateLimit{Attempts: 5, WindowStart: now.Add(-2 * time.Hour)}

	ResetPasswordResetRateLimit(rec, now)

	if rec.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1 (reset counts as first attempt of new window)", rec.Attempts)
	}
	if !rec.WindowStart.Equal(now.UTC()) {
		t.Errorf("WindowStart = %v, want %v", rec.WindowStart, now.UTC())
	}

	// nil must not panic
	ResetPasswordResetRateLimit(nil, now)
}

func TestMFATrustedDeviceActive(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	revoked := now.Add(-time.Minute)

	tests := []struct {
		name        string
		dev         *auth.MFATrustedDevice
		wantExpired bool
		wantActive  bool
	}{
		{"nil", nil, false, false},
		{"active", &auth.MFATrustedDevice{ExpiresAt: future}, false, true},
		{"revoked", &auth.MFATrustedDevice{ExpiresAt: future, RevokedAt: &revoked}, false, false},
		{"expired", &auth.MFATrustedDevice{ExpiresAt: past}, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MFATrustedDeviceExpired(tt.dev, now); got != tt.wantExpired {
				t.Errorf("MFATrustedDeviceExpired() = %v, want %v", got, tt.wantExpired)
			}
			if got := MFATrustedDeviceActive(tt.dev, now); got != tt.wantActive {
				t.Errorf("MFATrustedDeviceActive() = %v, want %v", got, tt.wantActive)
			}
		})
	}
}
