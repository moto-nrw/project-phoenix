package auth

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestGuardianInvitation_Validate(t *testing.T) {
	futureTime := time.Now().Add(48 * time.Hour)
	pastTime := time.Now().Add(-1 * time.Hour)

	tests := []struct {
		name    string
		inv     *GuardianInvitation
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid invitation",
			inv: &GuardianInvitation{
				Token:             "valid-token-uuid",
				GuardianProfileID: 1,
				CreatedBy:         1,
				ExpiresAt:         futureTime,
			},
			wantErr: false,
		},
		{
			name: "empty token",
			inv: &GuardianInvitation{
				Token:             "",
				GuardianProfileID: 1,
				CreatedBy:         1,
				ExpiresAt:         futureTime,
			},
			wantErr: true,
			errMsg:  "token is required",
		},
		{
			name: "whitespace only token",
			inv: &GuardianInvitation{
				Token:             "   ",
				GuardianProfileID: 1,
				CreatedBy:         1,
				ExpiresAt:         futureTime,
			},
			wantErr: true,
			errMsg:  "token is required",
		},
		{
			name: "zero guardian profile ID",
			inv: &GuardianInvitation{
				Token:             "valid-token-uuid",
				GuardianProfileID: 0,
				CreatedBy:         1,
				ExpiresAt:         futureTime,
			},
			wantErr: true,
			errMsg:  "guardian profile ID is required",
		},
		{
			name: "negative guardian profile ID",
			inv: &GuardianInvitation{
				Token:             "valid-token-uuid",
				GuardianProfileID: -1,
				CreatedBy:         1,
				ExpiresAt:         futureTime,
			},
			wantErr: true,
			errMsg:  "guardian profile ID is required",
		},
		{
			name: "zero created by",
			inv: &GuardianInvitation{
				Token:             "valid-token-uuid",
				GuardianProfileID: 1,
				CreatedBy:         0,
				ExpiresAt:         futureTime,
			},
			wantErr: true,
			errMsg:  "created_by is required",
		},
		{
			name: "zero expires at",
			inv: &GuardianInvitation{
				Token:             "valid-token-uuid",
				GuardianProfileID: 1,
				CreatedBy:         1,
				ExpiresAt:         time.Time{},
			},
			wantErr: true,
			errMsg:  "expires_at is required",
		},
		{
			name: "expired invitation",
			inv: &GuardianInvitation{
				Token:             "valid-token-uuid",
				GuardianProfileID: 1,
				CreatedBy:         1,
				ExpiresAt:         pastTime,
			},
			wantErr: true,
			errMsg:  "invitation expiry must be in the future",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.inv.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("GuardianInvitation.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && err.Error() != tt.errMsg {
				t.Errorf("GuardianInvitation.Validate() error = %q, want %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestGuardianInvitation_TableName(t *testing.T) {
	inv := &GuardianInvitation{}
	expected := "auth.guardian_invitations"

	got := inv.TableName()
	if got != expected {
		t.Errorf("GuardianInvitation.TableName() = %q, want %q", got, expected)
	}
}

func TestGuardianInvitation_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		expected  bool
	}{
		{
			name:      "not expired",
			expiresAt: time.Now().Add(1 * time.Hour),
			expected:  false,
		},
		{
			name:      "expired",
			expiresAt: time.Now().Add(-1 * time.Hour),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := &GuardianInvitation{
				Token:             "test-token",
				GuardianProfileID: 1,
				CreatedBy:         1,
				ExpiresAt:         tt.expiresAt,
			}

			if got := inv.IsExpired(); got != tt.expected {
				t.Errorf("GuardianInvitation.IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGuardianInvitation_IsAccepted(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		acceptedAt *time.Time
		expected   bool
	}{
		{
			name:       "not accepted",
			acceptedAt: nil,
			expected:   false,
		},
		{
			name:       "accepted",
			acceptedAt: &now,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := &GuardianInvitation{
				Token:             "test-token",
				GuardianProfileID: 1,
				CreatedBy:         1,
				ExpiresAt:         time.Now().Add(48 * time.Hour),
				AcceptedAt:        tt.acceptedAt,
			}

			if got := inv.IsAccepted(); got != tt.expected {
				t.Errorf("GuardianInvitation.IsAccepted() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGuardianInvitation_IsValid(t *testing.T) {
	now := time.Now()
	futureTime := time.Now().Add(48 * time.Hour)
	pastTime := time.Now().Add(-1 * time.Hour)

	tests := []struct {
		name       string
		expiresAt  time.Time
		acceptedAt *time.Time
		expected   bool
	}{
		{
			name:       "valid - not expired and not accepted",
			expiresAt:  futureTime,
			acceptedAt: nil,
			expected:   true,
		},
		{
			name:       "invalid - expired",
			expiresAt:  pastTime,
			acceptedAt: nil,
			expected:   false,
		},
		{
			name:       "invalid - accepted",
			expiresAt:  futureTime,
			acceptedAt: &now,
			expected:   false,
		},
		{
			name:       "invalid - expired and accepted",
			expiresAt:  pastTime,
			acceptedAt: &now,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := &GuardianInvitation{
				Token:             "test-token",
				GuardianProfileID: 1,
				CreatedBy:         1,
				ExpiresAt:         tt.expiresAt,
				AcceptedAt:        tt.acceptedAt,
			}

			if got := inv.IsValid(); got != tt.expected {
				t.Errorf("GuardianInvitation.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGuardianInvitation_MarkAsAccepted(t *testing.T) {
	inv := &GuardianInvitation{
		Token:             "test-token",
		GuardianProfileID: 1,
		CreatedBy:         1,
		ExpiresAt:         time.Now().Add(48 * time.Hour),
	}

	if inv.AcceptedAt != nil {
		t.Error("AcceptedAt should be nil initially")
	}

	before := time.Now()
	inv.MarkAsAccepted()
	after := time.Now()

	if inv.AcceptedAt == nil {
		t.Error("AcceptedAt should not be nil after MarkAsAccepted")
	}

	if inv.AcceptedAt.Before(before) || inv.AcceptedAt.After(after) {
		t.Errorf("AcceptedAt = %v, expected between %v and %v", inv.AcceptedAt, before, after)
	}
}

func TestGuardianInvitation_SetExpiry(t *testing.T) {
	inv := &GuardianInvitation{
		Token:             "test-token",
		GuardianProfileID: 1,
		CreatedBy:         1,
	}

	before := time.Now()
	inv.SetExpiry(48 * time.Hour)
	after := time.Now()

	expectedMin := before.Add(48 * time.Hour)
	expectedMax := after.Add(48 * time.Hour)

	if inv.ExpiresAt.Before(expectedMin) || inv.ExpiresAt.After(expectedMax) {
		t.Errorf("ExpiresAt = %v, expected between %v and %v", inv.ExpiresAt, expectedMin, expectedMax)
	}
}

func TestGuardianInvitation_EntityInterface(t *testing.T) {
	now := time.Now()
	inv := &GuardianInvitation{
		Model: base.Model{
			ID:        123,
			CreatedAt: now,
			UpdatedAt: now.Add(time.Hour),
		},
		Token:             "test-token",
		GuardianProfileID: 1,
		CreatedBy:         1,
		ExpiresAt:         now.Add(48 * time.Hour),
	}

	t.Run("GetID", func(t *testing.T) {
		got := inv.GetID()
		if got != int64(123) {
			t.Errorf("GuardianInvitation.GetID() = %v, want %v", got, int64(123))
		}
	})

	t.Run("GetCreatedAt", func(t *testing.T) {
		got := inv.GetCreatedAt()
		if !got.Equal(now) {
			t.Errorf("GuardianInvitation.GetCreatedAt() = %v, want %v", got, now)
		}
	})

	t.Run("GetUpdatedAt", func(t *testing.T) {
		expected := now.Add(time.Hour)
		got := inv.GetUpdatedAt()
		if !got.Equal(expected) {
			t.Errorf("GuardianInvitation.GetUpdatedAt() = %v, want %v", got, expected)
		}
	})
}
