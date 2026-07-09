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
