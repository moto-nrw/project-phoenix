package owners

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	fileStorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingPhotoUnlinker struct {
	mu       sync.Mutex
	unlinked []string
}

func (u *recordingPhotoUnlinker) UnlinkStored(storedURL string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.unlinked = append(u.unlinked, storedURL)
}

// TestStudentPhotoCareEndStepRunsOnTheServePool runs the nightly step of
// #6650 the way the Worker composes it, inside the school's tenant
// transaction on the least-privilege Serve pool: the photo data of a child
// whose care ended goes, the deletion log names the child as a system
// deletion with the reason care_ended, the durable file cleanup entry
// commits with it, and the file is removed after the commit.
func TestStudentPhotoCareEndStepRunsOnTheServePool(t *testing.T) {
	t.Parallel()
	admin := testpkg.SetupTestDB(t)
	serve := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, serve.Close()) })
	db, err := transactionruntime.NewDatabase(admin)
	require.NoError(t, err)
	audit, _, _, err := NewRetentionTestOwners(db, testpkg.TenantRuntime(t, admin))
	require.NoError(t, err)
	unlinker := &recordingPhotoUnlinker{}
	runtime := StudentPhotoRuntime{Cleanup: fileStorageCompose.NewUploadCleanup(admin, "student-photos").Queue, Unlinker: unlinker}
	photos, err := peopleCompose.NewStudentPhotoCareEnd(admin, StudentDirectoryReader(), runtime, func(peopleCompose.Observation) {}, slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	step := studentPhotoCareEndStep(photos, audit)
	assert.True(t, step.LogsSubjects, "the step logs each child itself")

	tenantID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudent(t, admin, "Ende", "Foto", "4a")
	url := "/uploads/student-photos/ende-foto.jpg"
	ctx := context.Background()
	_, err = admin.NewRaw(`UPDATE users.student_profiles SET photo_path = ?, photo_consent_given_at = ? WHERE id = ?`,
		url, calendar.NewDate(2025, time.September, 1).BerlinMidnight(), student.ID).Exec(ctx)
	require.NoError(t, err)
	// A last care day well before any day this test runs on.
	_, err = admin.NewRaw(`UPDATE users.student_school_memberships SET enrolled_from = ?, enrolled_until = ?, status = 'inactive'
		WHERE student_profile_id = ?`, calendar.NewDate(2024, time.August, 1), calendar.NewDate(2025, time.July, 31), student.ID).Exec(ctx)
	require.NoError(t, err)

	var outcomeDeleted int
	require.NoError(t, testpkg.WithinTenantContext(t, ctx, serve, tenantID, func(txCtx context.Context) error {
		outcome, runErr := step.Run(txCtx)
		outcomeDeleted = outcome.Deleted
		assert.Empty(t, unlinker.unlinked, "no file goes before the commit")
		return runErr
	}))
	assert.Equal(t, 1, outcomeDeleted)
	assert.Equal(t, []string{url}, unlinker.unlinked, "the file goes after the commit")

	var row struct {
		PhotoPath           *string    `bun:"photo_path"`
		PhotoConsentGivenAt *time.Time `bun:"photo_consent_given_at"`
	}
	require.NoError(t, admin.NewRaw(`SELECT photo_path, photo_consent_given_at FROM users.student_profiles WHERE id = ?`, student.ID).Scan(ctx, &row))
	assert.Nil(t, row.PhotoPath)
	assert.Nil(t, row.PhotoConsentGivenAt)

	var entry struct {
		RecordsDeleted int    `bun:"records_deleted"`
		Reason         string `bun:"deletion_reason"`
		DeletedBy      string `bun:"deleted_by"`
		LastCareDay    string `bun:"last_care_day"`
		ConsentEnded   bool   `bun:"consent_ended"`
	}
	require.NoError(t, admin.NewRaw(`SELECT records_deleted, deletion_reason, deleted_by,
			metadata->>'last_care_day' AS last_care_day, (metadata->>'consent_ended')::boolean AS consent_ended
		FROM audit.data_deletions WHERE tenant_id = ? AND student_id = ? AND deletion_type = ?`,
		tenantID, student.ID, auditModels.DeletionTypeStudentPhotoRetention).Scan(ctx, &entry))
	assert.Equal(t, 1, entry.RecordsDeleted)
	assert.Equal(t, "care_ended", entry.Reason)
	assert.Equal(t, "system", entry.DeletedBy)
	assert.Equal(t, "2025-07-31", entry.LastCareDay)
	assert.True(t, entry.ConsentEnded)

	queued, err := admin.NewSelect().TableExpr("documents.file_cleanup").
		Where("tenant_id = ? AND object_kind = ?", tenantID, "student-photos").Count(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, queued, "the durable cleanup entry backs the after-commit unlink")

	var trail int
	require.NoError(t, admin.NewRaw(`SELECT count(*) FROM audit.student_consent_changes WHERE student_id = ?`, student.ID).Scan(ctx, &trail))
	assert.Zero(t, trail, "the end of care records no withdrawal")
}
