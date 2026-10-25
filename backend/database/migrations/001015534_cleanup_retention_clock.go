package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const cleanupRetentionClockVersion = "1.15.534"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     cleanupRetentionClockVersion,
		Description: "Feste Aufbewahrungsfristen auch bei Zeitumstellungen einhalten",
		DependsOn: []string{
			auditRetentionCapabilitiesVersion,
			auditLedgerRetentionVersion,
			cleanupJobRetentionVersion,
			protocolRetentionVersion,
			schoolContentRetentionVersion,
		},
	})
	Migrations.MustRegister(cleanupRetentionClockUp, cleanupRetentionClockDown)
}

// These functions compare instants against fixed windows of the owners:
// INTERVAL '30 days' and '90 days' against 30 and 90 times 24 hours, and
// INTERVAL '3 years' against the same UTC clock time three years earlier.
// In a Berlin session a day across a clock change lasts 23 or 25 hours and a
// year back can land in the other offset, so the window would depend on the
// calling session's timezone and could refuse the owner's cutoff. Evaluate
// them in UTC instead. The calendar-year windows of the access log and the
// following-year protocols name Europe/Berlin explicitly and stay as they are.
// ALTER FUNCTION preserves their tenant authorization, search_path and grants.
const cleanupRetentionClockUpSQL = `
	ALTER FUNCTION documents.delete_finished_file_cleanups(BIGINT, TIMESTAMPTZ) SET TimeZone = 'UTC';
	ALTER FUNCTION documents.delete_finished_announcement_attachment_cleanups(BIGINT, TIMESTAMPTZ) SET TimeZone = 'UTC';
	ALTER FUNCTION users.delete_finished_staff_document_file_cleanups(BIGINT, TIMESTAMPTZ) SET TimeZone = 'UTC';
	ALTER FUNCTION users.delete_finished_student_document_file_cleanups(BIGINT, TIMESTAMPTZ) SET TimeZone = 'UTC';
	ALTER FUNCTION documents.delete_removed_files(BIGINT, TIMESTAMPTZ) SET TimeZone = 'UTC';
	ALTER FUNCTION audit.delete_expired_unregistered_tag_scans(BIGINT, TIMESTAMPTZ) SET TimeZone = 'UTC';
	ALTER FUNCTION audit.delete_expired_auth_events(BIGINT, TIMESTAMPTZ) SET TimeZone = 'UTC';
	ALTER FUNCTION audit.delete_expired_data_deletions(BIGINT, TIMESTAMPTZ) SET TimeZone = 'UTC';
	ALTER FUNCTION audit.delete_expired_protocol(BIGINT, TEXT, TIMESTAMPTZ) SET TimeZone = 'UTC';
`

const cleanupRetentionClockDownSQL = `
	ALTER FUNCTION documents.delete_finished_file_cleanups(BIGINT, TIMESTAMPTZ) RESET TimeZone;
	ALTER FUNCTION documents.delete_finished_announcement_attachment_cleanups(BIGINT, TIMESTAMPTZ) RESET TimeZone;
	ALTER FUNCTION users.delete_finished_staff_document_file_cleanups(BIGINT, TIMESTAMPTZ) RESET TimeZone;
	ALTER FUNCTION users.delete_finished_student_document_file_cleanups(BIGINT, TIMESTAMPTZ) RESET TimeZone;
	ALTER FUNCTION documents.delete_removed_files(BIGINT, TIMESTAMPTZ) RESET TimeZone;
	ALTER FUNCTION audit.delete_expired_unregistered_tag_scans(BIGINT, TIMESTAMPTZ) RESET TimeZone;
	ALTER FUNCTION audit.delete_expired_auth_events(BIGINT, TIMESTAMPTZ) RESET TimeZone;
	ALTER FUNCTION audit.delete_expired_data_deletions(BIGINT, TIMESTAMPTZ) RESET TimeZone;
	ALTER FUNCTION audit.delete_expired_protocol(BIGINT, TEXT, TIMESTAMPTZ) RESET TimeZone;
`

func cleanupRetentionClockUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, cleanupRetentionClockUpSQL); err != nil {
		return fmt.Errorf("set the fixed retention clock: %w", err)
	}
	return nil
}

func cleanupRetentionClockDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, cleanupRetentionClockDownSQL); err != nil {
		return fmt.Errorf("restore the session retention clock: %w", err)
	}
	return nil
}
