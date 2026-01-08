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

func TestAccountParent_TableName(t *testing.T) {
	account := &AccountParent{}
	expected := "auth.accounts_parents"

	got := account.TableName()
	if got != expected {
		t.Errorf("AccountParent.TableName() = %q, want %q", got, expected)
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

func TestAccountParent_EntityInterface(t *testing.T) {
	now := time.Now()
	account := &AccountParent{
		Model: base.Model{
			ID:        123,
			CreatedAt: now,
			UpdatedAt: now.Add(time.Hour),
		},
		Email: "parent@example.com",
	}

	t.Run("GetID", func(t *testing.T) {
		got := account.GetID()
		if got != int64(123) {
			t.Errorf("AccountParent.GetID() = %v, want %v", got, int64(123))
		}
	})

	t.Run("GetCreatedAt", func(t *testing.T) {
		got := account.GetCreatedAt()
		if !got.Equal(now) {
			t.Errorf("AccountParent.GetCreatedAt() = %v, want %v", got, now)
		}
	})

	t.Run("GetUpdatedAt", func(t *testing.T) {
		expected := now.Add(time.Hour)
		got := account.GetUpdatedAt()
		if !got.Equal(expected) {
			t.Errorf("AccountParent.GetUpdatedAt() = %v, want %v", got, expected)
		}
	})
}
