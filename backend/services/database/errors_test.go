package database

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDatabaseErrorVariables tests that error variables have correct messages
func TestDatabaseErrorVariables(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"ErrNoPermissions", ErrNoPermissions, "no permissions to view database statistics"},
		{"ErrServiceUnavailable", ErrServiceUnavailable, "database statistics service temporarily unavailable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

// TestDatabaseErrorsAreDistinct ensures errors can be uniquely identified
func TestDatabaseErrorsAreDistinct(t *testing.T) {
	errorVars := []error{
		ErrNoPermissions,
		ErrServiceUnavailable,
	}

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

// TestDatabaseErrorUsage demonstrates error usage in context
func TestDatabaseErrorUsage(t *testing.T) {
	t.Run("ErrNoPermissions can be wrapped", func(t *testing.T) {
		wrappedErr := errors.New("user 123 lacks permissions")
		finalErr := errors.Join(ErrNoPermissions, wrappedErr)

		assert.True(t, errors.Is(finalErr, ErrNoPermissions))
	})

	t.Run("ErrServiceUnavailable can be wrapped", func(t *testing.T) {
		wrappedErr := errors.New("database connection pool exhausted")
		finalErr := errors.Join(ErrServiceUnavailable, wrappedErr)

		assert.True(t, errors.Is(finalErr, ErrServiceUnavailable))
	})
}
