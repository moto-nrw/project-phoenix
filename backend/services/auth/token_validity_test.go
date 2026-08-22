package auth

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/auth"
)

func TestInvitationTokenExpired(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	tests := []struct {
		name        string
		token       *auth.InvitationToken
		wantExpired bool
	}{
		{"nil", nil, false},
		{"valid", &auth.InvitationToken{ExpiresAt: future}, false},
		{"expired", &auth.InvitationToken{ExpiresAt: past}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InvitationTokenExpired(tt.token, now); got != tt.wantExpired {
				t.Errorf("InvitationTokenExpired() = %v, want %v", got, tt.wantExpired)
			}
		})
	}
}

func TestGuardianInvitationValidity(t *testing.T) {
	t.Parallel()

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
