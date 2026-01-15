package auth

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestAccountParent_Validate(t *testing.T) {
	tests := []struct {
		name    string
		account *AccountParent
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid account",
			account: &AccountParent{
				Email: "parent@example.com",
			},
			wantErr: false,
		},
		{
			name: "empty email",
			account: &AccountParent{
				Email: "",
			},
			wantErr: true,
			errMsg:  "email is required",
		},
		{
			name: "invalid email format",
			account: &AccountParent{
				Email: "invalid-email",
			},
			wantErr: true,
			errMsg:  "invalid email format",
		},
		{
			name: "email normalization",
			account: &AccountParent{
				Email: "PARENT@Example.COM",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.account.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("AccountParent.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && err.Error() != tt.errMsg {
				t.Errorf("AccountParent.Validate() error = %q, want %q", err.Error(), tt.errMsg)
			}
			// Check email normalization
			if !tt.wantErr && tt.name == "email normalization" && tt.account.Email != "parent@example.com" {
				t.Errorf("Email was not normalized to lowercase, got %s", tt.account.Email)
			}
		})
	}
}

func TestAccountParent_IsActive(t *testing.T) {
	tests := []struct {
		name     string
		active   bool
		expected bool
	}{
		{
			name:     "active account",
			active:   true,
			expected: true,
		},
		{
			name:     "inactive account",
			active:   false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &AccountParent{
				Email:  "parent@example.com",
				Active: tt.active,
			}

			if got := account.IsActive(); got != tt.expected {
				t.Errorf("AccountParent.IsActive() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAccountParent_SetLastLogin(t *testing.T) {
	account := &AccountParent{
		Email: "parent@example.com",
	}

	if account.LastLogin != nil {
		t.Error("LastLogin should be nil initially")
	}

	now := time.Now()
	account.SetLastLogin(now)

	if account.LastLogin == nil {
		t.Error("LastLogin should not be nil after SetLastLogin")
	}

	if !account.LastLogin.Equal(now) {
		t.Errorf("LastLogin = %v, want %v", account.LastLogin, now)
	}
}

func TestAccountParent_TableName(t *testing.T) {
	ap := &AccountParent{}
	if got := ap.TableName(); got != "auth.accounts_parents" {
		t.Errorf("TableName() = %v, want auth.accounts_parents", got)
	}
}

func TestAccountParent_BeforeAppendModel(t *testing.T) {
	// BeforeAppendModel modifies query table expressions for different query types
	// It doesn't set timestamps - those are handled by the base model or repository

	t.Run("handles nil query", func(t *testing.T) {
		ap := &AccountParent{Email: "parent@example.com"}
		err := ap.BeforeAppendModel(nil)
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})

	t.Run("returns no error for unknown query type", func(t *testing.T) {
		ap := &AccountParent{Email: "parent@example.com"}
		err := ap.BeforeAppendModel("some string")
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})
}

func TestAccountParent_GetID(t *testing.T) {
	ap := &AccountParent{
		Model: base.Model{ID: 42},
		Email: "parent@example.com",
	}

	// GetID returns interface{}, so we compare with int64
	if got, ok := ap.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", ap.GetID())
	}
}

func TestAccountParent_GetCreatedAt(t *testing.T) {
	now := time.Now()
	ap := &AccountParent{
		Model: base.Model{CreatedAt: now},
		Email: "parent@example.com",
	}

	if got := ap.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestAccountParent_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	ap := &AccountParent{
		Model: base.Model{UpdatedAt: now},
		Email: "parent@example.com",
	}

	if got := ap.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
