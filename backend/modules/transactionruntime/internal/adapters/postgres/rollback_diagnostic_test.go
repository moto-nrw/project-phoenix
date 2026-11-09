package postgres

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun/driver/pgdriver"
)

// An attached ROLLBACK failure is diagnostics only (#6316): neither its
// chain nor its text classifies the operation, also when callers wrap the
// unit's error further. The operation's own error still classifies.
func TestRollbackDiagnosticClassifiesTheOperationOnly(t *testing.T) {
	t.Parallel()
	plain := errors.New("operation failed")
	rollbacks := map[string]error{
		"bad_conn":    driver.ErrBadConn,
		"text_conn":   errors.New("driver: bad connection"),
		"deadlock":    pgdriver.NewError(map[byte]string{'C': "40P01", 'M': "deadlock"}),
		"fk_text":     errors.New(`ERROR: insert violates foreign key constraint "fk_x" (SQLSTATE=23503)`),
		"unique_text": errors.New(`ERROR: duplicate key value violates unique constraint "uq_x" (SQLSTATE=23505)`),
	}
	for name, rollback := range rollbacks {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := fmt.Errorf("decision: %w", &rollbackDiagnostic{err: plain, rollback: rollback})
			require.Contains(t, err.Error(), rollback.Error(), "the rollback failure stays in the text")
			require.False(t, IsRetryableTransactionError(err))
			require.False(t, IsTransientDatabaseError(err))
			require.False(t, IsConstraintViolation(err))
			require.False(t, IsUniqueViolation(err))
			require.False(t, IsUniqueViolationOn(err, "uq_x"))
		})
	}
	t.Run("operation_classifies", func(t *testing.T) {
		t.Parallel()
		rollback := errors.New("rollback failed")
		require.True(t, IsTransientDatabaseError(&rollbackDiagnostic{err: driver.ErrBadConn, rollback: rollback}))
		require.True(t, IsTransientDatabaseError(&rollbackDiagnostic{err: errors.New("x: driver: bad connection"), rollback: rollback}))
		require.True(t, IsRetryableTransactionError(&rollbackDiagnostic{err: pgdriver.NewError(map[byte]string{'C': "40001"}), rollback: rollback}))
		require.True(t, IsConstraintViolation(&rollbackDiagnostic{err: pgdriver.NewError(map[byte]string{'C': "23503"}), rollback: rollback}))
	})
}
