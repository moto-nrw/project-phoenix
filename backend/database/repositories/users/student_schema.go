package users

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

// requiredStudentColumns are verified once at startup, never on request paths.
var requiredStudentColumns = []string{
	"student_profiles.id", "student_profiles.person_id", "student_profiles.tenant_id",
	"student_school_memberships.id", "student_school_memberships.student_profile_id",
	"student_school_memberships.tenant_id", "student_school_memberships.deleted_at",
	"student_school_memberships.school_class", "student_school_memberships.status",
	"student_school_memberships.enrolled_from", "student_school_memberships.enrolled_until",
	"student_care_profiles.membership_id", "student_care_profiles.tenant_id",
	"student_care_profiles.pickup_status", "student_care_profiles.bus_days",
	"student_care_profiles.pickup_days", "student_care_profiles.departure_days",
	"student_care_profiles.allowed_departure_modes", "student_care_profiles.departure_companion_note",
	"student_care_profiles.sick", "student_care_profiles.sick_since",
	"student_care_profiles.excused", "student_care_profiles.excused_since",
}

// VerifyStudentSchema fails fast when the connected database is missing a
// student owner column or the users.student_companions table (1.15.209) that
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
		SELECT c.relname || '.' || a.attname
		FROM pg_catalog.pg_attribute a
		JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'users'
		  AND c.relname IN ('student_profiles', 'student_school_memberships', 'student_care_profiles')
		  AND a.attnum > 0
		  AND NOT a.attisdropped
		  AND (c.relname || '.' || a.attname) IN (?)
	`, bun.List(requiredStudentColumns)).Scan(ctx, &present); err != nil {
		return fmt.Errorf("verify student owner schema: %w", err)
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
			"student owner schema is missing required column(s) %s: the database schema is not fully migrated — run migrations before starting the server",
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
