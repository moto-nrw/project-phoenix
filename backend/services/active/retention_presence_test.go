package active_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/services"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type retentionAuditFault struct {
	command     services.AuditCommand
	afterAppend error
	writes      int
}

func (f *retentionAuditFault) Append(ctx context.Context, event any) error {
	if err := f.command.Append(ctx, event); err != nil {
		return err
	}
	f.writes++
	return f.afterAppend
}

type retentionDeleteFault struct {
	activeService.PresenceRetention
	afterDelete error
	deleted     int64
}

func (p *retentionDeleteFault) DeleteCompletedVisitsBefore(ctx context.Context, id int64, cutoff time.Time) (int64, error) {
	count, err := p.PresenceRetention.DeleteCompletedVisitsBefore(ctx, id, cutoff)
	if err == nil {
		p.deleted += count
		err = p.afterDelete
	}
	return count, err
}

func TestVisitRetentionRollsBackDeletionAndAuditThenRetries(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"delete", "audit"} {
		t.Run(stage, func(t *testing.T) { testRetentionRollback(t, stage) })
	}
}

func testRetentionRollback(t *testing.T, stage string) {
	t.Helper()
	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	presence := testSchoolPresence(t, db)
	command, err := services.NewCleanupAuditCommand(slog.Default())
	require.NoError(t, err)
	injected := errors.New("fail after retention write")
	fault := &retentionAuditFault{command: command}
	deleteFault := &retentionDeleteFault{PresenceRetention: presence}
	if stage == "delete" {
		deleteFault.afterDelete = injected
	} else {
		fault.afterAppend = injected
	}
	repos := repositories.NewRetentionCleanupRepositories(db, fault)
	svc := activeService.NewCleanupService(deleteFault, repos.Supervisor, repos.Consent, repos.Deletion, nil, db)
	consent := testpkg.CreateTestPrivacyConsent(t, db, "retention-rollback")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	entry := time.Now().AddDate(0, 0, -45)
	exit := entry.Add(time.Hour)
	visit, err := presence.RecordVisit(ctx, studentpresence.Visit{
		CreatedAt: entry,
		StudentID: consent.StudentID, ActiveGroupID: group.ID, EntryTime: entry, ExitTime: &exit,
	})
	require.NoError(t, err)

	open := testpkg.CreateTestVisit(t, db, consent.StudentID, group.ID, time.Now(), nil)
	recentExit := time.Now()
	recent := testpkg.CreateTestVisit(t, db, consent.StudentID, group.ID, recentExit.Add(-time.Hour), &recentExit)
	failed, err := svc.CleanupExpiredVisits(ctx)
	require.NoError(t, err, "per-student failures are reported in the cleanup result")
	assert.False(t, failed.Success)
	require.Len(t, failed.Errors, 1)
	assert.Contains(t, failed.Errors[0].Error, injected.Error())
	require.EqualValues(t, 1, deleteFault.deleted, "failure must follow the real visit deletion")
	if stage == "audit" {
		require.Equal(t, 1, fault.writes, "failure must follow the real audit append")
	}
	assert.Zero(t, failed.RecordsDeleted)
	_, err = presence.FindVisit(ctx, visit.ID)
	require.NoError(t, err, "audit failure must roll back the authoritative delete")
	audits, err := repos.Deletion.FindByStudentID(ctx, consent.StudentID)
	require.NoError(t, err)
	assert.Empty(t, audits, "the audit append must roll back with its delete")

	fault.afterAppend = nil
	deleteFault.afterDelete = nil
	result, err := svc.CleanupExpiredVisits(ctx)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.EqualValues(t, 1, result.RecordsDeleted)
	_, err = presence.FindVisit(ctx, visit.ID)
	require.ErrorIs(t, err, studentpresence.ErrVisitNotFound)
	audits, err = repos.Deletion.FindByStudentID(ctx, consent.StudentID)
	require.NoError(t, err)
	require.Len(t, audits, 1)

	result, err = svc.CleanupExpiredVisits(ctx)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Zero(t, result.RecordsDeleted)
	audits, err = repos.Deletion.FindByStudentID(ctx, consent.StudentID)
	require.NoError(t, err)
	require.Len(t, audits, 1, "an idempotent retry must not duplicate the audit")
	assert.Equal(t, 1, audits[0].RecordsDeleted)
	for _, id := range []int64{open.ID, recent.ID} {
		_, err := presence.FindVisit(ctx, id)
		require.NoError(t, err, "retention must preserve open and recent visits")
	}
}

type failedRetentionRead struct {
	activeService.PresenceRetention
	oldestErr, monthlyErr error
}

func (*failedRetentionRead) CountExpiredVisits(context.Context) (int64, error) {
	return 1, nil
}

func (*failedRetentionRead) ListVisitRetentionCounts(context.Context) ([]studentpresence.VisitRetentionCount, error) {
	return []studentpresence.VisitRetentionCount{{StudentID: 1, Count: 1}}, nil
}

func (r *failedRetentionRead) OldestExpiredVisitDate(context.Context) (*time.Time, error) {
	oldest := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	return &oldest, r.oldestErr
}

func (r *failedRetentionRead) ListExpiredVisitMonths(context.Context) ([]studentpresence.VisitMonthCount, error) {
	return nil, r.monthlyErr
}

func TestRetentionReportsOwnerReadFailures(t *testing.T) {
	t.Parallel()
	injected := errors.New("retention read failed")
	for _, tc := range []struct {
		name    string
		port    *failedRetentionRead
		preview bool
	}{
		{name: "statistics oldest visit", port: &failedRetentionRead{oldestErr: injected}},
		{name: "statistics monthly counts", port: &failedRetentionRead{monthlyErr: injected}},
		{name: "preview oldest visit", port: &failedRetentionRead{oldestErr: injected}, preview: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := activeService.NewCleanupService(tc.port, nil, nil, nil, nil, nil)
			if tc.preview {
				result, err := svc.PreviewCleanup(context.Background())
				require.ErrorIs(t, err, injected)
				require.Nil(t, result, "a failed read must not produce a complete-looking preview")
			} else {
				result, err := svc.GetRetentionStatistics(context.Background())
				require.ErrorIs(t, err, injected)
				require.Nil(t, result, "a failed read must not produce complete-looking statistics")
			}
		})
	}
}
