package users

import (
	"errors"
	"testing"

	ports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

type fakePostgresError struct{ fields map[byte]string }

func (e *fakePostgresError) Error() string           { return "postgres error" }
func (e *fakePostgresError) Field(field byte) string { return e.fields[field] }

func TestGuardianWriteErrorMarksOnlyTheEmailIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		err   error
		taken bool
	}{
		{name: "e-mail index", err: &fakePostgresError{fields: map[byte]string{'C': "23505", 'n': guardianEmailIndex}}, taken: true},
		{name: "other unique index", err: &fakePostgresError{fields: map[byte]string{'C': "23505", 'n': "idx_guardian_profiles_tenant_account"}}},
		{name: "other error class", err: &fakePostgresError{fields: map[byte]string{'C': "40001", 'n': guardianEmailIndex}}},
		{name: "plain error", err: errors.New("connection reset")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := guardianWriteError("failed to create guardian profile", tt.err)
			if got := errors.Is(err, ports.ErrGuardianEmailTaken); got != tt.taken {
				t.Fatalf("errors.Is(err, ErrGuardianEmailTaken) = %v, want %v", got, tt.taken)
			}
			if !errors.Is(err, tt.err) {
				t.Fatalf("guardianWriteError dropped the driver error: %v", err)
			}
		})
	}
}
