package presence_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	service "github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type interceptStatusStudent struct {
	studentpresence.StatusDayStudents
	beforeLock func()
}

func (s interceptStatusStudent) LockForStatusWrite(ctx context.Context, id int64, status string) (*studentpresence.StudentRecord, error) {
	s.beforeLock()
	return s.StatusDayStudents.LockForStatusWrite(ctx, id, status)
}

func TestDeleteStatusDayProtectsSchoolSource(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	factory := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	people := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	student := testpkg.CreateTestStudent(t, db, "School", "Report", "3a")
	ctx := testpkg.Ctx(t)
	date := calendar.NewDate(2099, 3, 2)
	row := &absencerecords.StudentStatusDay{StudentID: student.ID, Date: date, Status: "sick", Source: "school", ReportedAt: time.Now()}
	require.NoError(t, factory.StudentStatusDay.UpsertReported(ctx, row))
	rows, err := factory.StudentStatusDay.FindActiveByStudentAndDateRange(ctx, student.ID, date, date)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	svc := service.NewStudentStatusDayServiceWithPartialAbsences(factory.StudentStatusDay, nil, nil, factory.CarePlan().LockExceptionDay)
	err = svc.DeleteByID(ctx, studentpresence.StatusDayWriteContext{
		DB: db, TenantID: testpkg.Tenant(t), StudentService: api.StatusDayStudentsFromRepository(people.Student, api.AllowAllStatusDayWrites), AfterCommit: func(int64) {},
	}, rows[0].ID, student.ID)
	require.ErrorIs(t, err, studentpresence.ErrStudentStatusDayReassigned)
	rows, err = factory.StudentStatusDay.FindActiveByStudentAndDateRange(ctx, student.ID, date, date)
	require.NoError(t, err)
	require.Len(t, rows, 1, "the ordinary delete must preserve school reports")
}

func TestDeleteStatusDayRechecksSourceAfterConcurrentReplacement(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	factory := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	people := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	student := testpkg.CreateTestStudent(t, db, "School", "Replacement", "3a")
	ctx := testpkg.Ctx(t)
	date := calendar.NewDate(2099, 3, 2)
	row := &absencerecords.StudentStatusDay{StudentID: student.ID, Date: date, Status: "sick", Source: "school", ReportedAt: time.Now()}
	require.NoError(t, factory.StudentStatusDay.UpsertReported(ctx, row))
	rows, err := factory.StudentStatusDay.FindActiveByStudentAndDateRange(ctx, student.ID, date, date)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	svc := service.NewStudentStatusDayServiceWithPartialAbsences(factory.StudentStatusDay, nil, nil, factory.CarePlan().LockExceptionDay)
	guarded := interceptStatusStudent{StatusDayStudents: api.StatusDayStudentsFromRepository(people.Student, api.AllowAllStatusDayWrites), beforeLock: func() {
		// A committed same-status upsert in the gap preserves the row ID.
		row.Source = "parent"
		require.NoError(t, factory.StudentStatusDay.UpsertReported(ctx, row))
	}}
	err = svc.DeleteByID(ctx, studentpresence.StatusDayWriteContext{
		DB: db, TenantID: testpkg.Tenant(t), StudentService: guarded, Source: "school", AfterCommit: func(int64) {},
	}, rows[0].ID, student.ID)
	require.Error(t, err)
	rows, err = factory.StudentStatusDay.FindActiveByStudentAndDateRange(ctx, student.ID, date, date)
	require.NoError(t, err)
	require.Len(t, rows, 1, "a replacement from parents must survive the withdrawal")
	require.Equal(t, "parent", rows[0].Source)
}
