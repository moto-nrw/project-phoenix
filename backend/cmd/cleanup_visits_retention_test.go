package cmd

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"strings"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// schoolSection returns the CLI output printed for one school.
func schoolSection(output string, tenantID int64) string {
	_, section, found := strings.Cut(output, fmt.Sprintf("\nSchool %d\n", tenantID))
	if !found {
		return ""
	}
	section, _, _ = strings.Cut(section, "\nSchool ")
	return section
}

// TestCleanupVisitsCLI_BackfillsChildWithoutConsent drives the one-off
// backfill of #3835 through the CLI: the dry run counts the expired visits of
// a child without consent under the tenant's retention setting, and the real
// run deletes exactly those with an audit row.
func TestCleanupVisitsCLI_BackfillsChildWithoutConsent(t *testing.T) {
	t.Parallel()
	// The CLI sweeps every school, so parallel cleanup tests must not share
	// this test's visits. Bind the private clone before building the service.
	db := testpkg.SetupIsolatedTestDB(t)
	cleanupDB, schools, retention := setupTestRetentionCleanup(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	_, err := db.NewRaw(`
		INSERT INTO config.setting_values (tenant_id, setting_key, value, updated_by)
		VALUES (?, 'gdpr.privacy_consent_retention_days', '7', NULL), (?, 'gdpr.attendance_log_enabled', 'true', NULL)
		ON CONFLICT (tenant_id, setting_key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()
	`, tenantID, tenantID).Exec(ctx)
	require.NoError(t, err)

	student := testpkg.CreateTestStudent(t, db, "Altbestand", "Ohne", "2a")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, tenantID)
	insertVisit := func(daysAgo int) int64 {
		createdAt := time.Now().AddDate(0, 0, -daysAgo)
		var id int64
		err := db.NewRaw(`
			INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at, tenant_id)
			VALUES (?, ?, ?, ?, ?, NOW(), ?) RETURNING id
		`, student.ID, group.ID, createdAt, createdAt.Add(time.Hour), createdAt, tenantID).Scan(ctx, &id)
		require.NoError(t, err)
		return id
	}
	expired := []int64{insertVisit(400), insertVisit(10)}
	recent := insertVisit(3)

	var preview bytes.Buffer
	require.NoError(t, runVisitsDryRun(log.New(io.Discard, "", 0), cleanupDB, schools, retention, &preview, false))
	section := schoolSection(preview.String(), tenantID)
	assert.Contains(t, section, "Total visits to delete: 2", "dry run output: %s", preview.String())
	assert.Contains(t, section, fmt.Sprintf(fmtStudentsAffected, 1))

	var summary bytes.Buffer
	require.NoError(t, runVisitsCleanup(log.New(io.Discard, "", 0), cleanupDB, schools, retention, &summary, false))
	assert.Contains(t, schoolSection(summary.String(), tenantID), "Records deleted: 2")

	for _, id := range expired {
		exists, err := db.NewSelect().Table("active.visits").Where("id = ?", id).Exists(ctx)
		require.NoError(t, err)
		assert.False(t, exists, "expired visit %d must be deleted", id)
	}
	exists, err := db.NewSelect().Table("active.visits").Where("id = ?", recent).Exists(ctx)
	require.NoError(t, err)
	assert.True(t, exists, "a visit inside the tenant window must stay")

	var deleted []int
	err = db.NewSelect().Table("audit.data_deletions").Column("records_deleted").
		Where("student_id = ?", student.ID).Where("deletion_type = 'visit_retention'").Scan(ctx, &deleted)
	require.NoError(t, err)
	assert.Equal(t, []int{2}, deleted)
}
