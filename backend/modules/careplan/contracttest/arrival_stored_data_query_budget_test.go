package contracttest_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestStoredArrivalDataQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	owner := careplantest.NewCarePlan(t, db, testpkg.StudentDirectory{})
	service, err := compose.NewArrivalSchedules(db, owner, nil, nil, compose.ArrivalScheduleDependencies{})
	require.NoError(t, err)
	staff := testpkg.CreateTestStaff(t, db, "Stored", "Data")
	student := testpkg.CreateTestStudent(t, db, "Stored", "Data", "1a")
	empty := testpkg.CreateTestStudent(t, db, "Empty", "Data", "1a")
	date := calendar.NewDate(2026, time.November, 2)
	expected := &careplan.StudentArrivalData{
		Schedules: []*careplan.ArrivalSchedule{}, Exceptions: []*careplan.ArrivalException{}, Notes: []*careplan.ArrivalNote{},
	}
	for _, day := range []int{1, 5} {
		notes := "stored schedule"
		row, createErr := owner.CreateArrivalSchedule(ctx, careplan.ArrivalSchedule{
			StudentID: student.ID, Weekday: day, CreatedBy: staff.ID, Notes: &notes,
			ExpectedArrival: time.Date(1, 1, 1, 8, day, 0, 0, time.UTC),
		})
		require.NoError(t, createErr)
		expected.Schedules = append(expected.Schedules, &row)
	}
	exception, err := owner.CreateArrivalException(ctx, careplan.ArrivalException{
		StudentID: student.ID, ExceptionDate: careplan.Date(date.AddDays(10)), CreatedBy: staff.ID,
	})
	require.NoError(t, err)
	expected.Exceptions = append(expected.Exceptions, &exception)
	note, err := owner.CreateArrivalNote(ctx, careplan.ArrivalNote{
		StudentID: student.ID, NoteDate: careplan.Date(date.AddDays(-10)), Content: "stored note", CreatedBy: staff.ID,
	})
	require.NoError(t, err)
	expected.Notes = append(expected.Notes, &note)
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	foreignStudent := testpkg.CreateTestStudentForTenant(t, db, otherTenant, "Foreign", "Data", "1a")
	foreignStaff := testpkg.CreateTestStaffForTenant(t, db, otherTenant, "Foreign", "Data")
	_, err = owner.CreateArrivalSchedule(tenant.WithTenantID(ctx, otherTenant), careplan.ArrivalSchedule{
		StudentID: foreignStudent.ID, Weekday: 1, CreatedBy: foreignStaff.ID,
		ExpectedArrival: time.Date(1, 1, 1, 9, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	counter := testpkg.CaptureQueriesForContext(t, db)
	readers := []struct {
		name string
		read func(context.Context, int64) (*careplan.StudentArrivalData, error)
	}{
		{"date", func(ctx context.Context, id int64) (*careplan.StudentArrivalData, error) {
			return service.GetStudentArrivalDataForDate(ctx, id, date)
		}},
		{"range", func(ctx context.Context, id int64) (*careplan.StudentArrivalData, error) {
			return service.GetStudentArrivalDataForDateRange(ctx, id, date.AddDays(1), date.AddDays(2))
		}},
		{"default", service.GetStudentArrivalData},
	}
	for _, reader := range readers {
		t.Run(reader.name, func(t *testing.T) {
			counter.Reset()
			actual, readErr := testpkg.TenantQuery1(t, counter.Context(ctx), db, reader.read, student.ID)
			require.NoError(t, readErr)
			require.Equal(t, expected, actual, "fallback keeps the entire stored week, exceptions and notes, including dates outside the requested range")
			switch reader.name {
			case "date":
				querybudgets.AssertQueryBudget(t, "modules.careplan.arrival.stored_data.date", counter.Measurement(counter.Queries()))
			case "range":
				querybudgets.AssertQueryBudget(t, "modules.careplan.arrival.stored_data.range", counter.Measurement(counter.Queries()))
			case "default":
				querybudgets.AssertQueryBudget(t, "modules.careplan.arrival.stored_data.default", counter.Measurement(counter.Queries()))
			}
			for _, id := range []int64{empty.ID, foreignStudent.ID} {
				actual, readErr = testpkg.TenantQuery1(t, ctx, db, reader.read, id)
				require.NoError(t, readErr)
				require.Equal(t, &careplan.StudentArrivalData{
					Schedules: []*careplan.ArrivalSchedule{}, Exceptions: []*careplan.ArrivalException{}, Notes: []*careplan.ArrivalNote{},
				}, actual, "empty and foreign plans keep non-nil empty slices")
			}
		})
	}
	actual, err := testpkg.TenantQuery3(t, ctx, db, service.GetStudentArrivalDataForDateRange, student.ID, date, date.AddDays(-1))
	require.Nil(t, actual)
	require.EqualError(t, err, "schedule error during project arrival schedule range: arrival schedule range ends before it starts")
}
