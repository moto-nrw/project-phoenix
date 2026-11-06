package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const germanNameCollationVersion = "1.15.755"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     germanNameCollationVersion,
		Description: "Namen in deutscher Wörterbuchordnung sortieren: ICU-Collation auf Namensspalten (#4949)",
		DependsOn: []string{
			traegerDocumentTemplateInvoiceLayoutVersion, // the last migration before this one
		},
	})
	Migrations.MustRegister(germanNameCollationUp, germanNameCollationDown)
}

// germanNameCollationUpSQL moves the name columns that lists sort by to a
// deterministic ICU collation. The database's default collation is the libc
// locale en_US.utf8; on postgres:17-alpine (musl) that sorts by code point,
// so "Özdemir", "Ábel" and "von Bergen" land behind "Zimmermann" (#4949).
//
// public.german_name is German dictionary order (umlauts and accents with
// their base letter, case only as a tie-break) with numeric runs compared as
// numbers, so "Raum 2" sorts before "Raum 10". Deterministic keeps equality,
// unique indexes and LIKE/ILIKE byte-exact, as before. Each column keeps its
// type, so Postgres rebuilds the indexes and CHECK constraints on it without
// rewriting the table.
//
// active.staff_absence_types.name stays on the default: the view
// audit.time_tracking_audit_log reads it, and a view blocks ALTER COLUMN TYPE.
const germanNameCollationUpSQL = `
CREATE COLLATION IF NOT EXISTS public.german_name (provider = icu, locale = 'de-u-kn', deterministic = true);
ALTER TABLE users.persons ALTER COLUMN first_name TYPE text COLLATE public.german_name;
ALTER TABLE users.persons ALTER COLUMN last_name TYPE text COLLATE public.german_name;
ALTER TABLE users.guardian_profiles ALTER COLUMN first_name TYPE text COLLATE public.german_name;
ALTER TABLE users.guardian_profiles ALTER COLUMN last_name TYPE text COLLATE public.german_name;
ALTER TABLE users.class_list_entries ALTER COLUMN first_name TYPE text COLLATE public.german_name;
ALTER TABLE users.class_list_entries ALTER COLUMN last_name TYPE text COLLATE public.german_name;
ALTER TABLE education.grade_transition_class_list_entries ALTER COLUMN first_name TYPE text COLLATE public.german_name;
ALTER TABLE education.grade_transition_class_list_entries ALTER COLUMN last_name TYPE text COLLATE public.german_name;
ALTER TABLE enrollment.request_children ALTER COLUMN first_name TYPE text COLLATE public.german_name;
ALTER TABLE enrollment.request_children ALTER COLUMN last_name TYPE text COLLATE public.german_name;
ALTER TABLE enrollment.request_guardians ALTER COLUMN first_name TYPE text COLLATE public.german_name;
ALTER TABLE enrollment.request_guardians ALTER COLUMN last_name TYPE text COLLATE public.german_name;
ALTER TABLE enrollment.holiday_registration_children ALTER COLUMN first_name TYPE text COLLATE public.german_name;
ALTER TABLE enrollment.holiday_registration_children ALTER COLUMN last_name TYPE text COLLATE public.german_name;
ALTER TABLE auth.invitation_tokens ALTER COLUMN first_name TYPE text COLLATE public.german_name;
ALTER TABLE auth.invitation_tokens ALTER COLUMN last_name TYPE text COLLATE public.german_name;
ALTER TABLE education.groups ALTER COLUMN name TYPE text COLLATE public.german_name;
ALTER TABLE activities.groups ALTER COLUMN name TYPE text COLLATE public.german_name;
ALTER TABLE activities.categories ALTER COLUMN name TYPE text COLLATE public.german_name;
ALTER TABLE facilities.rooms ALTER COLUMN name TYPE text COLLATE public.german_name;
ALTER TABLE schedule.dateframes ALTER COLUMN name TYPE text COLLATE public.german_name;
ALTER TABLE schedule.shift_types ALTER COLUMN name TYPE text COLLATE public.german_name;
ALTER TABLE schedule.planning_tracks ALTER COLUMN name TYPE text COLLATE public.german_name;
ALTER TABLE documents.folders ALTER COLUMN name TYPE text COLLATE public.german_name;
ALTER TABLE active.payroll_wage_types ALTER COLUMN name TYPE text COLLATE public.german_name;
ALTER TABLE config.work_time_models ALTER COLUMN name TYPE character varying(100) COLLATE public.german_name;
ALTER TABLE auth.caterers ALTER COLUMN name TYPE text COLLATE public.german_name;
ALTER TABLE platform.schools ALTER COLUMN name TYPE character varying(200) COLLATE public.german_name;
ALTER TABLE platform.organizations ALTER COLUMN name TYPE character varying(200) COLLATE public.german_name;
`

const germanNameCollationDownSQL = `
ALTER TABLE users.persons ALTER COLUMN first_name TYPE text COLLATE pg_catalog."default";
ALTER TABLE users.persons ALTER COLUMN last_name TYPE text COLLATE pg_catalog."default";
ALTER TABLE users.guardian_profiles ALTER COLUMN first_name TYPE text COLLATE pg_catalog."default";
ALTER TABLE users.guardian_profiles ALTER COLUMN last_name TYPE text COLLATE pg_catalog."default";
ALTER TABLE users.class_list_entries ALTER COLUMN first_name TYPE text COLLATE pg_catalog."default";
ALTER TABLE users.class_list_entries ALTER COLUMN last_name TYPE text COLLATE pg_catalog."default";
ALTER TABLE education.grade_transition_class_list_entries ALTER COLUMN first_name TYPE text COLLATE pg_catalog."default";
ALTER TABLE education.grade_transition_class_list_entries ALTER COLUMN last_name TYPE text COLLATE pg_catalog."default";
ALTER TABLE enrollment.request_children ALTER COLUMN first_name TYPE text COLLATE pg_catalog."default";
ALTER TABLE enrollment.request_children ALTER COLUMN last_name TYPE text COLLATE pg_catalog."default";
ALTER TABLE enrollment.request_guardians ALTER COLUMN first_name TYPE text COLLATE pg_catalog."default";
ALTER TABLE enrollment.request_guardians ALTER COLUMN last_name TYPE text COLLATE pg_catalog."default";
ALTER TABLE enrollment.holiday_registration_children ALTER COLUMN first_name TYPE text COLLATE pg_catalog."default";
ALTER TABLE enrollment.holiday_registration_children ALTER COLUMN last_name TYPE text COLLATE pg_catalog."default";
ALTER TABLE auth.invitation_tokens ALTER COLUMN first_name TYPE text COLLATE pg_catalog."default";
ALTER TABLE auth.invitation_tokens ALTER COLUMN last_name TYPE text COLLATE pg_catalog."default";
ALTER TABLE education.groups ALTER COLUMN name TYPE text COLLATE pg_catalog."default";
ALTER TABLE activities.groups ALTER COLUMN name TYPE text COLLATE pg_catalog."default";
ALTER TABLE activities.categories ALTER COLUMN name TYPE text COLLATE pg_catalog."default";
ALTER TABLE facilities.rooms ALTER COLUMN name TYPE text COLLATE pg_catalog."default";
ALTER TABLE schedule.dateframes ALTER COLUMN name TYPE text COLLATE pg_catalog."default";
ALTER TABLE schedule.shift_types ALTER COLUMN name TYPE text COLLATE pg_catalog."default";
ALTER TABLE schedule.planning_tracks ALTER COLUMN name TYPE text COLLATE pg_catalog."default";
ALTER TABLE documents.folders ALTER COLUMN name TYPE text COLLATE pg_catalog."default";
ALTER TABLE active.payroll_wage_types ALTER COLUMN name TYPE text COLLATE pg_catalog."default";
ALTER TABLE config.work_time_models ALTER COLUMN name TYPE character varying(100) COLLATE pg_catalog."default";
ALTER TABLE auth.caterers ALTER COLUMN name TYPE text COLLATE pg_catalog."default";
ALTER TABLE platform.schools ALTER COLUMN name TYPE character varying(200) COLLATE pg_catalog."default";
ALTER TABLE platform.organizations ALTER COLUMN name TYPE character varying(200) COLLATE pg_catalog."default";
DROP COLLATION IF EXISTS public.german_name;
`

// One multi-statement query runs as one implicit transaction, so a failed
// ALTER leaves every column as it was.
func germanNameCollationUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, germanNameCollationUpSQL); err != nil {
		return fmt.Errorf("move name columns to the German name collation: %w", err)
	}
	return nil
}

func germanNameCollationDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, germanNameCollationDownSQL); err != nil {
		return fmt.Errorf("move name columns back to the default collation: %w", err)
	}
	return nil
}
