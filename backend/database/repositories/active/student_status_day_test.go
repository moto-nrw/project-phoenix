package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudentStatusDayRepository_UpsertAndFind(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentStatusDay
	ctx := testpkg.TenantContext(1)
	student := testpkg.CreateTestStudent(t, db, "StatusRepo", "Student", "SR1")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	date := timezone.DateOfUTC(time.Now().AddDate(0, 0, 3))
	reportedAt := time.Now().Add(-time.Hour)
	entry := &active.StudentStatusDay{
		StudentID:  student.ID,
		Date:       date.Add(8 * time.Hour),
		Status:     active.StudentStatusDaySick,
		ReportedAt: reportedAt,
		Source:     active.StudentStatusSourcePlanned,
	}
	require.NoError(t, repo.UpsertReported(ctx, entry))

	rows, err := repo.FindActiveByStudentAndDateRange(ctx, student.ID, date.AddDate(0, 0, -1), date.AddDate(0, 0, 1))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, student.ID, rows[0].StudentID)
	assert.Equal(t, active.StudentStatusDaySick, rows[0].Status)
	assert.True(t, timezone.DateOfUTC(rows[0].Date).Equal(date))
	assert.Equal(t, active.StudentStatusSourcePlanned, rows[0].Source)

	entry.ReportedAt = reportedAt.Add(time.Hour)
	entry.Source = active.StudentStatusSourceManual
	require.NoError(t, repo.UpsertReported(ctx, entry))

	rows, err = repo.FindByStudentAndDateRange(ctx, student.ID, date, date)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, active.StudentStatusSourceManual, rows[0].Source)
	assert.True(t, rows[0].ClearedAt == nil)

	byID, err := repo.FindActiveByID(ctx, rows[0].ID)
	require.NoError(t, err)
	assert.Equal(t, rows[0].ID, byID.ID)

	byStudents, err := repo.FindActiveByStudentIDsAndDate(ctx, []int64{student.ID}, date)
	require.NoError(t, err)
	require.Len(t, byStudents, 1)
	assert.Equal(t, rows[0].ID, byStudents[0].ID)

	empty, err := repo.FindActiveByStudentIDsAndDate(ctx, nil, date)
	require.NoError(t, err)
	assert.Empty(t, empty)

	require.NoError(t, repo.MarkCleared(ctx, student.ID, active.StudentStatusDaySick, date, time.Now(), active.StudentStatusSourceNextCheckin))

	activeRows, err := repo.FindActiveByStudentAndDateRange(ctx, student.ID, date, date)
	require.NoError(t, err)
	assert.Empty(t, activeRows)

	allRows, err := repo.FindByStudentAndDateRange(ctx, student.ID, date, date)
	require.NoError(t, err)
	require.Len(t, allRows, 1)
	assert.NotNil(t, allRows[0].ClearedAt)
	assert.Equal(t, active.StudentStatusSourceNextCheckin, allRows[0].Source)
}

func TestStudentStatusDayRepository_ClearByIDAndDates(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentStatusDay
	ctx := testpkg.TenantContext(1)
	student := testpkg.CreateTestStudent(t, db, "StatusClear", "Student", "SC1")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	now := time.Now()
	firstDate := timezone.DateOfUTC(now.AddDate(0, 0, 4))
	secondDate := timezone.DateOfUTC(now.AddDate(0, 0, 5))
	for _, date := range []time.Time{firstDate, secondDate} {
		require.NoError(t, repo.UpsertReported(ctx, &active.StudentStatusDay{
			StudentID:  student.ID,
			Date:       date,
			Status:     active.StudentStatusDayExcused,
			ReportedAt: now,
			Source:     active.StudentStatusSourcePlanned,
		}))
	}

	rows, err := repo.FindActiveByStudentAndDateRange(ctx, student.ID, firstDate, secondDate)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	var firstRowID int64
	for _, row := range rows {
		if timezone.DateOfUTC(row.Date).Equal(firstDate) {
			firstRowID = row.ID
		}
	}
	require.NotZero(t, firstRowID)
	require.NoError(t, repo.MarkClearedByID(ctx, firstRowID, now, active.StudentStatusSourceManual))
	require.NoError(t, repo.MarkClearedForDates(ctx, student.ID, active.StudentStatusDayExcused, []time.Time{secondDate, secondDate.Add(2 * time.Hour)}, now, active.StudentStatusSourceManual))
	require.NoError(t, repo.MarkClearedForDates(ctx, student.ID, active.StudentStatusDayExcused, nil, now, active.StudentStatusSourceManual))

	activeRows, err := repo.FindActiveByStudentAndDateRange(ctx, student.ID, firstDate, secondDate)
	require.NoError(t, err)
	assert.Empty(t, activeRows)

	allRows, err := repo.FindByStudentAndDateRange(ctx, student.ID, firstDate, secondDate)
	require.NoError(t, err)
	require.Len(t, allRows, 2)
	for _, row := range allRows {
		assert.NotNil(t, row.ClearedAt)
		assert.Equal(t, active.StudentStatusSourceManual, row.Source)
	}
}

func TestStudentStatusDayRepository_TenantScope(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentStatusDay
	student := testpkg.CreateTestStudent(t, db, "StatusTenant", "Student", "ST1")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	date := timezone.DateOfUTC(time.Now().AddDate(0, 0, 6))
	require.NoError(t, repo.UpsertReported(context.Background(), &active.StudentStatusDay{
		TenantModel: modelBase.TenantModel{TenantID: 1},
		StudentID:   student.ID,
		Date:        date,
		Status:      active.StudentStatusDaySick,
		ReportedAt:  time.Now(),
		Source:      active.StudentStatusSourcePlanned,
	}))

	rows, err := repo.FindActiveByStudentAndDateRange(tenant.WithTenantID(context.Background(), 2), student.ID, date, date)
	require.NoError(t, err)
	assert.Empty(t, rows)

	rows, err = repo.FindActiveByStudentAndDateRange(testpkg.TenantContext(1), student.ID, date, date)
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

func TestStudentStatusDayRepository_UpsertNil(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentStatusDay

	err := repo.UpsertReported(testpkg.TenantContext(1), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

// TestStudentStatusDayRepository_NoteOnReReport pins the upsert's note
// handling: an active re-report with no reason keeps the prior note, but
// reactivating a previously-cleared row with no reason must NOT resurrect
// the stale note from the superseded report.
func TestStudentStatusDayRepository_NoteOnReReport(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentStatusDay
	ctx := testpkg.TenantContext(1)
	student := testpkg.CreateTestStudent(t, db, "StatusNote", "Student", "SN1")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	date := timezone.DateOfUTC(time.Now().AddDate(0, 0, 5))
	reason := "Fieber"

	// 1. Report sick with a reason.
	require.NoError(t, repo.UpsertReported(ctx, &active.StudentStatusDay{
		StudentID:  student.ID,
		Date:       date,
		Status:     active.StudentStatusDaySick,
		ReportedAt: time.Now(),
		Source:     active.StudentStatusSourceParent,
		Note:       &reason,
	}))

	// 2. Active re-report without a reason — note must be preserved.
	require.NoError(t, repo.UpsertReported(ctx, &active.StudentStatusDay{
		StudentID:  student.ID,
		Date:       date,
		Status:     active.StudentStatusDaySick,
		ReportedAt: time.Now(),
		Source:     active.StudentStatusSourceParent,
	}))
	rows, err := repo.FindActiveByStudentAndDateRange(ctx, student.ID, date, date)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].Note, "active re-report without a reason must keep the prior note")
	assert.Equal(t, reason, *rows[0].Note)

	// 3. Clear the day, then re-report sick with NO reason.
	require.NoError(t, repo.MarkCleared(ctx, student.ID, active.StudentStatusDaySick, date, time.Now(), active.StudentStatusSourceNextCheckin))
	require.NoError(t, repo.UpsertReported(ctx, &active.StudentStatusDay{
		StudentID:  student.ID,
		Date:       date,
		Status:     active.StudentStatusDaySick,
		ReportedAt: time.Now(),
		Source:     active.StudentStatusSourceParent,
	}))

	rows, err = repo.FindActiveByStudentAndDateRange(ctx, student.ID, date, date)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].Note, "reactivating a cleared row without a reason must not resurrect the old note")
}
