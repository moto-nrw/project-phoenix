package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Test error variables have correct messages
func TestErrorVariables(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"ErrInvalidCredentials", ErrInvalidCredentials, "invalid username or password"},
		{"ErrAccountNotFound", ErrAccountNotFound, "account not found"},
		{"ErrAccountInactive", ErrAccountInactive, "account is inactive"},
		{"ErrEmailAlreadyExists", ErrEmailAlreadyExists, "email already exists"},
		{"ErrUsernameAlreadyExists", ErrUsernameAlreadyExists, "username already exists"},
		{"ErrInvalidToken", ErrInvalidToken, "invalid token format"},
		{"ErrTokenExpired", ErrTokenExpired, "token has expired"},
		{"ErrTokenNotFound", ErrTokenNotFound, "token not found"},
		{"ErrPasswordTooWeak", ErrPasswordTooWeak, "password doesn't meet complexity requirements"},
		{"ErrPasswordMismatch", ErrPasswordMismatch, "passwords don't match"},
		{"ErrRateLimitExceeded", ErrRateLimitExceeded, "too many password reset requests"},
		{"ErrPermissionNotFound", ErrPermissionNotFound, "permission not found"},
		{"ErrRoleNotFound", ErrRoleNotFound, "role not found"},
		{"ErrParentAccountNotFound", ErrParentAccountNotFound, "parent account not found"},
		{"ErrInvitationNotFound", ErrInvitationNotFound, "invitation not found"},
		{"ErrInvitationExpired", ErrInvitationExpired, "invitation has expired"},
		{"ErrInvitationUsed", ErrInvitationUsed, "invitation has already been used"},
		{"ErrInvitationNameRequired", ErrInvitationNameRequired, "first name and last name are required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

// Test error variables are distinct
func TestErrorVariablesAreDistinct(t *testing.T) {
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
		ErrRateLimitExceeded,
		ErrPermissionNotFound,
		ErrRoleNotFound,
		ErrParentAccountNotFound,
		ErrInvitationNotFound,
		ErrInvitationExpired,
		ErrInvitationUsed,
		ErrInvitationNameRequired,
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

// TestRateLimitError tests the RateLimitError type
func TestRateLimitError(t *testing.T) {
	t.Run("Error with underlying error", func(t *testing.T) {
		underlyingErr := errors.New("rate limit exceeded for user@example.com")
		rateLimitErr := &RateLimitError{
			Err:      underlyingErr,
			Attempts: 3,
			RetryAt:  time.Now().Add(time.Hour),
		}

		assert.Equal(t, underlyingErr.Error(), rateLimitErr.Error())
	})

	t.Run("Error without underlying error", func(t *testing.T) {
		rateLimitErr := &RateLimitError{
			Err:      nil,
			Attempts: 3,
			RetryAt:  time.Now().Add(time.Hour),
		}

		assert.Equal(t, "rate limit exceeded", rateLimitErr.Error())
	})

	t.Run("Unwrap returns underlying error", func(t *testing.T) {
		underlyingErr := ErrRateLimitExceeded
		rateLimitErr := &RateLimitError{
			Err:      underlyingErr,
			Attempts: 3,
			RetryAt:  time.Now(),
		}

		assert.Equal(t, underlyingErr, rateLimitErr.Unwrap())
	})

	t.Run("Unwrap returns nil when no underlying error", func(t *testing.T) {
		rateLimitErr := &RateLimitError{
			Err:      nil,
			Attempts: 3,
			RetryAt:  time.Now(),
		}

		assert.Nil(t, rateLimitErr.Unwrap())
	})
}

// TestRetryAfterSeconds tests the RetryAfterSeconds method
func TestRetryAfterSeconds(t *testing.T) {
	t.Run("returns seconds until retry when RetryAt is in future", func(t *testing.T) {
		now := time.Now()
		retryAt := now.Add(30 * time.Second)

		rateLimitErr := &RateLimitError{
			Err:      ErrRateLimitExceeded,
			Attempts: 3,
			RetryAt:  retryAt,
		}

		seconds := rateLimitErr.RetryAfterSeconds(now)
		assert.Equal(t, 30, seconds)
	})

	t.Run("returns zero when RetryAt is in past", func(t *testing.T) {
		now := time.Now()
		retryAt := now.Add(-10 * time.Second)

		rateLimitErr := &RateLimitError{
			Err:      ErrRateLimitExceeded,
			Attempts: 3,
			RetryAt:  retryAt,
		}

		seconds := rateLimitErr.RetryAfterSeconds(now)
		assert.Equal(t, 0, seconds)
	})

	t.Run("returns zero when RetryAt is exactly now", func(t *testing.T) {
		now := time.Now()

		rateLimitErr := &RateLimitError{
			Err:      ErrRateLimitExceeded,
			Attempts: 3,
			RetryAt:  now,
		}

		seconds := rateLimitErr.RetryAfterSeconds(now)
		assert.Equal(t, 0, seconds)
	})

	t.Run("returns zero when RetryAt is zero value", func(t *testing.T) {
		rateLimitErr := &RateLimitError{
			Err:      ErrRateLimitExceeded,
			Attempts: 3,
			RetryAt:  time.Time{},
		}

		seconds := rateLimitErr.RetryAfterSeconds(time.Now())
		assert.Equal(t, 0, seconds)
	})

	t.Run("returns zero when error is nil", func(t *testing.T) {
		var rateLimitErr *RateLimitError = nil
		seconds := rateLimitErr.RetryAfterSeconds(time.Now())
		assert.Equal(t, 0, seconds)
	})

	t.Run("rounds down partial seconds", func(t *testing.T) {
		now := time.Now()
		retryAt := now.Add(45*time.Second + 600*time.Millisecond)

		rateLimitErr := &RateLimitError{
			Err:      ErrRateLimitExceeded,
			Attempts: 3,
			RetryAt:  retryAt,
		}

		seconds := rateLimitErr.RetryAfterSeconds(now)
		assert.Equal(t, 45, seconds)
	})
}
