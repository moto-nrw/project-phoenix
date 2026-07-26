package platform

import (
	"errors"
	"testing"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// isUniqueViolation — DatabaseError wrapping tests
// =============================================================================

func TestIsUniqueViolation_WrappedInDatabaseError_UniqueViolation(t *testing.T) {
	// pgdriver.Error with code 23505 wrapped in a DatabaseError
	pgErr := newPgError("23505")
	dbErr := &modelBase.DatabaseError{Op: "create", Err: pgErr}
	assert.True(t, modelBase.IsUniqueViolation(dbErr), "should detect unique violation wrapped in DatabaseError")
}

func TestIsUniqueViolation_DirectPgError_UniqueViolation(t *testing.T) {
	// pgdriver.Error with code 23505 passed directly (not wrapped)
	pgErr := newPgError("23505")
	assert.True(t, modelBase.IsUniqueViolation(pgErr), "should detect unique violation from direct pgdriver.Error")
}

func TestIsUniqueViolation_NonUniqueConstraintPgError(t *testing.T) {
	// pgdriver.Error with code 23503 (foreign_key_violation) — not a unique violation
	pgErr := newPgError("23503")
	assert.False(t, modelBase.IsUniqueViolation(pgErr), "foreign key violation should not be detected as unique violation")
}

func TestIsUniqueViolation_DatabaseErrorWrappingGenericError(t *testing.T) {
	// DatabaseError wrapping a non-pgdriver error
	dbErr := &modelBase.DatabaseError{Op: "update", Err: errors.New("connection refused")}
	assert.False(t, modelBase.IsUniqueViolation(dbErr), "generic error wrapped in DatabaseError should return false")
}

func TestIsUniqueViolation_DatabaseErrorWrappingNilError(t *testing.T) {
	// DatabaseError with nil inner error
	dbErr := &modelBase.DatabaseError{Op: "delete", Err: nil}
	assert.False(t, modelBase.IsUniqueViolation(dbErr), "DatabaseError with nil inner error should return false")
}
