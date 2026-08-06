package users

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

// requiredStudentColumns are the users.students columns the student
// repository selects and writes unconditionally. They all landed in their own
// migrations (bus_days 1.15.112, pickup_days 1.15.116, departure_days
// 1.15.120, allowed_departure_modes 1.15.130, departure_companion_note
// 1.15.138; pickup_status predates them), and the request hot paths no longer
// probe for them per request (#2059) — instead their presence is verified
// ONCE at server startup via VerifyStudentSchema.
var requiredStudentColumns = []string{
	"pickup_status",
	"bus_days",
	"pickup_days",
	"departure_days",
	"allowed_departure_modes",
	"departure_companion_note",
}

// VerifyStudentSchema fails fast when the connected database is missing a
// users.students column or the users.student_companions table (1.15.209) that
// the student repository relies on unconditionally. api.New calls it at boot,
// so the server only starts against a fully migrated schema and a partially
// migrated database surfaces as a clear startup error instead of per-request
// failures (#2059).
//
// It reads pg_catalog rather than information_schema: the serve connection
// runs as the least-privilege phoenix_auth role, and information_schema
// filters rows by column privileges, which would make an authorization gap
// indistinguishable from a missing column. pg_catalog is visible to every
// role and reflects the schema itself.
func VerifyStudentSchema(ctx context.Context, db *bun.DB) error {
	var present []string
	if err := db.NewRaw(`
		SELECT a.attname
		FROM pg_catalog.pg_attribute a
		JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'users'
		  AND c.relname = 'students'
		  AND a.attnum > 0
		  AND NOT a.attisdropped
		  AND a.attname IN (?)
	`, bun.List(requiredStudentColumns)).Scan(ctx, &present); err != nil {
		return fmt.Errorf("verify users.students schema: %w", err)
	}

	presentSet := make(map[string]bool, len(present))
	for _, col := range present {
		presentSet[col] = true
	}
	var missing []string
	for _, col := range requiredStudentColumns {
		if !presentSet[col] {
			missing = append(missing, col)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf(
			"users.students is missing required column(s) %s: the database schema is not fully migrated — run migrations before starting the server",
			strings.Join(missing, ", "),
		)
	}

	var companionsExists bool
	if err := db.NewRaw(`
		SELECT EXISTS (
			SELECT 1
			FROM pg_catalog.pg_class c
			JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname = 'users'
			  AND c.relname = 'student_companions'
			  AND c.relkind = 'r'
		)
	`).Scan(ctx, &companionsExists); err != nil {
		return fmt.Errorf("verify users.student_companions table: %w", err)
	}
	if !companionsExists {
		return fmt.Errorf("users.student_companions does not exist: the database schema is not fully migrated — run migrations before starting the server")
	}

	return nil
}
