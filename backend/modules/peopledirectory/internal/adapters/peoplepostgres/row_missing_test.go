package users

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	owner0 "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	ports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

func TestRowMissing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "not-found sentinel", err: ports.ErrRepositoryNotFound, want: true},
		{name: "not-found in a database error", err: &ports.DatabaseError{Op: "find", Err: errors.Join(ports.ErrRepositoryNotFound, errors.New("driver"))}, want: true},
		{name: "guardian profile sentinel", err: fmt.Errorf("find: %w", usersModels.ErrGuardianProfileNotFound), want: true},
		{name: "student guardian sentinel", err: fmt.Errorf("find: %w", usersModels.ErrStudentGuardianNotFound), want: true},
		{name: "missing student", err: ports.MissingStudentError("find student"), want: true},
		{name: "store failure", err: &ports.DatabaseError{Op: "find", Err: errors.New("connection reset")}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ports.RowMissing(tt.err); got != tt.want {
				t.Fatalf("ports.RowMissing(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestMissingStudentErrorKeepsTheCallerSentinels(t *testing.T) {
	t.Parallel()

	err := ports.MissingStudentError("find student")

	for _, target := range []error{ports.ErrRepositoryNotFound, sql.ErrNoRows, owner0.ErrStudentRowMissing} {
		if !errors.Is(err, target) {
			t.Fatalf("MissingStudentError does not wrap %v", target)
		}
	}
	var dbErr *ports.DatabaseError
	if !errors.As(err, &dbErr) || dbErr.Op != "find student" {
		t.Fatalf("MissingStudentError = %#v, want a DatabaseError for op %q", err, "find student")
	}
}
