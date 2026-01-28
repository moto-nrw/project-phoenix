package auth

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// NOTE: base package is used for StringPtr helper and Model struct

func TestInvitationToken_Validate(t *testing.T) {
	futureTime := time.Now().Add(48 * time.Hour)
	pastTime := time.Now().Add(-time.Hour)

	tests := []struct {
		name    string
		token   *InvitationToken
		wantErr bool
	}{
		{
			name: "valid invitation token",
			token: &InvitationToken{
				Email:     "invite@example.com",
				Token:     "abc123token",
				RoleID:    1,
				CreatedBy: 1,
				ExpiresAt: futureTime,
			},
			wantErr: false,
		},
		{
			name: "valid with optional fields",
			token: &InvitationToken{
				Email:     "invite@example.com",
				Token:     "abc123token",
				RoleID:    1,
				CreatedBy: 1,
				ExpiresAt: futureTime,
				FirstName: base.StringPtr("John"),
				LastName:  base.StringPtr("Doe"),
				Position:  base.StringPtr("Teacher"),
			},
			wantErr: false,
		},
		{
			name: "empty email",
			token: &InvitationToken{
				Email:     "",
				Token:     "abc123token",
				RoleID:    1,
				CreatedBy: 1,
				ExpiresAt: futureTime,
			},
			wantErr: true,
		},
		{
			name: "whitespace only email",
			token: &InvitationToken{
				Email:     "   ",
				Token:     "abc123token",
				RoleID:    1,
				CreatedBy: 1,
				ExpiresAt: futureTime,
			},
			wantErr: true,
		},
		{
			name: "empty token",
			token: &InvitationToken{
				Email:     "invite@example.com",
				Token:     "",
				RoleID:    1,
				CreatedBy: 1,
				ExpiresAt: futureTime,
			},
			wantErr: true,
		},
		{
			name: "zero role ID",
			token: &InvitationToken{
				Email:     "invite@example.com",
				Token:     "abc123token",
				RoleID:    0,
				CreatedBy: 1,
				ExpiresAt: futureTime,
			},
			wantErr: true,
		},
		{
			name: "negative role ID",
			token: &InvitationToken{
				Email:     "invite@example.com",
				Token:     "abc123token",
				RoleID:    -1,
				CreatedBy: 1,
				ExpiresAt: futureTime,
			},
			wantErr: true,
		},
		{
			name: "zero created by",
			token: &InvitationToken{
				Email:     "invite@example.com",
				Token:     "abc123token",
				RoleID:    1,
				CreatedBy: 0,
				ExpiresAt: futureTime,
			},
			wantErr: true,
		},
		{
			name: "zero expiry time",
			token: &InvitationToken{
				Email:     "invite@example.com",
				Token:     "abc123token",
				RoleID:    1,
				CreatedBy: 1,
				ExpiresAt: time.Time{},
			},
			wantErr: true,
		},
		{
			name: "expired invitation",
			token: &InvitationToken{
				Email:     "invite@example.com",
				Token:     "abc123token",
				RoleID:    1,
				CreatedBy: 1,
				ExpiresAt: pastTime,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.token.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("InvitationToken.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInvitationToken_IsExpired(t *testing.T) {
	tests := []struct {
		name     string
		token    *InvitationToken
		expected bool
	}{
		{
			name: "not expired",
			token: &InvitationToken{
				ExpiresAt: time.Now().Add(time.Hour),
			},
			expected: false,
		},
		{
			name: "expired",
			token: &InvitationToken{
				ExpiresAt: time.Now().Add(-time.Hour),
			},
			expected: true,
		},
		{
			name: "zero time is expired",
			token: &InvitationToken{
				ExpiresAt: time.Time{},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.token.IsExpired()
			if got != tt.expected {
				t.Errorf("InvitationToken.IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestInvitationToken_IsUsed(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		token    *InvitationToken
		expected bool
	}{
		{
			name: "not used - nil UsedAt",
			token: &InvitationToken{
				UsedAt: nil,
			},
			expected: false,
		},
		{
			name: "used - has UsedAt",
			token: &InvitationToken{
				UsedAt: &now,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.token.IsUsed()
			if got != tt.expected {
				t.Errorf("InvitationToken.IsUsed() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestInvitationToken_IsValid(t *testing.T) {
	now := time.Now()
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)

	tests := []struct {
		name     string
		token    *InvitationToken
		expected bool
	}{
		{
			name: "valid - not expired, not used",
			token: &InvitationToken{
				ExpiresAt: future,
				UsedAt:    nil,
			},
			expected: true,
		},
		{
			name: "invalid - expired",
			token: &InvitationToken{
				ExpiresAt: past,
				UsedAt:    nil,
			},
			expected: false,
		},
		{
			name: "invalid - used",
			token: &InvitationToken{
				ExpiresAt: future,
				UsedAt:    &now,
			},
			expected: false,
		},
		{
			name: "invalid - both expired and used",
			token: &InvitationToken{
				ExpiresAt: past,
				UsedAt:    &now,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.token.IsValid()
			if got != tt.expected {
				t.Errorf("InvitationToken.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestInvitationToken_MarkAsUsed(t *testing.T) {
	token := &InvitationToken{
		Email:     "test@example.com",
		Token:     "abc123",
		RoleID:    1,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(time.Hour),
		UsedAt:    nil,
	}

	if token.IsUsed() {
		t.Error("Token should not be used initially")
	}

	before := time.Now()
	token.MarkAsUsed()
	after := time.Now()

	if !token.IsUsed() {
		t.Error("Token should be marked as used")
	}

	if token.UsedAt == nil {
		t.Fatal("UsedAt should not be nil after MarkAsUsed")
	}

	if token.UsedAt.Before(before) || token.UsedAt.After(after) {
		t.Errorf("UsedAt = %v, want between %v and %v", token.UsedAt, before, after)
	}
}

func TestInvitationToken_SetExpiry(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
	}{
		{
			name:     "48 hour expiry",
			duration: 48 * time.Hour,
		},
		{
			name:     "1 hour expiry",
			duration: time.Hour,
		},
		{
			name:     "7 day expiry",
			duration: 7 * 24 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &InvitationToken{}

			before := time.Now()
			token.SetExpiry(tt.duration)
			after := time.Now()

			expectedMin := before.Add(tt.duration)
			expectedMax := after.Add(tt.duration)

			if token.ExpiresAt.Before(expectedMin) || token.ExpiresAt.After(expectedMax) {
				t.Errorf("InvitationToken.SetExpiry() expiry = %v, want between %v and %v",
					token.ExpiresAt, expectedMin, expectedMax)
			}
		})
	}
}

func TestInvitationToken_EmailTracking(t *testing.T) {
	now := time.Now()
	errMsg := "SMTP connection failed"

	t.Run("track email sent", func(t *testing.T) {
		token := &InvitationToken{
			Email:       "test@example.com",
			Token:       "abc123",
			EmailSentAt: &now,
		}

		if token.EmailSentAt == nil {
			t.Error("EmailSentAt should be set")
		}
	})

	t.Run("track email error", func(t *testing.T) {
		token := &InvitationToken{
			Email:           "test@example.com",
			Token:           "abc123",
			EmailError:      &errMsg,
			EmailRetryCount: 3,
		}

		if token.EmailError == nil || *token.EmailError != errMsg {
			t.Errorf("EmailError = %v, want %q", token.EmailError, errMsg)
		}

		if token.EmailRetryCount != 3 {
			t.Errorf("EmailRetryCount = %d, want 3", token.EmailRetryCount)
		}
	})
}

func TestInvitationToken_TableName(t *testing.T) {
	token := &InvitationToken{}
	if got := token.TableName(); got != "auth.invitation_tokens" {
		t.Errorf("TableName() = %v, want auth.invitation_tokens", got)
	}
}

func TestInvitationToken_BeforeAppendModel(t *testing.T) {
	// BeforeAppendModel modifies query table expressions for different query types
	// It doesn't set timestamps - those are handled by the base model or repository

	t.Run("handles nil query", func(t *testing.T) {
		token := &InvitationToken{Email: "test@example.com", Token: "test", RoleID: 1, CreatedBy: 1, ExpiresAt: time.Now().Add(48 * time.Hour)}
		err := token.BeforeAppendModel(nil)
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})

	t.Run("returns no error for unknown query type", func(t *testing.T) {
		token := &InvitationToken{Email: "test@example.com", Token: "test", RoleID: 1, CreatedBy: 1, ExpiresAt: time.Now().Add(48 * time.Hour)}
		err := token.BeforeAppendModel("some string")
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})
}

func TestInvitationToken_GetID(t *testing.T) {
	token := &InvitationToken{
		Model: base.Model{ID: 42},
	}

	// GetID returns interface{}, so we compare with int64
	if got, ok := token.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", token.GetID())
	}
}

func TestInvitationToken_GetCreatedAt(t *testing.T) {
	now := time.Now()
	token := &InvitationToken{
		Model: base.Model{CreatedAt: now},
	}

	if got := token.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestInvitationToken_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	token := &InvitationToken{
		Model: base.Model{UpdatedAt: now},
	}

	if got := token.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
