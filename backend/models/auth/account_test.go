package auth

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/ptrtest"
	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestAccount_Validate(t *testing.T) {
	t.Parallel()

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

// Note: the role/permission membership checks (formerly Account.HasRole /
// Account.HasPermission) and the PIN/MFA lockout helpers moved out of the
// model in issue #586 (Rule 12). Their tests moved with them:
//   - auth/authorize/account_roles_test.go (HasRole/HasPermission)
//   - services/auth (PIN/MFA lockout decisions)
//   - database/repositories/auth (atomic counter mutations)

func TestAccount_SetLastLogin(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
			pinHash:  ptrtest.Ptr(""),
			expected: false,
		},
		{
			name:     "valid PIN hash",
			pinHash:  ptrtest.Ptr("$argon2id$v=19$somehash"),
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

func TestAccount_GetID(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	now := time.Now()
	account := &Account{
		Model: base.Model{UpdatedAt: now},
		Email: "test@example.com",
	}

	if got := account.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
