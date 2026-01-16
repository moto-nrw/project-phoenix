package auth

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
)

// Test helpers - local to avoid external dependencies
func stringPtr(s string) *string     { return &s }
func timePtr(t time.Time) *time.Time { return &t }

func TestAccount_Validate(t *testing.T) {
	tests := []struct {
		name    string
		account Account
		wantErr bool
	}{
		{
			name: "Valid account",
			account: Account{
				Email: "test@example.com",
			},
			wantErr: false,
		},
		{
			name: "Empty email",
			account: Account{
				Email: "",
			},
			wantErr: true,
		},
		{
			name: "Invalid email format",
			account: Account{
				Email: "invalid-email",
			},
			wantErr: true,
		},
		{
			name: "Email normalization",
			account: Account{
				Email: "TEST@Example.COM",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.account.Validate()

			// Check error condition
			if (err != nil) != tt.wantErr {
				t.Errorf("Account.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check email normalization
			if !tt.wantErr && tt.name == "Email normalization" && tt.account.Email != "test@example.com" {
				t.Errorf("Email was not normalized to lowercase, got %s", tt.account.Email)
			}
		})
	}
}

func TestAccount_HasRole(t *testing.T) {
	admin := &Role{Name: "admin"}
	// This variable is used to demonstrate a role that shouldn't match
	_ = &Role{Name: "user"}

	account := Account{
		Email: "test@example.com",
		Roles: []*Role{admin},
	}

	if !account.HasRole("admin") {
		t.Error("HasRole() should return true for 'admin'")
	}

	if account.HasRole("user") {
		t.Error("HasRole() should return false for 'user'")
	}

	// Test case insensitivity
	if !account.HasRole("ADMIN") {
		t.Error("HasRole() should be case-insensitive")
	}

	// Test with nil roles
	account.Roles = nil
	if account.HasRole("admin") {
		t.Error("HasRole() should return false when roles is nil")
	}
}

func TestAccount_HasPermission(t *testing.T) {
	readPerm := &Permission{Name: "read"}
	// This variable is used to demonstrate a permission that shouldn't match
	_ = &Permission{Name: "write"}

	account := Account{
		Email:       "test@example.com",
		Permissions: []*Permission{readPerm},
	}

	if !account.HasPermission("read") {
		t.Error("HasPermission() should return true for 'read'")
	}

	if account.HasPermission("write") {
		t.Error("HasPermission() should return false for 'write'")
	}

	// Test case insensitivity
	if !account.HasPermission("READ") {
		t.Error("HasPermission() should be case-insensitive")
	}

	// Test with nil permissions
	account.Permissions = nil
	if account.HasPermission("read") {
		t.Error("HasPermission() should return false when permissions is nil")
	}
}

func TestAccount_SetLastLogin(t *testing.T) {
	account := Account{
		Email: "test@example.com",
	}

	now := time.Now()
	account.SetLastLogin(now)

	if account.LastLogin == nil {
		t.Error("SetLastLogin() should set the LastLogin field")
	}

	if !account.LastLogin.Equal(now) {
		t.Errorf("LastLogin should equal %v, got %v", now, account.LastLogin)
	}
}

func TestAccount_IsActive(t *testing.T) {
	tests := []struct {
		name     string
		active   bool
		expected bool
	}{
		{
			name:     "Active account",
			active:   true,
			expected: true,
		},
		{
			name:     "Inactive account",
			active:   false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := Account{
				Email:  "test@example.com",
				Active: tt.active,
			}

			if got := account.IsActive(); got != tt.expected {
				t.Errorf("IsActive() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestAccount_HashPIN(t *testing.T) {
	account := &Account{
		Email: "test@example.com",
	}

	// Initially no PIN
	if account.PINHash != nil {
		t.Error("Account should have no PIN initially")
	}

	// Hash a PIN
	err := account.HashPIN("1234")
	if err != nil {
		t.Errorf("HashPIN() error = %v", err)
	}

	if account.PINHash == nil {
		t.Error("HashPIN() should set PINHash")
	}

	// PIN hash should not be the plain PIN
	if *account.PINHash == "1234" {
		t.Error("HashPIN() should hash the PIN, not store plain text")
	}
}

func TestAccount_VerifyPIN(t *testing.T) {
	account := &Account{
		Email: "test@example.com",
	}

	t.Run("no PIN set", func(t *testing.T) {
		if account.VerifyPIN("1234") {
			t.Error("VerifyPIN() should return false when no PIN is set")
		}
	})

	t.Run("correct PIN", func(t *testing.T) {
		err := account.HashPIN("1234")
		if err != nil {
			t.Fatalf("HashPIN() error = %v", err)
		}

		if !account.VerifyPIN("1234") {
			t.Error("VerifyPIN() should return true for correct PIN")
		}
	})

	t.Run("incorrect PIN", func(t *testing.T) {
		if account.VerifyPIN("9999") {
			t.Error("VerifyPIN() should return false for incorrect PIN")
		}
	})
}

func TestAccount_HasPIN(t *testing.T) {
	tests := []struct {
		name     string
		pinHash  *string
		expected bool
	}{
		{
			name:     "nil PIN hash",
			pinHash:  nil,
			expected: false,
		},
		{
			name:     "empty PIN hash",
			pinHash:  stringPtr(""),
			expected: false,
		},
		{
			name:     "valid PIN hash",
			pinHash:  stringPtr("$argon2id$v=19$somehash"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{
				Email:   "test@example.com",
				PINHash: tt.pinHash,
			}

			if got := account.HasPIN(); got != tt.expected {
				t.Errorf("HasPIN() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAccount_IsPINLocked(t *testing.T) {
	tests := []struct {
		name           string
		pinLockedUntil *time.Time
		expected       bool
	}{
		{
			name:           "nil locked until",
			pinLockedUntil: nil,
			expected:       false,
		},
		{
			name:           "locked until past",
			pinLockedUntil: timePtr(time.Now().Add(-1 * time.Hour)),
			expected:       false,
		},
		{
			name:           "locked until future",
			pinLockedUntil: timePtr(time.Now().Add(1 * time.Hour)),
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{
				Email:          "test@example.com",
				PINLockedUntil: tt.pinLockedUntil,
			}

			if got := account.IsPINLocked(); got != tt.expected {
				t.Errorf("IsPINLocked() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAccount_IncrementPINAttempts(t *testing.T) {
	t.Run("increments counter", func(t *testing.T) {
		account := &Account{
			Email:       "test@example.com",
			PINAttempts: 0,
		}

		account.IncrementPINAttempts()
		if account.PINAttempts != 1 {
			t.Errorf("PINAttempts = %d, want 1", account.PINAttempts)
		}

		account.IncrementPINAttempts()
		if account.PINAttempts != 2 {
			t.Errorf("PINAttempts = %d, want 2", account.PINAttempts)
		}
	})

	t.Run("locks after 5 attempts", func(t *testing.T) {
		account := &Account{
			Email:       "test@example.com",
			PINAttempts: 4, // 4 attempts already
		}

		// 5th attempt should lock
		account.IncrementPINAttempts()

		if account.PINAttempts != 5 {
			t.Errorf("PINAttempts = %d, want 5", account.PINAttempts)
		}

		if account.PINLockedUntil == nil {
			t.Error("PINLockedUntil should be set after 5 attempts")
		}

		if !account.IsPINLocked() {
			t.Error("Account should be locked after 5 attempts")
		}
	})

	t.Run("no lock before 5 attempts", func(t *testing.T) {
		account := &Account{
			Email:       "test@example.com",
			PINAttempts: 3, // 3 attempts already
		}

		// 4th attempt should not lock
		account.IncrementPINAttempts()

		if account.PINLockedUntil != nil {
			t.Error("PINLockedUntil should not be set before 5 attempts")
		}
	})
}

func TestAccount_ResetPINAttempts(t *testing.T) {
	futureTime := time.Now().Add(1 * time.Hour)
	account := &Account{
		Email:          "test@example.com",
		PINAttempts:    5,
		PINLockedUntil: &futureTime,
	}

	account.ResetPINAttempts()

	if account.PINAttempts != 0 {
		t.Errorf("PINAttempts = %d, want 0", account.PINAttempts)
	}

	if account.PINLockedUntil != nil {
		t.Error("PINLockedUntil should be nil after reset")
	}
}

func TestAccount_ClearPIN(t *testing.T) {
	pinHash := "$argon2id$v=19$somehash"
	futureTime := time.Now().Add(1 * time.Hour)
	account := &Account{
		Email:          "test@example.com",
		PINHash:        &pinHash,
		PINAttempts:    5,
		PINLockedUntil: &futureTime,
	}

	account.ClearPIN()

	if account.PINHash != nil {
		t.Error("PINHash should be nil after ClearPIN")
	}

	if account.PINAttempts != 0 {
		t.Errorf("PINAttempts = %d, want 0 after ClearPIN", account.PINAttempts)
	}

	if account.PINLockedUntil != nil {
		t.Error("PINLockedUntil should be nil after ClearPIN")
	}
}

func TestAccount_TableName(t *testing.T) {
	account := &Account{}
	if got := account.TableName(); got != "auth.accounts" {
		t.Errorf("TableName() = %v, want auth.accounts", got)
	}
}

func TestAccount_BeforeAppendModel(t *testing.T) {
	// BeforeAppendModel modifies query table expressions for different query types
	// It doesn't set timestamps - those are handled by the base model or repository

	t.Run("handles nil query", func(t *testing.T) {
		account := &Account{Email: "test@example.com"}
		err := account.BeforeAppendModel(nil)
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})

	t.Run("returns no error for unknown query type", func(t *testing.T) {
		account := &Account{Email: "test@example.com"}
		err := account.BeforeAppendModel("some string")
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})
}

func TestAccount_GetID(t *testing.T) {
	account := &Account{
		Model: base.Model{ID: 42},
		Email: "test@example.com",
	}

	// GetID returns interface{}, so we compare with int64
	if got, ok := account.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", account.GetID())
	}
}

func TestAccount_GetCreatedAt(t *testing.T) {
	now := time.Now()
	account := &Account{
		Model: base.Model{CreatedAt: now},
		Email: "test@example.com",
	}

	if got := account.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestAccount_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	account := &Account{
		Model: base.Model{UpdatedAt: now},
		Email: "test@example.com",
	}

	if got := account.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
