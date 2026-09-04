package compose

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type recordingStudentNames struct {
	values []StudentName
	calls  int
}

func (p *recordingStudentNames) ListStudentNamesByID(context.Context, []int64) ([]StudentName, error) {
	p.calls++
	return p.values, nil
}

func TestCompanionFacadePreservesNamesIsolationAndQueryCount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	first := testpkg.CreateTestStudent(t, db, "Erstes", "Kind", "1a")
	second := testpkg.CreateTestStudent(t, db, "Zweites", "Kind", "1a")
	people := &recordingStudentNames{values: []StudentName{{
		StudentID: second.ID, FirstName: "Zweites", LastName: "Kind",
	}}}
	var observations []Observation
	module, err := New(Dependencies{
		DB: db, Observe: func(value Observation) { observations = append(observations, value) },
		AmbientDB: func(context.Context) bun.IDB { return db }, People: people,
		StatusStudents:  emptyPeopleDirectory{},
		StudentLock:     func(context.Context, int64) error { return nil },
		StudentNotFound: errors.New("student not found"),
	})
	require.NoError(t, err)

	require.NoError(t, module.ReplaceCompanionEdges(ctx, first.ID, []careplan.CompanionEdge{{
		StudentLowID: first.ID, StudentHighID: second.ID, Weekday: 1,
	}}))
	links, err := module.ListCompanionLinks(ctx, []int64{first.ID})
	require.NoError(t, err)
	require.Len(t, links[first.ID], 1)
	assert.Equal(t, "Zweites", links[first.ID][0].FirstName)
	assert.Equal(t, 1, people.calls, "all companion names use one People Directory batch query")

	var listObservation *Observation
	for i := range observations {
		if observations[i].Operation == "list_companion_links" {
			listObservation = &observations[i]
		}
	}
	require.NotNil(t, listObservation)
	assert.EqualValues(t, 1, listObservation.Stats.Queries)

	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	otherCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), otherTenantID)
	foreign, err := module.ListCompanionLinks(otherCtx, []int64{first.ID})
	require.NoError(t, err)
	assert.Empty(t, foreign)
}

func TestNamedCarePlanTablesEnforceTwoTenantRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	firstCtx := testpkg.Ctx(t)
	firstTenantID := tenant.FromContext(firstCtx)
	first := testpkg.CreateTestStudent(t, db, "Isolated", "Care A", "1a")
	firstCompanion := testpkg.CreateTestStudent(t, db, "Companion", "Care A", "1a")
	firstStaff := testpkg.CreateTestStaff(t, db, "Care", "Owner A")
	firstAccount := testpkg.CreateTestAccount(t, db, "parent")
	seedNamedCarePlanTables(t, module, firstCtx, first.ID, firstCompanion.ID, firstStaff.ID, firstAccount.ID)

	secondTenantID := testpkg.UniqueTestTenantID(t)
	secondCtx := tenantContext(t, db, secondTenantID)
	second := testpkg.CreateTestStudentForTenant(t, db, secondTenantID, "Isolated", "Care B", "1b")
	secondCompanion := testpkg.CreateTestStudentForTenant(t, db, secondTenantID, "Companion", "Care B", "1b")
	secondStaff := testpkg.CreateTestStaffForTenant(t, db, secondTenantID, "Care", "Owner B")
	secondAccount := testpkg.CreateTestAccount(t, db, "parent")
	seedNamedCarePlanTables(t, module, secondCtx, second.ID, secondCompanion.ID, secondStaff.ID, secondAccount.ID)

	assertNamedCarePlanTableCounts(t, db, firstCtx, firstTenantID)
	assertNamedCarePlanTableCounts(t, db, secondCtx, secondTenantID)
}

func TestRequestAndStatusNotFoundErrorsAreStable(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	missingID := int64(9_223_372_036_854_775_000)

	_, statusErr := module.FindStudentStatusDay(ctx, missingID, true)
	_, excusedErr := module.FindExcusedAbsenceRequest(ctx, missingID, false)
	_, scheduleErr := module.FindCareScheduleRequest(ctx, missingID, false)
	_, dataErr := module.FindStudentDataRequest(ctx, missingID, false)

	assert.ErrorIs(t, statusErr, careplan.ErrStudentStatusDayNotFound)
	assert.ErrorIs(t, excusedErr, careplan.ErrExcusedRequestNotFound)
	assert.ErrorIs(t, scheduleErr, careplan.ErrCareScheduleRequestNotFound)
	assert.ErrorIs(t, dataErr, careplan.ErrStudentDataRequestNotFound)
	for _, err := range []error{statusErr, excusedErr, scheduleErr, dataErr} {
		assert.Equal(t, "not_found", careplan.ErrorCode(err))
	}
}

func TestRequestAndStatusReadFailuresAreObserved(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	observations := make([]Observation, 0, 4)
	module := buildModule(t, db, func(observation Observation) { observations = append(observations, observation) })
	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()

	_, statusErr := module.ListStudentStatusDays(ctx, careplan.StudentStatusDayFilter{})
	_, excusedErr := module.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{})
	_, scheduleErr := module.ListCareScheduleRequests(ctx, careplan.CareScheduleRequestFilter{})
	_, dataErr := module.ListStudentDataRequests(ctx, careplan.StudentDataRequestFilter{})

	for _, err := range []error{statusErr, excusedErr, scheduleErr, dataErr} {
		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, "internal_error", careplan.ErrorCode(err))
	}
	require.Len(t, observations, 4)
	assert.Equal(t, []string{
		"list_student_status_days", "list_excused_absence_requests",
		"list_care_schedule_requests", "list_student_data_requests",
	}, []string{
		observations[0].Operation, observations[1].Operation,
		observations[2].Operation, observations[3].Operation,
	})
	for _, observation := range observations {
		require.ErrorIs(t, observation.Err, context.Canceled)
	}
}

func TestCareScheduleDuplicateConflictIsObserved(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	observations := make([]Observation, 0, 2)
	module := buildModule(t, db, func(observation Observation) { observations = append(observations, observation) })
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Duplicate", "Request", "1a")
	account := testpkg.CreateTestAccount(t, db, "parent")
	request := careplan.CareScheduleChangeRequest{
		StudentID: student.ID, SubmittedBy: account.ID, RequestKind: "weekly_schedule",
		Payload: json.RawMessage(`{"weekdays":[]}`), Status: "pending",
	}

	_, err := module.CreateCareScheduleRequest(ctx, request)
	require.NoError(t, err)
	_, err = module.CreateCareScheduleRequest(ctx, request)
	require.Error(t, err)
	require.Len(t, observations, 2)
	assert.Equal(t, "create_care_schedule_request", observations[1].Operation)
	assert.EqualValues(t, 1, observations[1].Stats.Conflicts)
	assert.Error(t, observations[1].Err)
}

func seedNamedCarePlanTables(t *testing.T, module *careplan.Module, ctx context.Context, studentID, companionID, staffID, accountID int64) {
	t.Helper()
	enrollmentID := studentID + 1_000_000
	require.NoError(t, module.UpsertCareExit(ctx, careplan.CareExit{StudentID: studentID, Reason: careplan.CareExitReasonMovedAway}))
	require.NoError(t, module.RecordCareExitRemovals(ctx, []careplan.CareExitRemoval{{
		StudentID: studentID, Kind: careplan.CareExitRemovalBooking, EnrollmentID: &enrollmentID,
	}}))
	require.NoError(t, module.RecordCareExitSourceRemovals(ctx, []careplan.CareExitSourceRemoval{{
		StudentID: studentID, Kind: careplan.CareExitSourceBooking,
		SourceRowID: enrollmentID, Snapshot: json.RawMessage(`{"id":1}`),
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

func assertNamedCarePlanTableCounts(t *testing.T, db *bun.DB, ctx context.Context, tenantID int64) {
	t.Helper()
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		scoped, activeTenantID, databaseErr := carePlanDatabase(db)(txCtx)
		require.NoError(t, databaseErr)
		assert.Equal(t, tenantID, activeTenantID)
		for _, table := range []string{
			"users.student_care_exit_removals",
			"users.student_care_exit_source_removals",
			"users.student_care_exits",
			"users.student_companions",
			"schedule.student_arrival_schedules",
			"schedule.student_arrival_exceptions",
			"schedule.student_arrival_notes",
			"schedule.student_pickup_schedules",
			"schedule.student_pickup_exceptions",
			"schedule.student_pickup_notes",
			"active.excused_absence_requests",
			"active.student_status_days",
			"schedule.care_schedule_change_requests",
			"users.student_data_change_requests",
		} {
			count, countErr := scoped.NewSelect().TableExpr(table).Count(txCtx)
			require.NoError(t, countErr)
			assert.Equal(t, 1, count, "%s must expose only the active tenant's row", table)
		}
		return nil
	})
	require.NoError(t, err)
}

func TestRemovalLedgerDuplicatePreventionIsObserved(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	observations := make([]Observation, 0, 4)
	module := buildModule(t, db, func(observation Observation) { observations = append(observations, observation) })
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Duplicate", "Ledger", "1a")
	enrollmentID := student.ID + 1_000_000
	removal := careplan.CareExitRemoval{
		StudentID: student.ID, Kind: careplan.CareExitRemovalBooking, EnrollmentID: &enrollmentID,
	}
	source := careplan.CareExitSourceRemoval{
		StudentID: student.ID, Kind: careplan.CareExitSourceBooking,
		SourceRowID: enrollmentID, Snapshot: json.RawMessage(`{"id":1}`),
	}

	require.NoError(t, module.RecordCareExitRemovals(ctx, []careplan.CareExitRemoval{removal}))
	require.NoError(t, module.RecordCareExitRemovals(ctx, []careplan.CareExitRemoval{removal}))
	require.NoError(t, module.RecordCareExitSourceRemovals(ctx, []careplan.CareExitSourceRemoval{source}))
	require.NoError(t, module.RecordCareExitSourceRemovals(ctx, []careplan.CareExitSourceRemoval{source}))
	missingStudentID := int64(9_223_372_036_854_775_000)
	missingEnrollmentID := missingStudentID - 1
	err := module.RecordCareExitRemovals(ctx, []careplan.CareExitRemoval{{
		StudentID: missingStudentID, Kind: careplan.CareExitRemovalBooking, EnrollmentID: &missingEnrollmentID,
	}})
	require.Error(t, err)

	assert.EqualValues(t, 1, observations[1].Stats.Conflicts, "%+v", observations)
	assert.EqualValues(t, 1, observations[3].Stats.Conflicts, "%+v", observations)
	assert.EqualValues(t, 0, observations[1].Stats.Rows)
	assert.EqualValues(t, 0, observations[3].Stats.Rows)
	assert.EqualValues(t, 0, observations[4].Stats.Conflicts, "failed writes are not duplicate conflicts")
}

func TestNamedCarePlanReadFailuresAreObservedAndNotSwallowed(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	observations := make([]Observation, 0, 4)
	module := buildModule(t, db, func(observation Observation) { observations = append(observations, observation) })
	student := testpkg.CreateTestStudent(t, db, "Read", "Failure", "1a")
	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()

	_, exitErr := module.FindCareExits(ctx, []int64{student.ID})
	_, removalErr := module.ListCareExitRemovals(ctx, []int64{student.ID})
	_, sourceErr := module.ListCareExitSourceRemovals(ctx, []int64{student.ID})
	_, companionErr := module.ListCompanionEdges(ctx, student.ID)
	_, arrivalScheduleErr := module.ListArrivalSchedules(ctx, careplan.StudentScheduleFilter{})
	_, arrivalExceptionErr := module.ListArrivalExceptions(ctx, careplan.StudentScheduleFilter{})
	_, arrivalNoteErr := module.ListArrivalNotes(ctx, careplan.StudentScheduleFilter{})
	_, pickupScheduleErr := module.ListPickupSchedules(ctx, careplan.StudentScheduleFilter{})
	_, pickupExceptionErr := module.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{})
	_, pickupNoteErr := module.ListPickupNotes(ctx, careplan.StudentScheduleFilter{})

	for _, err := range []error{exitErr, removalErr, sourceErr, companionErr, arrivalScheduleErr, arrivalExceptionErr, arrivalNoteErr, pickupScheduleErr, pickupExceptionErr, pickupNoteErr} {
		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, "internal_error", careplan.ErrorCode(err))
	}
	require.Len(t, observations, 10)
	for _, observation := range observations {
		require.ErrorIs(t, observation.Err, context.Canceled)
	}
}

func TestCompanionDuplicateConflictRollsBackDeleteAndRetries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	observations := make([]Observation, 0, 3)
	module := buildModule(t, db, func(observation Observation) { observations = append(observations, observation) })
	ctx := testpkg.Ctx(t)
	first := testpkg.CreateTestStudent(t, db, "Conflict", "Subject", "1a")
	original := testpkg.CreateTestStudent(t, db, "Conflict", "Original", "1a")
	replacement := testpkg.CreateTestStudent(t, db, "Conflict", "Replacement", "1a")
	require.NoError(t, module.ReplaceCompanionEdges(ctx, first.ID, []careplan.CompanionEdge{{
		StudentLowID: first.ID, StudentHighID: original.ID, Weekday: 1,
	}}))
	duplicate := careplan.CompanionEdge{StudentLowID: first.ID, StudentHighID: replacement.ID, Weekday: 2}

	err := module.ReplaceCompanionEdges(ctx, first.ID, []careplan.CompanionEdge{duplicate, duplicate})
	require.Error(t, err)
	edges, readErr := module.ListCompanionEdges(ctx, first.ID)
	require.NoError(t, readErr)
	require.Len(t, edges, 1, "the insert conflict must roll back the preceding delete")
	assert.Equal(t, original.ID, edges[0].StudentHighID)
	assert.EqualValues(t, 1, observations[1].Stats.Conflicts)

	require.NoError(t, module.ReplaceCompanionEdges(ctx, first.ID, []careplan.CompanionEdge{duplicate}))
	edges, readErr = module.ListCompanionEdges(ctx, first.ID)
	require.NoError(t, readErr)
	require.Len(t, edges, 1)
	assert.Equal(t, replacement.ID, edges[0].StudentHighID)
}

func TestCareRecordCommandsRollBackAndRetry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	t.Run("care exit", func(t *testing.T) { testCareExitRollbackAndRetry(t, db, module, ctx) })
	t.Run("companion", func(t *testing.T) { testCompanionRollbackAndRetry(t, db, module, ctx) })
	t.Run("removal ledgers", func(t *testing.T) { testRemovalLedgersRollbackAndRetry(t, db, module, ctx) })
	t.Run("care document and cleanup intent", func(t *testing.T) { testCareDocumentRollbackAndRetry(t, db, module, ctx) })
}

func testCareExitRollbackAndRetry(t *testing.T, db *bun.DB, module *careplan.Module, ctx context.Context) {
	student := testpkg.CreateTestStudent(t, db, "Exit", "Rollback", "1a")
	assertRollbackAndRetry(t, ctx,
		func(commandCtx context.Context) error {
			return module.UpsertCareExit(commandCtx, careplan.CareExit{StudentID: student.ID, Reason: careplan.CareExitReasonMovedAway})
		},
		func(queryCtx context.Context) bool {
			values, err := module.FindCareExits(queryCtx, []int64{student.ID})
			require.NoError(t, err)
			return len(values) == 1
		},
	)
}

func testCompanionRollbackAndRetry(t *testing.T, db *bun.DB, module *careplan.Module, ctx context.Context) {
	first := testpkg.CreateTestStudent(t, db, "Companion", "Rollback", "1a")
	second := testpkg.CreateTestStudent(t, db, "Companion", "Retry", "1a")
	assertRollbackAndRetry(t, ctx,
		func(commandCtx context.Context) error {
			return module.ReplaceCompanionEdges(commandCtx, first.ID, []careplan.CompanionEdge{{
				StudentLowID: first.ID, StudentHighID: second.ID, Weekday: 1,
			}})
		},
		func(queryCtx context.Context) bool {
			values, err := module.ListCompanionEdges(queryCtx, first.ID)
			require.NoError(t, err)
			return len(values) == 1
		},
	)
}

func testRemovalLedgersRollbackAndRetry(t *testing.T, db *bun.DB, module *careplan.Module, ctx context.Context) {
	student := testpkg.CreateTestStudent(t, db, "Ledger", "Rollback", "1a")
	enrollmentID := student.ID + 1_000_000
	assertRollbackAndRetry(t, ctx,
		func(commandCtx context.Context) error {
			if err := module.RecordCareExitRemovals(commandCtx, []careplan.CareExitRemoval{{
				StudentID: student.ID, Kind: careplan.CareExitRemovalBooking, EnrollmentID: &enrollmentID,
			}}); err != nil {
				return err
			}
			return module.RecordCareExitSourceRemovals(commandCtx, []careplan.CareExitSourceRemoval{{
				StudentID: student.ID, Kind: careplan.CareExitSourceBooking,
				SourceRowID: enrollmentID, Snapshot: json.RawMessage(`{"id":1}`),
			}})
		},
		func(queryCtx context.Context) bool { return removalLedgersPresent(t, module, queryCtx, student.ID) },
	)
}

func removalLedgersPresent(t *testing.T, module *careplan.Module, ctx context.Context, studentID int64) bool {
	removals, err := module.ListCareExitRemovals(ctx, []int64{studentID})
	require.NoError(t, err)
	sources, err := module.ListCareExitSourceRemovals(ctx, []int64{studentID})
	require.NoError(t, err)
	return len(removals) == 1 && len(sources) == 1
}

func testCareDocumentRollbackAndRetry(t *testing.T, db *bun.DB, module *careplan.Module, ctx context.Context) {
	student := testpkg.CreateTestStudent(t, db, "Document", "Rollback", "1a")
	filename := fmt.Sprintf("care-record-rollback-%d.pdf", student.ID)
	assertRollbackAndRetry(t, ctx,
		func(commandCtx context.Context) error {
			if _, err := module.CreateCareDocument(commandCtx, careplan.CareDocument{
				StudentID: student.ID, Category: "attest", FilenameDisplay: "attest.pdf",
				FilenameStored: filename, SizeBytes: 42, ContentType: "application/pdf", UploadedBy: 1,
			}); err != nil {
				return err
			}
			_, err := module.QueueCareDocumentCleanup(commandCtx, careplan.CareDocumentCleanup{
				OwnerID: student.ID, FilenameStored: filename, RetryAfter: time.Now(),
			})
			return err
		},
		func(queryCtx context.Context) bool {
			return careDocumentRecordsPresent(t, module, queryCtx, student.ID)
		},
	)
}

func careDocumentRecordsPresent(t *testing.T, module *careplan.Module, ctx context.Context, studentID int64) bool {
	values, err := module.ListCareDocuments(ctx, studentID, []string{"attest"})
	require.NoError(t, err)
	cleanups, err := module.ListCareDocumentCleanups(ctx, &studentID)
	require.NoError(t, err)
	return len(values) == 1 && len(cleanups) == 1
}

func assertRollbackAndRetry(
	t *testing.T,
	ctx context.Context,
	command func(context.Context) error,
	present func(context.Context) bool,
) {
	t.Helper()
	rollback := errors.New("force rollback after authoritative write")
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		require.NoError(t, command(txCtx))
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	assert.False(t, present(ctx), "the forced failure must roll back every write")
	require.NoError(t, command(ctx))
	assert.True(t, present(ctx), "the retry must apply the command once")
}
