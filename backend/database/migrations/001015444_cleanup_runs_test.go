package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeletionLogAcceptsRetentionRowsWithoutASubjectOnly(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	school := testpkg.Tenant(t)

	insert := func(deletionType string, studentID, staffID any) error {
		_, err := db.ExecContext(ctx, `
			INSERT INTO audit.data_deletions (tenant_id, student_id, staff_id, deletion_type, records_deleted, deleted_by)
			VALUES (?, ?, ?, ?, 1, 'system')`, school, studentID, staffID, deletionType)
		return err
	}

	// One table instead of a test per migration: each SetupTestDB call clones
	// the database, and these rows only need the fully migrated schema.
	for _, migration := range []struct {
		version       string
		deletionTypes []string
	}{
		{"1.15.444", []string{
			"unregistered_tag_scan_retention", "status_day_retention", "parent_message_retention",
			"auth_event_retention", "data_access_log_retention", "deletion_log_retention",
		}},
		{"1.15.449", cleanupJobRetentionKinds},
		{"1.15.452", []string{settledEnrollmentRetentionKind}},
		{"1.15.453", []string{staffSickNoteRetentionKind}},
		{"1.15.460", []string{attendanceRetentionKind}},
		{"1.15.464", []string{guardianRetentionKind}},
		{"1.15.465", schoolContentRetentionKinds},
	} {
		for _, deletionType := range migration.deletionTypes {
			assert.NoError(t, insert(deletionType, nil, nil), "%s (%s) logs a data kind without a subject", deletionType, migration.version)
		}
	}

	assert.Error(t, insert("manual", nil, nil), "a manual deletion still names its subject")
	assert.Error(t, insert("visit_retention", nil, nil), "a per-child retention row still names the child")
	assert.Error(t, insert("unregistered_tag_scan_retention", 1, 1), "a row never names a child and a staff member")
	require.NoError(t, insert("visit_retention", 1, nil), "a per-child row stays valid")
}

func TestCleanupRunsAreTenantIsolated(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	var rlsForced bool
	require.NoError(t, db.NewRaw(`
		SELECT relrowsecurity AND relforcerowsecurity FROM pg_class
		WHERE oid = 'platform.cleanup_runs'::regclass`).Scan(context.Background(), &rlsForced))
	assert.True(t, rlsForced, "the last runs of one school stay invisible to another")
}
