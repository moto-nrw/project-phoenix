package common

import (
	"errors"
	"fmt"
	"testing"
)

// repositoryNotFound rebuilds what database/repositories/base.TranslateNotFound
// returns: the persistence-neutral sentinel joined onto the SQL driver's
// missing-row error. This package may import neither models/base nor
// database/sql, so the shape is spelled out here.
func repositoryNotFound() error {
	return errors.Join(
		errors.New("repository: not found"),
		errors.New("sql: no rows in result set"),
	)
}

func TestIsNotFound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "repository result", err: repositoryNotFound(), want: true},
		{name: "wrapped repository result", err: fmt.Errorf("find by id: %w", repositoryNotFound()), want: true},
		{name: "twice wrapped", err: fmt.Errorf("service: %w", fmt.Errorf("repo: %w", repositoryNotFound())), want: true},
		{name: "joined with an unrelated error", err: errors.Join(errors.New("audit write failed"), repositoryNotFound()), want: true},
		{name: "unrelated error", err: errors.New("database unavailable"), want: false},
		{name: "unrelated wrapped error", err: fmt.Errorf("find by id: %w", errors.New("connection refused")), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.want {
				t.Fatalf("IsNotFound(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
