package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	repoFactory "github.com/moto-nrw/project-phoenix/database/repositories"
	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingDeleteStudentFieldEditRepo struct {
	auditModels.StudentFieldEditRepository
	err error
}

func (r *failingDeleteStudentFieldEditRepo) DeleteOlderThan(context.Context, time.Time) (int64, error) {
	return 0, r.err
}

func TestStudentChangeLogCleanup_DeleteFailureRollsBackDeletionAudit(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.TenantContext(1)

	student := testpkg.CreateTestStudent(t, db, "Cleanup", "Rollback", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)
	defer func() {
		_, _ = db.NewRaw(`DELETE FROM audit.data_deletions WHERE student_id = ?`, student.ID).Exec(context.Background())
		_, _ = db.NewRaw(`DELETE FROM audit.student_field_edits WHERE student_id = ?`, student.ID).Exec(context.Background())
	}()

	repos := repoFactory.NewFactory(db)
	edit := &auditModels.StudentFieldEdit{
		StudentID:    student.ID,
		EditedBy:     auditModels.StudentFieldEditSystemActorID,
		EditedByName: auditModels.StudentFieldEditSystemActorName,
		FieldName:    auditModels.StudentFieldSupervisorNotes,
		OldValue:     testpkg.StrPtr("alt"),
		NewValue:     testpkg.StrPtr("neu"),
		CreatedAt:    time.Now().AddDate(0, 0, -200),
	}
	require.NoError(t, repos.StudentFieldEdit.CreateBatch(ctx, []*auditModels.StudentFieldEdit{edit}))

	scheduleNow := time.Now()
	if scheduleNow.Second() >= 58 {
		t.Skip("skipping to avoid minute-boundary race on timeMatchesNow")
	}

	deleteErr := errors.New("delete failed")
	cleanup := usersSvc.NewStudentChangeLogCleanupService(
		&failingDeleteStudentFieldEditRepo{
			StudentFieldEditRepository: repos.StudentFieldEdit,
			err:                        deleteErr,
		},
		repos.DataDeletion,
		&configtest.Mock{
			ResolveIntFn: func(_ context.Context, key string) (int, error) {
				if key != configModel.KeyGDPRStudentChangeLogRetentionDays {
					return 0, fmt.Errorf("unexpected setting key %q", key)
				}
				return 90, nil
			},
		},
		slog.Default(),
	)
	s := &Scheduler{
		db:                      db,
		schoolRepo:              platformRepo.NewSchoolRepository(db),
		studentChangeLogCleanup: cleanup,
		settings: &fakeSettingsResolver{
			boolValues: map[string]bool{
				configModel.KeyDataCleanupEnabled: true,
			},
			stringValues: map[string]string{
				configModel.KeyDataCleanupTime: scheduleNow.Format("15:04"),
			},
			intValues: map[string]int{
				configModel.KeyDataCleanupTimeoutMinutes: 30,
			},
		},
		logger: slog.Default(),
	}

	s.checkAndRunStudentChangeLogCleanup(&ScheduledTask{Name: "student-change-log-cleanup"})

	editCount, err := db.NewSelect().
		ModelTableExpr(`audit.student_field_edits`).
		Where(`student_id = ?`, student.ID).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, editCount, "failed delete must leave the change-history row intact")

	auditCount, err := db.NewSelect().
		ModelTableExpr(`audit.data_deletions`).
		Where(`student_id = ?`, student.ID).
		Where(`deletion_type = ?`, auditModels.DeletionTypeStudentChangeLogRetention).
		Count(context.Background())
	require.NoError(t, err)
	assert.Zero(t, auditCount, "failed delete must roll back its deletion audit")

	_, ranToday := s.lastStudentChangeLogCleanup.Load(student.TenantID)
	assert.False(t, ranToday, "failed cleanup must remain eligible for retry")
}
