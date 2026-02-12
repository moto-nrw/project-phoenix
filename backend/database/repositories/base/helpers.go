package base

import (
	"database/sql"
	"fmt"
)

// AssertRowsAffected checks that exactly the expected number of rows were affected
// by an UPDATE or DELETE operation. With RLS enabled, a mismatched count typically
// means the tenant could not see (and therefore could not modify) the target row.
func AssertRowsAffected(result sql.Result, expected int64) error {
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n != expected {
		return fmt.Errorf("expected %d rows affected, got %d", expected, n)
	}
	return nil
}
