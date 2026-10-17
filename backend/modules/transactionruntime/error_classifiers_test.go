package transactionruntime_test

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	"github.com/stretchr/testify/assert"
)

func TestIsConstraintViolation(t *testing.T) {
	t.Parallel()

	t.Run("detects foreign key violation string", func(t *testing.T) {
		err := errors.New(`ERROR: update or delete on table "staff" violates foreign key constraint "fk_attendance_checked_in_by"`)
		assert.True(t, transactionruntime.IsConstraintViolation(err))
	})

	t.Run("detects not-null violation string from cascading SET NULL", func(t *testing.T) {
		err := errors.New(`ERROR: null value in column "tenant_id" of relation "groups" violates not-null constraint`)
		assert.True(t, transactionruntime.IsConstraintViolation(err))
	})

	t.Run("detects wrapped FK violation string", func(t *testing.T) {
		inner := errors.New(`violates foreign key constraint "fk_test"`)
		wrapped := fmt.Errorf("database error: %w", inner)
		assert.True(t, transactionruntime.IsConstraintViolation(wrapped))
	})

	t.Run("returns false for nil error", func(t *testing.T) {
		assert.False(t, transactionruntime.IsConstraintViolation(nil))
	})

	t.Run("returns false for unrelated error", func(t *testing.T) {
		err := errors.New("connection refused")
		assert.False(t, transactionruntime.IsConstraintViolation(err))
	})
}

func TestIsTransientDatabaseError_DetectsBadConnection(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("decision: list child offerings: %w", driver.ErrBadConn)

	assert.True(t, transactionruntime.IsTransientDatabaseError(err))
}

func TestIsTransientDatabaseError_IgnoresRequestContextCancellation(t *testing.T) {
	t.Parallel()

	assert.False(t, transactionruntime.IsTransientDatabaseError(context.Canceled))
	assert.False(t, transactionruntime.IsTransientDatabaseError(context.DeadlineExceeded))
}

func TestIsTransientDatabaseError_IgnoresDomainValidation(t *testing.T) {
	t.Parallel()

	err := errors.New("invalid session data: check-in time must be before check-out time")

	assert.False(t, transactionruntime.IsTransientDatabaseError(err))
}
