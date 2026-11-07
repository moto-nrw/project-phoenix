package contracttest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestCarePlanDeletionCountsKeepCategoriesAndTenantScope(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module := careplantest.NewCarePlan(t, db, testpkg.StudentDirectory{})
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Deletion", "Counts", "1a")
	companion := testpkg.CreateTestStudent(t, db, "Deletion", "Companion", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Deletion", "Staff")
	account := testpkg.CreateTestAccount(t, db, "parent")
	seedDeletionCountCategories(t, module, ctx, student.ID, companion.ID, staff.ID, account.ID)
	_, err := module.SaveDepartureException(ctx, careplan.DepartureException{
		StudentID: student.ID, ExceptionDate: "2031-02-03", DepartureMode: careplan.DepartureModePickup, CreatedBy: &staff.ID,
	})
	require.NoError(t, err)

	// The original scalar remains an independent oracle for all seven
	// schedule categories, including notes and departure exceptions.
	schedules, err := module.CountStudentScheduleRows(ctx, student.ID)
	require.NoError(t, err)
	require.Equal(t, 7, schedules)
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		counter := testpkg.CaptureQueriesForContext(t, db)
		counts, countErr := module.CountCarePlanDeletionRecords(counter.Context(txCtx), student.ID)
		require.NoError(t, countErr)
		require.Equal(t, careplan.CarePlanDeletionCounts{
			ScheduleRows: schedules, StatusDays: 1, ExcusedRequests: 1, CareRequests: 1, DataRequests: 1,
		}, counts)
		require.Len(t, counter.Queries(), 1, "the owner payload reads one window, without opening another transaction")
		return nil
	})
	require.NoError(t, err)

	// A real second school cannot count any category of the first school's
	// child, even when given the known child ID.
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	otherCtx := tenant.WithTenantID(testpkg.WithTenantRuntime(t, context.Background(), db), otherTenant)
	counts, err := module.CountCarePlanDeletionRecords(otherCtx, student.ID)
	require.NoError(t, err)
	require.Equal(t, careplan.CarePlanDeletionCounts{}, counts)
	counts, err = module.CountCarePlanDeletionRecords(ctx, companion.ID)
	require.NoError(t, err)
	require.Equal(t, careplan.CarePlanDeletionCounts{}, counts)
}

func seedDeletionCountCategories(t *testing.T, module careplan.Capability, ctx context.Context, studentID, companionID, staffID, accountID int64) {
	t.Helper()
	enrollmentID := studentID + 1_000_000
	require.NoError(t, module.UpsertCareExit(ctx, careplan.CareExit{StudentID: studentID, Reason: careplan.CareExitReasonMovedAway}))
	require.NoError(t, module.RecordCareExitRemovals(ctx, []careplan.CareExitRemoval{{
		StudentID: studentID, Kind: careplan.CareExitRemovalBooking, EnrollmentID: &enrollmentID,
	}}))
	require.NoError(t, module.RecordCareExitSourceRemovals(ctx, []careplan.CareExitSourceRemoval{{
		StudentID: studentID, Kind: careplan.CareExitSourceBooking,
		SourceRowID: enrollmentID, Snapshot: json.RawMessage(fmt.Sprintf(`{"id":%d}`, enrollmentID)),
	}}))
	require.NoError(t, module.ReplaceCompanionEdges(ctx, studentID, []careplan.CompanionEdge{{
		StudentLowID: studentID, StudentHighID: companionID, Weekday: 1,
	}}))
	date := careplan.Date("2031-02-03")
	_, err := module.CreateArrivalSchedule(ctx, careplan.ArrivalSchedule{StudentID: studentID, Weekday: 1, CreatedBy: staffID})
	require.NoError(t, err)
	_, err = module.CreateArrivalException(ctx, careplan.ArrivalException{StudentID: studentID, ExceptionDate: date, CreatedBy: staffID})
	require.NoError(t, err)
	_, err = module.CreateArrivalNote(ctx, careplan.ArrivalNote{StudentID: studentID, NoteDate: date, Content: "Hinweis", CreatedBy: staffID})
	require.NoError(t, err)
	_, err = module.CreatePickupSchedule(ctx, careplan.PickupSchedule{StudentID: studentID, Weekday: 1, PickupTime: time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC), CreatedBy: staffID})
	require.NoError(t, err)
	_, err = module.CreatePickupException(ctx, careplan.PickupException{StudentID: studentID, ExceptionDate: date, CreatedBy: staffID})
	require.NoError(t, err)
	_, err = module.CreatePickupNote(ctx, careplan.PickupNote{StudentID: studentID, NoteDate: date, Content: "Hinweis", CreatedBy: staffID})
	require.NoError(t, err)
	_, err = module.UpsertStudentStatusDay(ctx, careplan.StudentStatusDay{
		StudentID: studentID, Date: date, Status: "sick", ReportedAt: time.Now(), Source: "parent",
	})
	require.NoError(t, err)
	_, err = module.CreateExcusedAbsenceRequest(ctx, careplan.ExcusedAbsenceRequest{
		StudentID: studentID, SubmittedBy: accountID, Dates: []careplan.Date{date},
		AbsenceStatus: "excused", Status: "pending",
	})
	require.NoError(t, err)
	_, err = module.CreateCareScheduleRequest(ctx, careplan.CareScheduleChangeRequest{
		StudentID: studentID, SubmittedBy: accountID, RequestKind: "weekly_schedule",
		Payload: json.RawMessage(`{"weekdays":[]}`), Status: "pending",
	})
	require.NoError(t, err)
	_, err = module.CreateStudentDataRequest(ctx, careplan.StudentDataChangeRequest{
		StudentID: studentID, SubmittedBy: accountID, Target: "person", FieldKey: "first_name",
		NewValue: json.RawMessage(`"Neu"`), Status: "pending",
	})
	require.NoError(t, err)
}
