package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	activeService "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/services/active"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateForDates_RejectsConflictWithoutPartialWrites(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	service := activeService.NewStudentStatusDayServiceWithPartialAbsences(repoFactory.StudentStatusDay, nil, nil, repoFactory.CarePlan().LockExceptionDay)
	studentService := services.StatusDayStudentsFromRepository(repoFactory.Student, services.AllowAllStatusDayWrites)
	student := testpkg.CreateTestStudent(t, db, "StatusConflict", "Student", "SCS1")

	ctx := testpkg.Ctx(t)
	conflictDate := timezone.NewDate(2026, 8, 24).AddDays(40)
	freshDate := conflictDate.AddDays(1)
	require.NoError(t, repoFactory.StudentStatusDay.UpsertReported(ctx, &absencerecords.StudentStatusDay{
		StudentID:  student.ID,
		Date:       conflictDate,
		Status:     absencerecords.StudentStatusDaySick,
		ReportedAt: time.Now(),
		Source:     absencerecords.StudentStatusSourceParent,
	}))

	err := service.CreateForDates(ctx, activeService.StatusDayWriteContext{
		DB:             db,
		TenantID:       testpkg.Tenant(t),
		StudentService: studentService,
		AfterCommit:    func(int64) {},
	}, student.ID, absencerecords.StudentStatusDayExcused, "Termin", []timezone.Date{conflictDate, freshDate})

	var conflictErr *activeService.StudentStatusDayConflictError
	require.ErrorAs(t, err, &conflictErr)
	require.Len(t, conflictErr.Conflicts, 1)
	assert.Equal(t, conflictDate, conflictErr.Conflicts[0].Date)
	assert.Equal(t, absencerecords.StudentStatusDaySick, conflictErr.Conflicts[0].Status)

	rows, findErr := service.GetActiveByStudentAndDateRange(ctx, student.ID, conflictDate, freshDate)
	require.NoError(t, findErr)
	require.Len(t, rows, 1, "a conflict must reject the entire write")
	assert.Equal(t, conflictDate, rows[0].Date)
	assert.Equal(t, absencerecords.StudentStatusDaySick, rows[0].Status)
}

// Bulk writes must preflight existing status rows the same way single-student
// CreateForDates does: any active conflict rejects the whole selection and
// leaves every student's rows unchanged.
func TestBulkCreateForDates_RejectsConflictWithoutPartialWrites(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	service := activeService.NewStudentStatusDayServiceWithPartialAbsences(repoFactory.StudentStatusDay, nil, nil, repoFactory.CarePlan().LockExceptionDay)
	studentService := services.StatusDayStudentsFromRepository(repoFactory.Student, services.AllowAllStatusDayWrites)

	withConflict := testpkg.CreateTestStudent(t, db, "BulkStatusConflict", "Student", "BSC1")
	clear := testpkg.CreateTestStudent(t, db, "BulkStatusClear", "Student", "BSC2")

	ctx := testpkg.Ctx(t)
	conflictDate := timezone.NewDate(2026, 8, 24).AddDays(50)
	freshDate := conflictDate.AddDays(1)
	require.NoError(t, repoFactory.StudentStatusDay.UpsertReported(ctx, &absencerecords.StudentStatusDay{
		StudentID:  withConflict.ID,
		Date:       conflictDate,
		Status:     absencerecords.StudentStatusDaySick,
		ReportedAt: time.Now(),
		Source:     absencerecords.StudentStatusSourceParent,
	}))

	err := service.BulkCreateForDates(ctx, activeService.StatusDayWriteContext{
		DB:             db,
		TenantID:       testpkg.Tenant(t),
		StudentService: studentService,
		AfterCommit:    func(int64) {},
	}, []int64{withConflict.ID, clear.ID}, absencerecords.StudentStatusDayClassTrip, "Klassenfahrt", []timezone.Date{conflictDate, freshDate})

	var conflictErr *activeService.StudentStatusDayConflictError
	require.ErrorAs(t, err, &conflictErr)
	require.Len(t, conflictErr.Conflicts, 1)
	assert.Equal(t, 1, conflictErr.ConflictTotal())
	assert.Equal(t, withConflict.ID, conflictErr.Conflicts[0].StudentID)
	assert.Equal(t, conflictDate, conflictErr.Conflicts[0].Date)
	assert.Equal(t, absencerecords.StudentStatusDaySick, conflictErr.Conflicts[0].Status)

	for _, studentID := range []int64{withConflict.ID, clear.ID} {
		rows, findErr := service.GetActiveByStudentAndDateRange(ctx, studentID, conflictDate, freshDate)
		require.NoError(t, findErr)
		if studentID == withConflict.ID {
			require.Len(t, rows, 1, "conflict student must keep the original row only")
			assert.Equal(t, conflictDate, rows[0].Date)
			assert.Equal(t, absencerecords.StudentStatusDaySick, rows[0].Status)
			continue
		}
		assert.Empty(t, rows, "clear student must not receive any rows on conflict")
	}
}

func TestStudentStatusDayConflictError_SampleAndTotal(t *testing.T) {
	t.Parallel()

	rows := make([]*absencerecords.StudentStatusDay, 0, activeService.MaxStudentStatusDayConflictDetails+5)
	for i := 0; i < activeService.MaxStudentStatusDayConflictDetails+5; i++ {
		rows = append(rows, &absencerecords.StudentStatusDay{StudentID: int64(i + 1)})
	}

	capped := &activeService.StudentStatusDayConflictError{Conflicts: rows, Total: 100}
	assert.Equal(t, 100, capped.ConflictTotal())
	assert.Len(t, capped.SampleConflicts(), activeService.MaxStudentStatusDayConflictDetails)
	assert.Equal(t, int64(1), capped.SampleConflicts()[0].StudentID)

	uncapped := &activeService.StudentStatusDayConflictError{Conflicts: rows[:3]}
	assert.Equal(t, 3, uncapped.ConflictTotal())
	assert.Len(t, uncapped.SampleConflicts(), 3)
}

// Mixed-scope bulk status writes must fail closed before any row lands.
// Regression for the class-trip bulk partial-commit path under outer withTx.
func TestBulkCreateForDates_RejectsUnauthorizedWithoutPartialWrites(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	service := activeService.NewStudentStatusDayServiceWithPartialAbsences(repoFactory.StudentStatusDay, nil, nil, repoFactory.CarePlan().LockExceptionDay)

	allowed := testpkg.CreateTestStudent(t, db, "BulkStatusAllowed", "Student", "BSA1")
	denied := testpkg.CreateTestStudent(t, db, "BulkStatusDenied", "Student", "BSD1")

	studentService := services.StatusDayStudentsFromRepository(repoFactory.Student,
		func(_ context.Context, student *activeService.StudentRecord, _ string) bool {
			return student.ID == allowed.ID
		})

	ctx := testpkg.Ctx(t)
	dates := []timezone.Date{timezone.NewDate(2026, 5, 12)}
	err := service.BulkCreateForDates(ctx, activeService.StatusDayWriteContext{
		DB:             db,
		TenantID:       testpkg.Tenant(t),
		StudentService: studentService,
		AfterCommit:    func(int64) {},
	}, []int64{allowed.ID, denied.ID}, absencerecords.StudentStatusDayClassTrip, "Klassenfahrt", dates)

	require.ErrorIs(t, err, activeService.ErrStudentStatusDayReassigned)

	for _, studentID := range []int64{allowed.ID, denied.ID} {
		rows, findErr := service.GetActiveByStudentAndDateRange(ctx, studentID, dates[0], dates[0])
		require.NoError(t, findErr)
		assert.Empty(t, rows, "authorization failure must not leave status days for student %d", studentID)
	}
}
