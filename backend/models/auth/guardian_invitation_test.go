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

func TestGuardianInvitation_TableName(t *testing.T) {
	inv := &GuardianInvitation{}
	if got := inv.TableName(); got != "auth.guardian_invitations" {
		t.Errorf("TableName() = %v, want auth.guardian_invitations", got)
	}
}

func TestGuardianInvitation_BeforeAppendModel(t *testing.T) {
	// BeforeAppendModel modifies query table expressions for different query types
	// It doesn't set timestamps - those are handled by the base model or repository

	t.Run("handles nil query", func(t *testing.T) {
		inv := &GuardianInvitation{Token: "test", GuardianProfileID: 1, CreatedBy: 1, ExpiresAt: time.Now().Add(48 * time.Hour)}
		err := inv.BeforeAppendModel(nil)
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})

	t.Run("returns no error for unknown query type", func(t *testing.T) {
		inv := &GuardianInvitation{Token: "test", GuardianProfileID: 1, CreatedBy: 1, ExpiresAt: time.Now().Add(48 * time.Hour)}
		err := inv.BeforeAppendModel("some string")
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})
}

func TestGuardianInvitation_GetID(t *testing.T) {
	inv := &GuardianInvitation{
		Model: base.Model{ID: 42},
	}

	// GetID returns interface{}, so we compare with int64
	if got, ok := inv.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", inv.GetID())
	}
}

func TestGuardianInvitation_GetCreatedAt(t *testing.T) {
	now := time.Now()
	inv := &GuardianInvitation{
		Model: base.Model{CreatedAt: now},
	}

	if got := inv.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestGuardianInvitation_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	inv := &GuardianInvitation{
		Model: base.Model{UpdatedAt: now},
	}

	if got := inv.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
