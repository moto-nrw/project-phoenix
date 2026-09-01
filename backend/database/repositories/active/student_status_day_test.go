package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudentStatusDayRepository_UpsertAndFind(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StudentStatusDay
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "StatusRepo", "Student", "SR1")

	date := timezone.NewDate(2026, 8, 24).AddDays(3)
	reportedAt := time.Now().Add(-time.Hour)
	entry := &active.StudentStatusDay{
		StudentID:  student.ID,
		Date:       date,
		Status:     active.StudentStatusDaySick,
		ReportedAt: reportedAt,
		Source:     active.StudentStatusSourcePlanned,
	}
	require.NoError(t, repo.UpsertReported(ctx, entry))

	rows, err := repo.FindActiveByStudentAndDateRange(ctx, student.ID, date.AddDays(-1), date.AddDays(1))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, student.ID, rows[0].StudentID)
	assert.Equal(t, active.StudentStatusDaySick, rows[0].Status)
	assert.True(t, rows[0].Date == date)
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StudentStatusDay
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "StatusClear", "Student", "SC1")

	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	firstDate := timezone.DateFromTime(now).AddDays(4)
	secondDate := timezone.DateFromTime(now).AddDays(5)
	for _, date := range []timezone.Date{firstDate, secondDate} {
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
		if row.Date == firstDate {
			firstRowID = row.ID
		}
	}
	require.NotZero(t, firstRowID)
	require.NoError(t, repo.MarkClearedByID(ctx, firstRowID, now, active.StudentStatusSourceManual))
	require.NoError(t, repo.MarkClearedForDates(ctx, student.ID, active.StudentStatusDayExcused, []timezone.Date{secondDate, secondDate}, now, active.StudentStatusSourceManual))
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StudentStatusDay
	student := testpkg.CreateTestStudent(t, db, "StatusTenant", "Student", "ST1")

	date := timezone.NewDate(2026, 8, 24).AddDays(6)
	require.NoError(t, repo.UpsertReported(context.Background(), &active.StudentStatusDay{
		TenantModel: modelBase.TenantModel{TenantID: testpkg.Tenant(t)},
		StudentID:   student.ID,
		Date:        date,
		Status:      active.StudentStatusDaySick,
		ReportedAt:  time.Now(),
		Source:      active.StudentStatusSourcePlanned,
	}))

	rows, err := repo.FindActiveByStudentAndDateRange(testpkg.TenantContext(2), student.ID, date, date)
	require.NoError(t, err)
	assert.Empty(t, rows)

	rows, err = repo.FindActiveByStudentAndDateRange(testpkg.Ctx(t), student.ID, date, date)
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

func TestStudentStatusDayRepository_CountEffectiveDashboardAbsences(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StudentStatusDay
	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)
	ctxA := testpkg.TenantContext(tenantA)
	ctxB := testpkg.TenantContext(tenantB)

	today := timezone.TodayDate()
	now := time.Now()
	trueValue := true
	studentIDs := make([]int64, 0, 10)
	create := func(first, last string) int64 {
		student := testpkg.CreateTestStudentForTenant(t, db, tenantA, first, last, "DA1")
		studentIDs = append(studentIDs, student.ID)
		return student.ID
	}
	setFlag := func(studentID int64, flag string) {
		_, err := db.NewUpdate().
			TableExpr(`users.students`).
			Set(flag+" = ?", trueValue).
			Where("id = ?", studentID).
			Exec(ctxA)
		require.NoError(t, err)
	}
	report := func(ctx context.Context, studentID int64, status string) {
		require.NoError(t, repo.UpsertReported(ctx, &active.StudentStatusDay{
			StudentID:  studentID,
			Date:       today,
			Status:     status,
			ReportedAt: now,
			Source:     active.StudentStatusSourcePlanned,
		}))
	}

	setFlag(create("FlagSick", "Dashboard"), "sick")
	report(ctxA, create("PlannedSick", "Dashboard"), active.StudentStatusDaySick)
	report(ctxA, create("PlannedExcused", "Dashboard"), active.StudentStatusDayExcused)
	setFlag(create("FlagExcused", "Dashboard"), "excused")

	overlap := create("Overlap", "Dashboard")
	setFlag(overlap, "excused")
	report(ctxA, overlap, active.StudentStatusDayExcused)

	report(ctxA, create("ClassTrip", "Dashboard"), active.StudentStatusDayClassTrip)

	sickWins := create("SickWins", "Dashboard")
	report(ctxA, sickWins, active.StudentStatusDaySick)
	report(ctxA, sickWins, active.StudentStatusDayExcused)
	report(ctxA, sickWins, active.StudentStatusDayClassTrip)

	cleared := create("Cleared", "Dashboard")
	report(ctxA, cleared, active.StudentStatusDayExcused)
	require.NoError(t, repo.MarkCleared(ctxA, cleared, active.StudentStatusDayExcused, today, now, active.StudentStatusSourceManual))

	inactive := create("Inactive", "Dashboard")
	report(ctxA, inactive, active.StudentStatusDayExcused)
	_, err := db.NewUpdate().
		TableExpr(`users.students`).
		Set("status = ?", string(usersModels.StudentStatusInactive)).
		Where("id = ?", inactive).
		Exec(ctxA)
	require.NoError(t, err)

	otherTenantStudent := testpkg.CreateTestStudentForTenant(t, db, tenantB, "OtherTenant", "Dashboard", "DB1")
	studentIDs = append(studentIDs, otherTenantStudent.ID)
	report(ctxB, otherTenantStudent.ID, active.StudentStatusDayExcused)

	counts, err := repo.CountEffectiveDashboardAbsences(ctxA, today)
	require.NoError(t, err)
	require.NotNil(t, counts)
	assert.Equal(t, 3, counts.Sick)
	assert.Equal(t, 4, counts.Excused)

	otherCounts, err := repo.CountEffectiveDashboardAbsences(ctxB, today)
	require.NoError(t, err)
	require.NotNil(t, otherCounts)
	assert.Equal(t, 0, otherCounts.Sick)
	assert.Equal(t, 1, otherCounts.Excused)
}

func TestStudentStatusDayRepository_UpsertNil(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StudentStatusDay

	err := repo.UpsertReported(testpkg.Ctx(t), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

// TestStudentStatusDayRepository_NoteOnReReport pins the upsert's note
// handling: an active re-report with no reason keeps the prior note, but
// reactivating a previously-cleared row with no reason must NOT resurrect
// the stale note from the superseded report.
func TestStudentStatusDayRepository_NoteOnReReport(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StudentStatusDay
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "StatusNote", "Student", "SN1")

	date := timezone.NewDate(2026, 8, 24).AddDays(5)
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

// TestStudentStatusDayRepository_DateBoundaryRoundtrip proves the
// timezone.Date storage contract end to end at a simulated midnight-window
// instant: the stored DATE equals the Berlin calendar day regardless of the
// Go process timezone (CI runs UTC) and the DB session timezone (CI container
// runs Europe/Berlin), the Scanner roundtrips it, WHERE equality matches, and
// a re-upsert hits the conflict path instead of duplicating the row — the
// exact failure mode of the historical bug class.
func TestStudentStatusDayRepository_DateBoundaryRoundtrip(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StudentStatusDay
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Boundary", "Student", "BR1")

	// 23:30 UTC on the eve of the 2026 spring DST transition is 00:30 CEST
	// on March 29 in Berlin — inside the historical failure window.
	boundaryInstant := time.Date(2026, 3, 28, 23, 30, 0, 0, time.UTC)
	d := timezone.DateFromTime(boundaryInstant)
	require.Equal(t, timezone.NewDate(2026, 3, 29), d)

	entry := &active.StudentStatusDay{
		StudentID:  student.ID,
		Date:       d,
		Status:     active.StudentStatusDayClassTrip,
		ReportedAt: boundaryInstant,
		Source:     active.StudentStatusSourcePlanned,
	}
	require.NoError(t, repo.UpsertReported(ctx, entry))

	// Raw readback: the stored DATE must be the Berlin calendar day,
	// independent of any driver or session timezone conversion.
	var stored string
	require.NoError(t, db.NewRaw(
		"SELECT to_char(date, 'YYYY-MM-DD') FROM active.student_status_days WHERE student_id = ? AND status = ?",
		student.ID, active.StudentStatusDayClassTrip,
	).Scan(ctx, &stored))
	assert.Equal(t, "2026-03-29", stored)

	// Scanner roundtrip + WHERE equality at the boundary date.
	rows, err := repo.FindActiveByStudentAndDateRange(ctx, student.ID, d, d)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, d, rows[0].Date)

	byIDs, err := repo.FindActiveByStudentIDsAndDate(ctx, []int64{student.ID}, d)
	require.NoError(t, err)
	require.Len(t, byIDs, 1)

	// Re-upsert at the same date must hit the conflict path, not insert a
	// second row for a shifted day.
	entry2 := &active.StudentStatusDay{
		StudentID:  student.ID,
		Date:       d,
		Status:     active.StudentStatusDayClassTrip,
		ReportedAt: boundaryInstant.Add(time.Hour),
		Source:     active.StudentStatusSourceManual,
	}
	require.NoError(t, repo.UpsertReported(ctx, entry2))
	rows, err = repo.FindActiveByStudentAndDateRange(ctx, student.ID, d, d)
	require.NoError(t, err)
	require.Len(t, rows, 1, "second upsert must update, not duplicate")
	assert.Equal(t, active.StudentStatusSourceManual, rows[0].Source)

	// The zero Date is rejected loudly instead of silently storing a
	// sentinel day.
	zeroEntry := &active.StudentStatusDay{
		StudentID:  student.ID,
		Status:     active.StudentStatusDaySick,
		ReportedAt: boundaryInstant,
		Source:     active.StudentStatusSourcePlanned,
	}
	require.Error(t, repo.UpsertReported(ctx, zeroEntry))
}
