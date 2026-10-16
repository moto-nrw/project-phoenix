package users

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

func TestRowMissing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "not-found sentinel", err: ErrRepositoryNotFound, want: true},
		{name: "not-found in a database error", err: &DatabaseError{Op: "find", Err: errors.Join(ErrRepositoryNotFound, errors.New("driver"))}, want: true},
		{name: "guardian profile sentinel", err: fmt.Errorf("find: %w", usersModels.ErrGuardianProfileNotFound), want: true},
		{name: "student guardian sentinel", err: fmt.Errorf("find: %w", usersModels.ErrStudentGuardianNotFound), want: true},
		{name: "missing student", err: MissingStudentError("find student"), want: true},
		{name: "store failure", err: &DatabaseError{Op: "find", Err: errors.New("connection reset")}, want: false},
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

	err := MissingStudentError("find student")

	for _, target := range []error{ErrRepositoryNotFound, sql.ErrNoRows, usersModels.ErrStudentRowMissing} {
		if !errors.Is(err, target) {
			t.Fatalf("MissingStudentError does not wrap %v", target)
		}
	}
	var dbErr *DatabaseError
	if !errors.As(err, &dbErr) || dbErr.Op != "find student" {
		t.Fatalf("MissingStudentError = %#v, want a DatabaseError for op %q", err, "find student")
	}
}
