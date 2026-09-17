package repositories

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// sqlStateError carries a PostgreSQL SQLSTATE the way the driver error
// exposes it.
type sqlStateError struct{ code string }

func (e sqlStateError) Error() string { return "sqlstate " + e.code }

func (e sqlStateError) Field(field byte) string {
	if field == 'C' {
		return e.code
	}
	return ""
}

// wrappedError wraps a cause the way the repository error shape does.
type wrappedError struct {
	op  string
	err error
}

func (e wrappedError) Error() string { return e.op + ": " + e.err.Error() }
func (e wrappedError) Unwrap() error { return e.err }

func TestIsForeignKeyViolation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "plain error", err: errors.New("foreign key violated"), want: false},
		{name: "wrapped plain error", err: wrappedError{op: "delete", err: errors.New("foreign key violated")}, want: false},
		{name: "foreign key violation", err: sqlStateError{code: "23503"}, want: true},
		{name: "unique violation", err: sqlStateError{code: "23505"}, want: false},
		{name: "wrapped foreign key violation", err: wrappedError{op: "delete", err: sqlStateError{code: "23503"}}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isForeignKeyViolation(tt.err))
		})
	}
}
