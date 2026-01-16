package auth

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
)

func TestPasswordResetToken_Validate(t *testing.T) {
	futureTime := time.Now().Add(1 * time.Hour)
	pastTime := time.Now().Add(-1 * time.Hour)

	tests := []struct {
		name    string
		token   *PasswordResetToken
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid token",
			token: &PasswordResetToken{
				AccountID: 1,
				Token:     "valid-token-123",
				Expiry:    futureTime,
				Used:      false,
			},
			wantErr: false,
		},
		{
			name: "zero account ID",
			token: &PasswordResetToken{
				AccountID: 0,
				Token:     "valid-token-123",
				Expiry:    futureTime,
				Used:      false,
			},
			wantErr: true,
			errMsg:  "account ID is required",
		},
		{
			name: "negative account ID",
			token: &PasswordResetToken{
				AccountID: -1,
				Token:     "valid-token-123",
				Expiry:    futureTime,
				Used:      false,
			},
			wantErr: true,
			errMsg:  "account ID is required",
		},
		{
			name: "empty token",
			token: &PasswordResetToken{
				AccountID: 1,
				Token:     "",
				Expiry:    futureTime,
				Used:      false,
			},
			wantErr: true,
			errMsg:  "token value is required",
		},
		{
			name: "expired token",
			token: &PasswordResetToken{
				AccountID: 1,
				Token:     "expired-token-123",
				Expiry:    pastTime,
				Used:      false,
			},
			wantErr: true,
			errMsg:  "token has already expired",
		},
		{
			name: "used token",
			token: &PasswordResetToken{
				AccountID: 1,
				Token:     "used-token-123",
				Expiry:    futureTime,
				Used:      true,
			},
			wantErr: true,
			errMsg:  "token has already been used",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.token.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("PasswordResetToken.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && err.Error() != tt.errMsg {
				t.Errorf("PasswordResetToken.Validate() error = %q, want %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestPasswordResetToken_IsExpired(t *testing.T) {
	tests := []struct {
		name     string
		expiry   time.Time
		expected bool
	}{
		{
			name:     "not expired",
			expiry:   time.Now().Add(1 * time.Hour),
			expected: false,
		},
		{
			name:     "expired",
			expiry:   time.Now().Add(-1 * time.Hour),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &PasswordResetToken{
				AccountID: 1,
				Token:     "test-token",
				Expiry:    tt.expiry,
			}

			if got := token.IsExpired(); got != tt.expected {
				t.Errorf("PasswordResetToken.IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPasswordResetToken_IsValid(t *testing.T) {
	futureTime := time.Now().Add(1 * time.Hour)
	pastTime := time.Now().Add(-1 * time.Hour)

	tests := []struct {
		name     string
		expiry   time.Time
		used     bool
		expected bool
	}{
		{
			name:     "valid - not expired and not used",
			expiry:   futureTime,
			used:     false,
			expected: true,
		},
		{
			name:     "invalid - expired",
			expiry:   pastTime,
			used:     false,
			expected: false,
		},
		{
			name:     "invalid - used",
			expiry:   futureTime,
			used:     true,
			expected: false,
		},
		{
			name:     "invalid - expired and used",
			expiry:   pastTime,
			used:     true,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &PasswordResetToken{
				AccountID: 1,
				Token:     "test-token",
				Expiry:    tt.expiry,
				Used:      tt.used,
			}

			if got := token.IsValid(); got != tt.expected {
				t.Errorf("PasswordResetToken.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPasswordResetToken_MarkAsUsed(t *testing.T) {
	token := &PasswordResetToken{
		AccountID: 1,
		Token:     "test-token",
		Expiry:    time.Now().Add(1 * time.Hour),
		Used:      false,
	}

	token.MarkAsUsed()

	if !token.Used {
		t.Error("PasswordResetToken.MarkAsUsed() should set Used to true")
	}
}

func TestPasswordResetToken_SetExpiry(t *testing.T) {
	token := &PasswordResetToken{
		AccountID: 1,
		Token:     "test-token",
	}

	before := time.Now()
	token.SetExpiry(30 * time.Minute)
	after := time.Now()

	expectedMin := before.Add(30 * time.Minute)
	expectedMax := after.Add(30 * time.Minute)

	if token.Expiry.Before(expectedMin) || token.Expiry.After(expectedMax) {
		t.Errorf("PasswordResetToken.SetExpiry() set expiry to %v, expected between %v and %v",
			token.Expiry, expectedMin, expectedMax)
	}
}

func TestPasswordResetToken_TableName(t *testing.T) {
	token := &PasswordResetToken{}
	if got := token.TableName(); got != "auth.password_reset_tokens" {
		t.Errorf("TableName() = %v, want auth.password_reset_tokens", got)
	}
}

func TestPasswordResetToken_BeforeAppendModel(t *testing.T) {
	// BeforeAppendModel modifies query table expressions for different query types
	// It doesn't set timestamps - those are handled by the base model or repository

	t.Run("handles nil query", func(t *testing.T) {
		token := &PasswordResetToken{AccountID: 1, Token: "test", Expiry: time.Now().Add(time.Hour)}
		err := token.BeforeAppendModel(nil)
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})

	t.Run("returns no error for unknown query type", func(t *testing.T) {
		token := &PasswordResetToken{AccountID: 1, Token: "test", Expiry: time.Now().Add(time.Hour)}
		err := token.BeforeAppendModel("some string")
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})
}

func TestPasswordResetToken_GetID(t *testing.T) {
	token := &PasswordResetToken{
		Model: base.Model{ID: 42},
	}

	// GetID returns interface{}, so we compare with int64
	if got, ok := token.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", token.GetID())
	}
}

func TestPasswordResetToken_GetCreatedAt(t *testing.T) {
	now := time.Now()
	token := &PasswordResetToken{
		Model: base.Model{CreatedAt: now},
	}

	if got := token.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestPasswordResetToken_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	token := &PasswordResetToken{
		Model: base.Model{UpdatedAt: now},
	}

	if got := token.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
