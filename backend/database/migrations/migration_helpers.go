package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// Migration represents a database migration with metadata
type Migration struct {
	Version     string   // Semantic version of the migration
	Description string   // Human-readable description
	DependsOn   []string // Versions this migration depends on
	Up          func(ctx context.Context, db *bun.DB) error
	Down        func(ctx context.Context, db *bun.DB) error
}

// Common validation patterns used across migrations
const (
	// EmailValidationRegex is the standard email validation pattern used in CHECK constraints
	// Format: local-part@domain.tld
	// Example: user.name+tag@example.com
	EmailValidationRegex = `^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$`
)

// TableExists checks if a table exists in the database
func TableExists(ctx context.Context, tx bun.Tx, table string) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = ?
		)
	`, table).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("error checking if table %s exists: %w", table, err)
	}

	return exists, nil
}

// ColumnExists checks if a column exists in a table
func ColumnExists(ctx context.Context, tx bun.Tx, table, column string) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.columns 
			WHERE table_name = ? AND column_name = ?
		)
	`, table, column).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("error checking if column %s in table %s exists: %w", column, table, err)
	}

	return exists, nil
}

// ConstraintExists checks if a constraint exists
func ConstraintExists(ctx context.Context, tx bun.Tx, name string) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.table_constraints
			WHERE constraint_name = ?
		)
	`, name).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("error checking if constraint %s exists: %w", name, err)
	}

	return exists, nil
}

// IndexExists checks if an index exists
func IndexExists(ctx context.Context, tx bun.Tx, name string) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT FROM pg_indexes
			WHERE indexname = ?
		)
	`, name).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("error checking if index %s exists: %w", name, err)
	}

	return exists, nil
}

// SafeAddColumn adds a column only if it doesn't exist
func SafeAddColumn(ctx context.Context, tx bun.Tx, table, column, dataType string) error {
	exists, err := ColumnExists(ctx, tx, table, column)
	if err != nil {
		return err
	}

	if !exists {
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			ALTER TABLE %s ADD COLUMN %s %s
		`, table, column, dataType))

		if err != nil {
			return fmt.Errorf("error adding column %s to table %s: %w", column, table, err)
		}
	}

	return nil
}

// SafeAddConstraint adds a constraint only if it doesn't exist
func SafeAddConstraint(ctx context.Context, tx bun.Tx, table, constraintName, constraintDef string) error {
	exists, err := ConstraintExists(ctx, tx, constraintName)
	if err != nil {
		return err
	}

	if !exists {
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			ALTER TABLE %s ADD CONSTRAINT %s %s
		`, table, constraintName, constraintDef))

		if err != nil {
			return fmt.Errorf("error adding constraint %s to table %s: %w", constraintName, table, err)
		}
	}

	return nil
}

// SafeCreateIndex creates an index only if it doesn't exist
func SafeCreateIndex(ctx context.Context, tx bun.Tx, indexName, table, columns string, unique bool) error {
	exists, err := IndexExists(ctx, tx, indexName)
	if err != nil {
		return err
	}

	if !exists {
		uniqueStr := ""
		if unique {
			uniqueStr = "UNIQUE"
		}

		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			CREATE %s INDEX %s ON %s(%s)
		`, uniqueStr, indexName, table, columns))

		if err != nil {
			return fmt.Errorf("error creating index %s on table %s: %w", indexName, table, err)
		}
	}

	return nil
}

// SafeDropConstraint drops a constraint only if it exists
func SafeDropConstraint(ctx context.Context, tx bun.Tx, table, constraintName string) error {
	exists, err := ConstraintExists(ctx, tx, constraintName)
	if err != nil {
		return err
	}

	if exists {
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			ALTER TABLE %s DROP CONSTRAINT %s
		`, table, constraintName))

		if err != nil {
			return fmt.Errorf("error dropping constraint %s from table %s: %w", constraintName, table, err)
		}
	}

	return nil
}

// SafeDropIndex drops an index only if it exists
func SafeDropIndex(ctx context.Context, tx bun.Tx, indexName string) error {
	exists, err := IndexExists(ctx, tx, indexName)
	if err != nil {
		return err
	}

	if exists {
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			DROP INDEX %s
		`, indexName))

		if err != nil {
			return fmt.Errorf("error dropping index %s: %w", indexName, err)
		}
	}

	return nil
}
