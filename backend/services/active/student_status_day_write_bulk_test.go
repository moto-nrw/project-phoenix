package active

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateForDates_RejectsConflictWithoutPartialWrites(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repoFactory := repositories.NewFactory(db)
	service := NewStudentStatusDayService(repoFactory.StudentStatusDay)
	studentService := users.NewStudentService(repoFactory.Student, repoFactory.PrivacyConsent, repoFactory.StudentCompanion, nil)
	student := testpkg.CreateTestStudent(t, db, "StatusConflict", "Student", "SCS1")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	ctx := testpkg.TenantContext(1)
	conflictDate := timezone.TodayDate().AddDays(40)
	freshDate := conflictDate.AddDays(1)
	require.NoError(t, repoFactory.StudentStatusDay.UpsertReported(ctx, &activeModels.StudentStatusDay{
		StudentID:  student.ID,
		Date:       conflictDate,
		Status:     activeModels.StudentStatusDaySick,
		ReportedAt: time.Now(),
		Source:     activeModels.StudentStatusSourceParent,
	}))

	err := service.CreateForDates(ctx, StatusDayWriteContext{
		DB:             db,
		TenantID:       1,
		StudentService: studentService,
		Authorize:      func(context.Context, *userModels.Student) bool { return true },
		AfterCommit:    func(int64) {},
	}, student.ID, activeModels.StudentStatusDayExcused, "Termin", []timezone.Date{conflictDate, freshDate})

	var conflictErr *StudentStatusDayConflictError
	require.ErrorAs(t, err, &conflictErr)
	require.Len(t, conflictErr.Conflicts, 1)
	assert.Equal(t, conflictDate, conflictErr.Conflicts[0].Date)
	assert.Equal(t, activeModels.StudentStatusDaySick, conflictErr.Conflicts[0].Status)

	rows, findErr := service.GetActiveByStudentAndDateRange(ctx, student.ID, conflictDate, freshDate)
	require.NoError(t, findErr)
	require.Len(t, rows, 1, "a conflict must reject the entire write")
	assert.Equal(t, conflictDate, rows[0].Date)
	assert.Equal(t, activeModels.StudentStatusDaySick, rows[0].Status)
}

// Bulk writes must preflight existing status rows the same way single-student
// CreateForDates does: any active conflict rejects the whole selection and
// leaves every student's rows unchanged.
func TestBulkCreateForDates_RejectsConflictWithoutPartialWrites(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repoFactory := repositories.NewFactory(db)
	service := NewStudentStatusDayService(repoFactory.StudentStatusDay)
	studentService := users.NewStudentService(repoFactory.Student, repoFactory.PrivacyConsent, repoFactory.StudentCompanion, nil)

	withConflict := testpkg.CreateTestStudent(t, db, "BulkStatusConflict", "Student", "BSC1")
	clear := testpkg.CreateTestStudent(t, db, "BulkStatusClear", "Student", "BSC2")
	defer testpkg.CleanupActivityFixtures(t, db, withConflict.ID, clear.ID)

	ctx := testpkg.TenantContext(1)
	conflictDate := timezone.TodayDate().AddDays(50)
	freshDate := conflictDate.AddDays(1)
	require.NoError(t, repoFactory.StudentStatusDay.UpsertReported(ctx, &activeModels.StudentStatusDay{
		StudentID:  withConflict.ID,
		Date:       conflictDate,
		Status:     activeModels.StudentStatusDaySick,
		ReportedAt: time.Now(),
		Source:     activeModels.StudentStatusSourceParent,
	}))

	err := service.BulkCreateForDates(ctx, StatusDayWriteContext{
		DB:             db,
		TenantID:       1,
		StudentService: studentService,
		Authorize:      func(context.Context, *userModels.Student) bool { return true },
		AfterCommit:    func(int64) {},
	}, []int64{withConflict.ID, clear.ID}, activeModels.StudentStatusDayClassTrip, "Klassenfahrt", []timezone.Date{conflictDate, freshDate})

	var conflictErr *StudentStatusDayConflictError
	require.ErrorAs(t, err, &conflictErr)
	require.Len(t, conflictErr.Conflicts, 1)
	assert.Equal(t, withConflict.ID, conflictErr.Conflicts[0].StudentID)
	assert.Equal(t, conflictDate, conflictErr.Conflicts[0].Date)
	assert.Equal(t, activeModels.StudentStatusDaySick, conflictErr.Conflicts[0].Status)

	for _, studentID := range []int64{withConflict.ID, clear.ID} {
		rows, findErr := service.GetActiveByStudentAndDateRange(ctx, studentID, conflictDate, freshDate)
		require.NoError(t, findErr)
		if studentID == withConflict.ID {
			require.Len(t, rows, 1, "conflict student must keep the original row only")
			assert.Equal(t, conflictDate, rows[0].Date)
			assert.Equal(t, activeModels.StudentStatusDaySick, rows[0].Status)
			continue
		}
		assert.Empty(t, rows, "clear student must not receive any rows on conflict")
	}
}

// Mixed-scope bulk status writes must fail closed before any row lands.
// Regression for the class-trip bulk partial-commit path under outer withTx.
func TestBulkCreateForDates_RejectsUnauthorizedWithoutPartialWrites(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repoFactory := repositories.NewFactory(db)
	service := NewStudentStatusDayService(repoFactory.StudentStatusDay)
	studentService := users.NewStudentService(repoFactory.Student, repoFactory.PrivacyConsent, repoFactory.StudentCompanion, nil)

	allowed := testpkg.CreateTestStudent(t, db, "BulkStatusAllowed", "Student", "BSA1")
	denied := testpkg.CreateTestStudent(t, db, "BulkStatusDenied", "Student", "BSD1")
	defer testpkg.CleanupActivityFixtures(t, db, allowed.ID, denied.ID)

	ctx := testpkg.TenantContext(1)
	dates := []timezone.Date{timezone.NewDate(2026, 5, 12)}
	err := service.BulkCreateForDates(ctx, StatusDayWriteContext{
		DB:             db,
		TenantID:       1,
		StudentService: studentService,
		Authorize: func(_ context.Context, student *userModels.Student) bool {
			return student.ID == allowed.ID
		},
		AfterCommit: func(int64) {},
	}, []int64{allowed.ID, denied.ID}, activeModels.StudentStatusDayClassTrip, "Klassenfahrt", dates)

	require.ErrorIs(t, err, ErrStudentStatusDayReassigned)

	for _, studentID := range []int64{allowed.ID, denied.ID} {
		rows, findErr := service.GetActiveByStudentAndDateRange(ctx, studentID, dates[0], dates[0])
		require.NoError(t, findErr)
		assert.Empty(t, rows, "authorization failure must not leave status days for student %d", studentID)
	}
}
