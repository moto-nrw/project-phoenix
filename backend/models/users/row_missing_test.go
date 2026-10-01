package users

import (
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestRowMissing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "not-found sentinel", err: base.ErrNotFound, want: true},
		{name: "not-found in a database error", err: &base.DatabaseError{Op: "find", Err: errors.Join(base.ErrNotFound, errors.New("driver"))}, want: true},
		{name: "guardian profile sentinel", err: fmt.Errorf("find: %w", ErrGuardianProfileNotFound), want: true},
		{name: "student guardian sentinel", err: fmt.Errorf("find: %w", ErrStudentGuardianNotFound), want: true},
		{name: "missing student", err: MissingStudentError("find student", errors.New("driver no rows")), want: true},
		{name: "store failure", err: &base.DatabaseError{Op: "find", Err: errors.New("connection reset")}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := RowMissing(tt.err); got != tt.want {
				t.Fatalf("RowMissing(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestMissingStudentErrorKeepsTheCallerSentinels(t *testing.T) {
	t.Parallel()

	noRows := errors.New("driver no rows")
	err := MissingStudentError("find student", noRows)

	for _, target := range []error{base.ErrNotFound, noRows, ErrStudentRowMissing} {
		if !errors.Is(err, target) {
			t.Fatalf("MissingStudentError does not wrap %v", target)
		}
	}
	var dbErr *base.DatabaseError
	if !errors.As(err, &dbErr) || dbErr.Op != "find student" {
		t.Fatalf("MissingStudentError = %#v, want a DatabaseError for op %q", err, "find student")
	}
}
