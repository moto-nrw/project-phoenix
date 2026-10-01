package base

import (
	"errors"
	"testing"

	"github.com/uptrace/bun/driver/pgdriver"
)

// The detector must fail cleanly on errors that carry no unique violation:
// no panic and no false positive. Moved from the rollover helper tests when
// the rollover left services/enrollment (#3564).

func TestIsUniqueViolationOn_NilError(t *testing.T) {
	t.Parallel()

	if IsUniqueViolationOn(nil, "anything") {
		t.Fatal("IsUniqueViolationOn(nil) = true, want false")
	}
}

func TestIsUniqueViolationOn_NonPGError(t *testing.T) {
	t.Parallel()

	if IsUniqueViolationOn(errors.New("synthetic"), "anything") {
		t.Fatal("IsUniqueViolationOn(plain error) = true, want false")
	}
}

func TestIsUniqueViolationOn_WrappedNonPGError(t *testing.T) {
	t.Parallel()

	wrapped := errors.Join(errors.New("synthetic"), errors.New("layer"))
	if IsUniqueViolationOn(wrapped, "anything") {
		t.Fatal("IsUniqueViolationOn(joined plain errors) = true, want false")
	}
}

// A pgdriver.Error zero value carries no SQLSTATE, so it must fail the
// "23505" check even though errors.As finds it in the chain.
func TestIsUniqueViolationOn_PgErrorAsTypeAssertHandled(t *testing.T) {
	t.Parallel()

	wrapped := errors.Join(errors.New("outer"), pgdriver.Error{})
	if IsUniqueViolationOn(wrapped, "uq_enrollment_request_children_rollover_source") {
		t.Fatal("IsUniqueViolationOn(zero pgdriver.Error) = true, want false")
	}
}
