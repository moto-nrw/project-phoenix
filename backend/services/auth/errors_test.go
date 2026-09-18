package auth

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test error variables have correct messages
func TestErrorVariables(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"ErrInvalidCredentials", ErrInvalidCredentials, "invalid username or password"},
		{"ErrAccountNotFound", ErrAccountNotFound, "account not found"},
		{"ErrAccountInactive", ErrAccountInactive, "account is inactive"},
		{"ErrEmailAlreadyExists", ErrEmailAlreadyExists, "Diese E-Mail-Adresse ist bereits registriert"},
		{"ErrUsernameAlreadyExists", ErrUsernameAlreadyExists, "Dieser Benutzername ist bereits vergeben"},
		{"ErrInvalidToken", ErrInvalidToken, "invalid token format"},
		{"ErrTokenExpired", ErrTokenExpired, "token has expired"},
		{"ErrTokenNotFound", ErrTokenNotFound, "token not found"},
		{"ErrPasswordTooWeak", ErrPasswordTooWeak, "password doesn't meet complexity requirements"},
		{"ErrPasswordMismatch", ErrPasswordMismatch, "passwords don't match"},
		{"ErrParentAccountNotFound", ErrParentAccountNotFound, "parent account not found"},
		{"ErrInvitationNotFound", ErrInvitationNotFound, "invitation not found"},
		{"ErrInvitationExpired", ErrInvitationExpired, "invitation has expired"},
		{"ErrInvitationUsed", ErrInvitationUsed, "invitation has already been used"},
		{"ErrInvitationNameRequired", ErrInvitationNameRequired, "first name and last name are required"},
		{"ErrAccountAlreadyHasTenantAccess", ErrAccountAlreadyHasTenantAccess, "account already has access to tenant"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

// Test error variables are distinct
func TestErrorVariablesAreDistinct(t *testing.T) {
	t.Parallel()

	errorVars := []error{
		ErrInvalidCredentials,
		ErrAccountNotFound,
		ErrAccountInactive,
		ErrEmailAlreadyExists,
		ErrUsernameAlreadyExists,
		ErrInvalidToken,
		ErrTokenExpired,
		ErrTokenNotFound,
		ErrPasswordTooWeak,
		ErrPasswordMismatch,
		ErrParentAccountNotFound,
		ErrInvitationNotFound,
		ErrInvitationExpired,
		ErrInvitationUsed,
		ErrInvitationNameRequired,
		ErrAccountAlreadyHasTenantAccess,
	}

	// Each error should be distinguishable with errors.Is
	for i, err1 := range errorVars {
		for j, err2 := range errorVars {
			if i == j {
				assert.True(t, errors.Is(err1, err2), "error should equal itself")
			} else {
				assert.False(t, errors.Is(err1, err2), "different errors should not be equal")
			}
		}
	}
}

// TestAuthError tests the AuthError type
func TestAuthError(t *testing.T) {
	t.Parallel()

	t.Run("Error with underlying error", func(t *testing.T) {
		underlyingErr := errors.New("database connection failed")
		authErr := &AuthError{
			Op:  "login",
			Err: underlyingErr,
		}

		expected := "auth error during login: database connection failed"
		assert.Equal(t, expected, authErr.Error())
	})

	t.Run("Error without underlying error", func(t *testing.T) {
		authErr := &AuthError{
			Op:  "validate",
			Err: nil,
		}

		expected := "auth error during validate"
		assert.Equal(t, expected, authErr.Error())
	})

	t.Run("Unwrap returns underlying error", func(t *testing.T) {
		underlyingErr := errors.New("test error")
		authErr := &AuthError{
			Op:  "test",
			Err: underlyingErr,
		}

		assert.Equal(t, underlyingErr, authErr.Unwrap())
	})

	t.Run("Unwrap returns nil when no underlying error", func(t *testing.T) {
		authErr := &AuthError{
			Op:  "test",
			Err: nil,
		}

		assert.Nil(t, authErr.Unwrap())
	})

	t.Run("errors.Is works with wrapped AuthError", func(t *testing.T) {
		authErr := &AuthError{
			Op:  "login",
			Err: ErrInvalidCredentials,
		}

		assert.True(t, errors.Is(authErr, ErrInvalidCredentials))
	})
}
