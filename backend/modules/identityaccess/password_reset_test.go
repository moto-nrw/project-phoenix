package identityaccess_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The reset routes of all three portals read Retry-After from this error, and
// the frontend countdown reads that header. The cases below pin the number
// the header carries (#3332: moved here with the capability).
func TestPasswordResetRateLimitError(t *testing.T) {
	t.Parallel()

	t.Run("reports the rate limit message", func(t *testing.T) {
		t.Parallel()
		limited := &identityaccess.PasswordResetRateLimitError{Attempts: 3, RetryAt: time.Now().Add(time.Hour)}
		assert.Equal(t, "too many password reset requests", limited.Error())
	})

	t.Run("unwraps to the rate limit sentinel", func(t *testing.T) {
		t.Parallel()
		limited := &identityaccess.PasswordResetRateLimitError{Attempts: 3, RetryAt: time.Now()}
		assert.True(t, errors.Is(limited, identityaccess.ErrPasswordResetRateLimited))
	})
}

func TestPasswordResetRateLimitErrorRetryAfterSeconds(t *testing.T) {
	t.Parallel()

	t.Run("returns seconds until retry when RetryAt is in future", func(t *testing.T) {
		t.Parallel()
		now := time.Now()
		limited := &identityaccess.PasswordResetRateLimitError{Attempts: 3, RetryAt: now.Add(30 * time.Second)}
		assert.Equal(t, 30, limited.RetryAfterSeconds(now))
	})

	t.Run("returns zero when RetryAt is in past", func(t *testing.T) {
		t.Parallel()
		now := time.Now()
		limited := &identityaccess.PasswordResetRateLimitError{Attempts: 3, RetryAt: now.Add(-10 * time.Second)}
		assert.Equal(t, 0, limited.RetryAfterSeconds(now))
	})

	t.Run("returns zero when RetryAt is exactly now", func(t *testing.T) {
		t.Parallel()
		now := time.Now()
		limited := &identityaccess.PasswordResetRateLimitError{Attempts: 3, RetryAt: now}
		assert.Equal(t, 0, limited.RetryAfterSeconds(now))
	})

	t.Run("returns zero when RetryAt is zero value", func(t *testing.T) {
		t.Parallel()
		limited := &identityaccess.PasswordResetRateLimitError{Attempts: 3}
		assert.Equal(t, 0, limited.RetryAfterSeconds(time.Now()))
	})

	t.Run("returns zero when error is nil", func(t *testing.T) {
		t.Parallel()
		var limited *identityaccess.PasswordResetRateLimitError
		assert.Equal(t, 0, limited.RetryAfterSeconds(time.Now()))
	})

	t.Run("rounds down partial seconds", func(t *testing.T) {
		t.Parallel()
		now := time.Now()
		limited := &identityaccess.PasswordResetRateLimitError{
			Attempts: 3,
			RetryAt:  now.Add(45*time.Second + 600*time.Millisecond),
		}
		assert.Equal(t, 45, limited.RetryAfterSeconds(now))
	})
}
