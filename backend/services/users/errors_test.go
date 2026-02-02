package users

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestUsersErrorVariables tests that error variables have correct messages
func TestUsersErrorVariables(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"ErrPersonNotFound", ErrPersonNotFound, "person not found"},
		{"ErrInvalidPersonData", ErrInvalidPersonData, "invalid person data"},
		{"ErrPersonIdentifierRequired", ErrPersonIdentifierRequired, "either tag ID or account ID is required"},
		{"ErrAccountNotFound", ErrAccountNotFound, "account not found"},
		{"ErrRFIDCardNotFound", ErrRFIDCardNotFound, "RFID card not found"},
		{"ErrAccountAlreadyLinked", ErrAccountAlreadyLinked, "account is already linked to another person"},
		{"ErrRFIDCardAlreadyLinked", ErrRFIDCardAlreadyLinked, "RFID card is already linked to another person"},
		{"ErrGuardianNotFound", ErrGuardianNotFound, "guardian not found"},
		{"ErrStaffNotFound", ErrStaffNotFound, "staff member not found"},
		{"ErrTeacherNotFound", ErrTeacherNotFound, "teacher not found"},
		{"ErrStaffAlreadyExists", ErrStaffAlreadyExists, "staff member already exists for this person"},
		{"ErrTeacherAlreadyExists", ErrTeacherAlreadyExists, "teacher already exists for this staff member"},
		{"ErrInvalidPIN", ErrInvalidPIN, "invalid staff PIN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

// TestUsersErrorsAreDistinct ensures each error can be identified uniquely
func TestUsersErrorsAreDistinct(t *testing.T) {
	errorVars := []error{
		ErrPersonNotFound,
		ErrInvalidPersonData,
		ErrPersonIdentifierRequired,
		ErrAccountNotFound,
		ErrRFIDCardNotFound,
		ErrAccountAlreadyLinked,
		ErrRFIDCardAlreadyLinked,
		ErrGuardianNotFound,
		ErrStaffNotFound,
		ErrTeacherNotFound,
		ErrStaffAlreadyExists,
		ErrTeacherAlreadyExists,
		ErrInvalidPIN,
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

// TestUsersError tests the UsersError custom error type
func TestUsersError(t *testing.T) {
	t.Run("Error message format", func(t *testing.T) {
		underlyingErr := errors.New("database connection failed")
		usersErr := &UsersError{
			Op:  "CreatePerson",
			Err: underlyingErr,
		}

		expected := "users.CreatePerson: database connection failed"
		assert.Equal(t, expected, usersErr.Error())
	})

	t.Run("Unwrap returns underlying error", func(t *testing.T) {
		underlyingErr := ErrPersonNotFound
		usersErr := &UsersError{
			Op:  "GetPerson",
			Err: underlyingErr,
		}

		assert.Equal(t, underlyingErr, usersErr.Unwrap())
	})

	t.Run("errors.Is works with wrapped UsersError", func(t *testing.T) {
		usersErr := &UsersError{
			Op:  "CreatePerson",
			Err: ErrInvalidPersonData,
		}

		assert.True(t, errors.Is(usersErr, ErrInvalidPersonData))
	})

	t.Run("errors.As works with UsersError", func(t *testing.T) {
		usersErr := &UsersError{
			Op:  "LinkAccount",
			Err: ErrAccountAlreadyLinked,
		}

		var targetErr *UsersError
		assert.True(t, errors.As(usersErr, &targetErr))
		assert.Equal(t, "LinkAccount", targetErr.Op)
	})

	t.Run("nested error wrapping", func(t *testing.T) {
		baseErr := ErrStaffNotFound
		usersErr := &UsersError{
			Op:  "GetStaffByID",
			Err: baseErr,
		}
		wrappedErr := errors.Join(errors.New("context: processing staff data"), usersErr)

		// Should still be able to identify the base error
		assert.True(t, errors.Is(wrappedErr, ErrStaffNotFound))

		// Should also be able to unwrap to UsersError
		var targetErr *UsersError
		assert.True(t, errors.As(wrappedErr, &targetErr))
		assert.Equal(t, "GetStaffByID", targetErr.Op)
	})
}
