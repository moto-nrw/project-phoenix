package platform

import (
	"errors"
	"testing"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// isRowsAffectedMismatch Tests
// ============================================================================

func TestIsRowsAffectedMismatch(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error returns false",
			err:  nil,
			want: false,
		},
		{
			name: "DatabaseError with rows affected message returns true",
			err:  &modelBase.DatabaseError{Op: "update", Err: errors.New("expected 1 rows affected, got 0")},
			want: true,
		},
		{
			name: "DatabaseError with other error returns false",
			err:  &modelBase.DatabaseError{Op: "update", Err: errors.New("other error")},
			want: false,
		},
		{
			name: "random error (not DatabaseError) returns false",
			err:  errors.New("expected 1 rows affected, got 0"),
			want: false,
		},
		{
			name: "DatabaseError with nil inner error returns false",
			err:  &modelBase.DatabaseError{Op: "update", Err: nil},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRowsAffectedMismatch(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ============================================================================
// isForeignKeyViolation Tests
// ============================================================================

func TestIsForeignKeyViolation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error returns false",
			err:  nil,
			want: false,
		},
		{
			name: "non-DatabaseError non-pgdriver error returns false",
			err:  errors.New("foreign key violated"),
			want: false,
		},
		{
			name: "DatabaseError wrapping non-pgdriver error returns false",
			err:  &modelBase.DatabaseError{Op: "delete", Err: errors.New("foreign key violated")},
			want: false,
		},
		{
			name: "pg 23503 foreign key violation",
			err:  newPgError("23503"),
			want: true,
		},
		{
			name: "pg 23505 unique violation is not foreign key",
			err:  newPgError("23505"),
			want: false,
		},
		{
			name: "DatabaseError wrapping pg 23503",
			err:  &modelBase.DatabaseError{Op: "delete", Err: newPgError("23503")},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isForeignKeyViolation(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ============================================================================
// isLookupNotFound — additional edge case
// ============================================================================

func TestIsSchoolLookupNotFound_DatabaseErrorWithOtherError(t *testing.T) {
	err := &modelBase.DatabaseError{Op: "find school", Err: errors.New("connection refused")}
	assert.False(t, isLookupNotFound(err))
}
