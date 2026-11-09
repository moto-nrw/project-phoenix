package devices

import (
	"context"
	"errors"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestRuntimeConstraintViolationUsesPostgresSQLState(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	classify := testRuntime().ConstraintViolation
	for _, tc := range []struct {
		name, statement, state string
		want                   bool
	}{
		{"foreign key", "INSERT INTO locale_constraint_probe VALUES (1, 2, 'value')", "23503", true},
		{"not null", "INSERT INTO locale_constraint_probe VALUES (1, NULL, NULL)", "23502", true},
		{"unique is not this refusal", "INSERT INTO locale_constraint_probe VALUES (1, NULL, 'a'), (1, NULL, 'b')", "23505", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := db.RunInTx(t.Context(), nil, func(ctx context.Context, tx bun.Tx) error {
				_, setupErr := tx.ExecContext(ctx, `CREATE TEMP TABLE locale_constraint_probe (
     id integer PRIMARY KEY,
     parent_id integer REFERENCES locale_constraint_probe(id),
     value text NOT NULL
    ) ON COMMIT DROP`)
				require.NoError(t, setupErr)
				_, err := tx.ExecContext(ctx, tc.statement)
				return err
			})
			require.Equal(t, tc.state, testpkg.PostgresSQLState(err))
			require.Equal(t, tc.want, classify(err))
			require.Equal(t, tc.want, classify(fmt.Errorf("write: %w", err)))
		})
	}
	require.False(t, classify(errors.New("violates foreign key constraint")), "text alone is not a PostgreSQL constraint error")
	require.False(t, classify(nil))
}
