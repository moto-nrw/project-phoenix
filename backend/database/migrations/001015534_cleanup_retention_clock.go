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
		Description: "Feste Aufbewahrungsdauer erledigter Dateilöschungen auch bei Zeitumstellungen einhalten",
		DependsOn: []string{
			cleanupJobRetentionVersion,
			schoolContentRetentionVersion,
		},
	})
	Migrations.MustRegister(cleanupRetentionClockUp, cleanupRetentionClockDown)
}

// These functions compare instants against the owners' fixed 30 * 24 hour
// windows. Evaluate their INTERVAL '30 days' in UTC so it remains 720 hours
// across clock changes, regardless of the calling session's timezone.
// ALTER FUNCTION preserves their tenant authorization, search_path and grants.
const cleanupRetentionClockUpSQL = `
	ALTER FUNCTION documents.delete_finished_file_cleanups(BIGINT, TIMESTAMPTZ) SET TimeZone = 'UTC';
	ALTER FUNCTION documents.delete_finished_announcement_attachment_cleanups(BIGINT, TIMESTAMPTZ) SET TimeZone = 'UTC';
	ALTER FUNCTION users.delete_finished_staff_document_file_cleanups(BIGINT, TIMESTAMPTZ) SET TimeZone = 'UTC';
	ALTER FUNCTION users.delete_finished_student_document_file_cleanups(BIGINT, TIMESTAMPTZ) SET TimeZone = 'UTC';
	ALTER FUNCTION documents.delete_removed_files(BIGINT, TIMESTAMPTZ) SET TimeZone = 'UTC';
`

const cleanupRetentionClockDownSQL = `
	ALTER FUNCTION documents.delete_finished_file_cleanups(BIGINT, TIMESTAMPTZ) RESET TimeZone;
	ALTER FUNCTION documents.delete_finished_announcement_attachment_cleanups(BIGINT, TIMESTAMPTZ) RESET TimeZone;
	ALTER FUNCTION users.delete_finished_staff_document_file_cleanups(BIGINT, TIMESTAMPTZ) RESET TimeZone;
	ALTER FUNCTION users.delete_finished_student_document_file_cleanups(BIGINT, TIMESTAMPTZ) RESET TimeZone;
	ALTER FUNCTION documents.delete_removed_files(BIGINT, TIMESTAMPTZ) RESET TimeZone;
`

func cleanupRetentionClockUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, cleanupRetentionClockUpSQL); err != nil {
		return fmt.Errorf("set the fixed cleanup retention clock: %w", err)
	}
	return nil
}

func cleanupRetentionClockDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, cleanupRetentionClockDownSQL); err != nil {
		return fmt.Errorf("restore the session cleanup retention clock: %w", err)
	}
	return nil
}
