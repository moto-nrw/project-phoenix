package test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun/driver/pgdriver"
)

func TestPostgresSQLStatePreservesErrorIdentity(t *testing.T) {
	t.Parallel()
	denied := pgdriver.NewError(map[byte]string{'C': "42501", 'M': "keine Berechtigung"})
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"German denial", denied, "42501"},
		{"wrapped denial", fmt.Errorf("operation: %w", denied), "42501"},
		{"different database error", pgdriver.NewError(map[byte]string{'C': "23505", 'M': "permission denied"}), "23505"},
		{"message alone is insufficient", errors.New("permission denied"), ""},
		{"nil", nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, PostgresSQLState(tc.err))
		})
	}
}
