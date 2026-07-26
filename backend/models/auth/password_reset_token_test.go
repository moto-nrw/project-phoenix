package auth

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestPasswordResetToken_Validate(t *testing.T) {
	futureTime := time.Now().Add(1 * time.Hour)

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
