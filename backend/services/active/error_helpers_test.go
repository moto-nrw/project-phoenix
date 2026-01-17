package active

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
)

func TestIsNotFoundError(t *testing.T) {
	t.Run("returns true for DatabaseError with sql.ErrNoRows", func(t *testing.T) {
		err := &base.DatabaseError{
			Op:  "find by id",
			Err: sql.ErrNoRows,
		}
		assert.True(t, isNotFoundError(err))
	})

	t.Run("returns false for DatabaseError with other error", func(t *testing.T) {
		err := &base.DatabaseError{
			Op:  "find by id",
			Err: errors.New("connection refused"),
		}
		assert.False(t, isNotFoundError(err))
	})

	t.Run("returns false for non-DatabaseError", func(t *testing.T) {
		err := errors.New("some error")
		assert.False(t, isNotFoundError(err))
	})

	t.Run("returns false for nil error", func(t *testing.T) {
		assert.False(t, isNotFoundError(nil))
	})

	t.Run("returns true for wrapped DatabaseError with sql.ErrNoRows", func(t *testing.T) {
		dbErr := &base.DatabaseError{
			Op:  "find by id",
			Err: sql.ErrNoRows,
		}
		// Wrap the error
		wrappedErr := errors.Join(errors.New("context"), dbErr)
		// Note: errors.As will find the DatabaseError in the chain
		assert.True(t, isNotFoundError(wrappedErr))
	})
}
