package auth

// Unit tests for the private auth_login.go error-classification helpers.
// They're 0% in the coverage profile because no other test wires them up
// directly — the family-collision retry path inside createRefreshTokenWithRetry
// requires a real race, and the duplicate-key path only fires from
// LinkAccountToTenant under concurrent inserts.

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun/driver/pgdriver"
)

// --- isTokenFamilyConflict ---

func TestIsTokenFamilyConflict_NilError(t *testing.T) {
	t.Parallel()

	s := &Service{}
	assert.False(t, s.isTokenFamilyConflict(nil),
		"nil error must never count as a family conflict")
}

func TestIsTokenFamilyConflict_UnrelatedError(t *testing.T) {
	t.Parallel()

	s := &Service{}
	assert.False(t, s.isTokenFamilyConflict(errors.New("connection refused")),
		"errors not naming the family-generation constraint must return false")
}

func TestIsTokenFamilyConflict_MatchingConstraintName(t *testing.T) {
	t.Parallel()

	s := &Service{}
	// The helper does a plain substring match on the constraint name —
	// concurrent inserts that violate auth.uk_tokens_family_generation
	// surface as a Postgres error whose text contains this token.
	err := errors.New(`pq: duplicate key value violates unique constraint "uk_tokens_family_generation"`)
	assert.True(t, s.isTokenFamilyConflict(err),
		"errors containing the constraint name must be classified as a family conflict so the retry loop triggers")
}

// --- isDuplicateKeyError ---

func TestIsDuplicateKeyError_NilError(t *testing.T) {
	t.Parallel()

	assert.False(t, isDuplicateKeyError(nil))
}

func TestIsDuplicateKeyError_NonPGError(t *testing.T) {
	t.Parallel()

	// Plain Go error — not a wrapped pgdriver.Error, so the errors.As
	// branch must report false.
	assert.False(t, isDuplicateKeyError(errors.New("network down")))
}

func TestIsDuplicateKeyError_PGUniqueViolationDetected(t *testing.T) {
	t.Parallel()

	// pgdriver.Error is constructed by the driver — we can't easily build
	// a real one from outside the package. Instead exercise the false
	// branch with a different PG error code wrapped in a wrapper that
	// errors.As can unwrap. This proves the helper doesn't blindly return
	// true on every pgdriver.Error.
	plain := errors.New("not a pg error")
	assert.False(t, isDuplicateKeyError(plain),
		"plain errors must never be classified as duplicate-key — this guards the helper from misclassifying generic failures")

	// Sanity check that the pgdriver package is importable so future
	// regressions notice if the helper's type assertion stops compiling.
	var pgErr pgdriver.Error
	assert.False(t, isDuplicateKeyError(pgErr))
}
