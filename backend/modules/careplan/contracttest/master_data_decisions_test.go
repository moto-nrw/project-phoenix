package contracttest_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	"github.com/moto-nrw/project-phoenix/modules/communication/communicationtest"
	requestreviewcompose "github.com/moto-nrw/project-phoenix/modules/requestreview/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The Stammdaten decision is Care Plan's (#3354). These suites drive it over
// the retained People Directory repositories the composition root binds, so
// they prove the decision and the child's record together.

// masterDataFixture is one child with a guardian who files Stammdaten
// requests, in the test's own tenant.
type masterDataFixture struct {
	db    *bun.DB
	repos *repositories.Factory
	chain testpkg.ParentChain
}

func newMasterDataFixture(t *testing.T, db *bun.DB) masterDataFixture {
	t.Helper()
	return masterDataFixture{
		db:    db,
		repos: repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)),
		chain: testpkg.CreateTestParentGuardianChain(t, db),
	}
}

// decisions composes the decision with a school-wide review scope unless a
// test narrows it.
func (f masterDataFixture) decisions(t *testing.T, configure ...func(*services.MasterDataDecisionTestOptions)) *careplanCompose.MasterDataDecisions {
	t.Helper()
	options := services.MasterDataDecisionTestOptions{
		CarePlan: f.repos.CarePlan(), People: repositories.MustNewPeopleDirectory(f.db),
		Students: f.repos.Student, Persons: f.repos.Person, Logger: slog.Default(),
	}
	for _, apply := range configure {
		apply(&options)
	}
	svc, err := services.NewTestMasterDataDecisions(options)
	require.NoError(t, err)
	return svc
}

// queue is the native open queue and history the decisions are listed from.
func (f masterDataFixture) queue(t *testing.T, scope careplanCompose.ReviewScopeResolver) careplan.MasterDataReviewQuery {
	t.Helper()
	queue, err := requestreviewcompose.NewMasterDataReviews(f.db, repositories.MustNewPeopleDirectory(f.db), scope,
		careplanCompose.Today, func(requestreviewcompose.CareObservation) {})
	require.NoError(t, err)
	return queue
}

func (f masterDataFixture) insert(t *testing.T, target, field, oldValue, newValue string) *userModels.StudentDataChangeRequest {
	t.Helper()
	row := &userModels.StudentDataChangeRequest{
		StudentID: f.chain.StudentID, SubmittedBy: f.chain.AccountID, Target: target, FieldKey: field,
		OldValue: json.RawMessage(oldValue), NewValue: json.RawMessage(newValue), Status: userModels.DataChangeStatusPending,
	}
	row.SetTenantID(f.chain.TenantID)
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), f.db, f.chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		return f.repos.StudentDataChangeRequest.Create(txCtx, row)
	}))
	return row
}

func (f masterDataFixture) inTx(t *testing.T, fn func(context.Context) error) error {
	t.Helper()
	return testpkg.WithTenantTx(t, context.Background(), f.db, f.chain.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}

func (f masterDataFixture) decide(t *testing.T, svc masterdatarequests.Decisions, input masterdatarequests.DecideInput) (*masterdatarequests.ReviewItem, error) {
	t.Helper()
	var item *masterdatarequests.ReviewItem
	err := f.inTx(t, func(ctx context.Context) error {
		var decideErr error
		item, decideErr = svc.Decide(ctx, input)
		return decideErr
	})
	return item, err
}

func (f masterDataFixture) person(t *testing.T) *userModels.Person {
	t.Helper()
	person, err := f.repos.Person.FindByID(context.Background(), f.chain.PersonID)
	require.NoError(t, err)
	return person
}

func (f masterDataFixture) status(t *testing.T, requestID int64) string {
	t.Helper()
	var status string
	require.NoError(t, f.db.NewSelect().TableExpr("users.student_data_change_requests").Column("status").
		Where("id = ?", requestID).Scan(context.Background(), &status))
	return status
}

func noReviewScope(context.Context) (careplanCompose.ReviewScope, error) {
	return careplanCompose.ReviewScope{}, nil
}

// TestMasterDataReview_ScopedToWritableChildren proves the per-child review
// gate: a caller whose review scope does not cover the child cannot see the
// request in the queue and cannot decide it — while a school-wide reviewer
// sees and decides the same request.
func TestMasterDataReview_ScopedToWritableChildren(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	row := f.insert(t, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)

	denied := f.decisions(t, func(o *services.MasterDataDecisionTestOptions) { o.Scope = noReviewScope })
	err := f.inTx(t, func(ctx context.Context) error {
		items, _, listErr := f.queue(t, noReviewScope).ListPending(ctx, careplan.RequestQueueFilter{})
		require.NoError(t, listErr)
		assert.Empty(t, items, "a caller who cannot review the child must not see its request in the queue")
		_, decideErr := denied.Decide(ctx, masterdatarequests.DecideInput{RequestID: row.ID, Approve: true})
		return decideErr
	})
	assert.ErrorIs(t, err, masterdatarequests.ErrReviewForbidden)

	err = f.inTx(t, func(ctx context.Context) error {
		items, _, listErr := f.queue(t, repositories.SchoolWideReviewScope).ListPending(ctx, careplan.RequestQueueFilter{})
		require.NoError(t, listErr)
		require.Len(t, items, 1)
		_, decideErr := f.decisions(t).Decide(ctx, masterdatarequests.DecideInput{RequestID: row.ID, Approve: false, Reason: "closing", ReviewedBy: f.chain.AccountID})
		return decideErr
	})
	require.NoError(t, err)
}

func TestMasterDataReview_ApproveAppliesNameChange(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	row := f.insert(t, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Maximilian"`)

	decided, err := f.decide(t, f.decisions(t), masterdatarequests.DecideInput{RequestID: row.ID, Approve: true})
	require.NoError(t, err)
	assert.Equal(t, masterdatarequests.StatusApproved, decided.Request.Status)
	require.NotNil(t, decided.Request.AppliedAt)
	assert.Equal(t, "Maximilian", decided.FirstName)
	assert.Equal(t, "Schneider", decided.LastName)
	assert.Equal(t, "Maximilian", f.person(t).FirstName)
}

func TestMasterDataReview_DecideRejectsStaleExpectedVersionAfterLock(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	row := f.insert(t, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)

	_, err := f.decide(t, f.decisions(t), masterdatarequests.DecideInput{RequestID: row.ID, Approve: true, ExpectedVersion: "stale"})
	require.ErrorIs(t, err, parentrequests.ErrStale)

	assert.Equal(t, userModels.DataChangeStatusPending, f.status(t, row.ID))
	assert.Equal(t, "Felix", f.person(t).FirstName)
}

// failSecondMasterData fails the second decision of a bulk command, after the
// first one already wrote to the database.
type failSecondMasterData struct {
	careplanCompose.ParentRequestMasterData
	calls int
}

func (s *failSecondMasterData) Decide(ctx context.Context, input masterdatarequests.DecideInput) (*masterdatarequests.ReviewItem, error) {
	s.calls++
	if s.calls == 2 {
		return nil, errors.New("forced second apply failure")
	}
	return s.ParentRequestMasterData.Decide(ctx, input)
}

// unusedConflictPort stands in for a queue the test never resolves.
type unusedConflictPort struct{}

func (unusedConflictPort) ConflictCandidate(context.Context, int64) (*careplanCompose.ConflictCandidate, error) {
	return nil, errors.New("unexpected conflict candidate")
}
func (unusedConflictPort) LockConflictRequest(context.Context, int64) error {
	return errors.New("unexpected conflict lock")
}
func (unusedConflictPort) DecideConflictRequest(context.Context, careplanCompose.ConflictDecision) error {
	return errors.New("unexpected conflict decision")
}
func (unusedConflictPort) WriteStaffValue(context.Context, careplanCompose.StaffValueWrite) error {
	return errors.New("unexpected staff value")
}

func TestParentRequestCoordinator_RollsBackEarlierDatabaseWrite(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	excused, err := services.NewTestExcusedAbsenceRequests(services.ExcusedRequestTestOptions{
		CarePlan: f.repos.CarePlan(), Students: f.repos.Student, Persons: f.repos.Person, Logger: slog.Default(),
	})
	require.NoError(t, err)
	coordinator, err := careplanCompose.NewParentRequestCoordinator(careplanCompose.ParentRequestCoordinatorDependencies{
		Rights: func(context.Context) careplanCompose.ParentRequestRights {
			return careplanCompose.ParentRequestRights{WriteQueues: true, Absences: true}
		},
		MasterData: &failSecondMasterData{ParentRequestMasterData: f.decisions(t)},
		Excused:    excused, Care: unusedConflictPort{}, Offering: unusedConflictPort{},
	})
	require.NoError(t, err)
	first := f.insert(t, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)
	second := f.insert(t, userModels.DataChangeTargetPerson, "last_name", `"Schneider"`, `"Becker"`)

	err = f.inTx(t, func(ctx context.Context) error {
		bulkErr := coordinator.BulkApprove(ctx, parentrequests.BulkApproveInput{
			Requests: []parentrequests.Ref{
				{Kind: parentrequests.KindMasterData, ID: first.ID, ExpectedVersion: careplan.ParentRequestVersion(first.UpdatedAt)},
				{Kind: parentrequests.KindMasterData, ID: second.ID, ExpectedVersion: careplan.ParentRequestVersion(second.UpdatedAt)},
			},
			Reason: "Gemeinsam geprüft", ReviewerID: f.chain.AccountID,
		})
		require.Error(t, bulkErr)
		return bulkErr
	})
	require.Error(t, err)

	assert.Equal(t, "Felix", f.person(t).FirstName)
	for _, requestID := range []int64{first.ID, second.ID} {
		assert.Equal(t, userModels.DataChangeStatusPending, f.status(t, requestID))
	}
}

func TestMasterDataReview_ApproveAppliesOtherPersonFields(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	svc := f.decisions(t)
	lastName := f.insert(t, userModels.DataChangeTargetPerson, "last_name", `"Schneider"`, `"Müller"`)
	birthday := f.insert(t, userModels.DataChangeTargetPerson, "birthday", `null`, `"2017-12-24"`)

	require.NoError(t, f.inTx(t, func(ctx context.Context) error {
		if _, err := svc.Decide(ctx, masterdatarequests.DecideInput{RequestID: lastName.ID, Approve: true}); err != nil {
			return err
		}
		_, err := svc.Decide(ctx, masterdatarequests.DecideInput{RequestID: birthday.ID, Approve: true})
		return err
	}))

	person := f.person(t)
	assert.Equal(t, "Müller", person.LastName)
	require.NotNil(t, person.Birthday)
	assert.Equal(t, "2017-12-24", person.Birthday.String())
}

func TestMasterDataReview_ApproveAppliesSchoolClass(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	audit := userService.NewStudentAuditService(testpkg.RequestAuditActor, repositories.NewStudentAudit(f.db))
	svc := f.decisions(t, func(o *services.MasterDataDecisionTestOptions) { o.Audit = audit })
	row := f.insert(t, userModels.DataChangeTargetStudent, "school_class", `"1a"`, `"2b"`)

	_, err := f.decide(t, svc, masterdatarequests.DecideInput{RequestID: row.ID, Approve: true, ReviewedBy: f.chain.AccountID})
	require.NoError(t, err)

	student, err := f.repos.Student.FindByID(testpkg.WithPackageTenantRuntime(context.Background()), f.chain.StudentID)
	require.NoError(t, err)
	assert.Equal(t, "2b", student.SchoolClass)
}

func TestMasterDataReview_ConcurrentPersonFieldApprovalsDoNotOverwrite(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	svc := f.decisions(t)
	firstName := f.insert(t, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)
	lastName := f.insert(t, userModels.DataChangeTargetPerson, "last_name", `"Schneider"`, `"Müller"`)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, row := range []*userModels.StudentDataChangeRequest{firstName, lastName} {
		wg.Add(1)
		go func(idx int, requestID int64) {
			defer wg.Done()
			_, errs[idx] = f.decide(t, svc, masterdatarequests.DecideInput{RequestID: requestID, Approve: true})
		}(i, row.ID)
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])

	person := f.person(t)
	assert.Equal(t, "Max", person.FirstName)
	assert.Equal(t, "Müller", person.LastName)
}

func TestMasterDataReview_ConcurrentDecisionsKeepStatusAndRecordConsistent(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	svc := f.decisions(t)
	row := f.insert(t, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, approve := range []bool{true, false} {
		wg.Add(1)
		go func(idx int, approve bool) {
			defer wg.Done()
			_, errs[idx] = f.decide(t, svc, masterdatarequests.DecideInput{RequestID: row.ID, Approve: approve})
		}(i, approve)
	}
	wg.Wait()

	successes, conflicts := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, masterdatarequests.ErrReviewNotPending):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent decision error: %v", err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)

	decided, err := f.repos.StudentDataChangeRequest.FindByID(tenant.WithTenantID(context.Background(), f.chain.TenantID), row.ID)
	require.NoError(t, err)
	switch decided.Status {
	case userModels.DataChangeStatusApproved:
		assert.Equal(t, "Max", f.person(t).FirstName)
	case userModels.DataChangeStatusRejected:
		assert.Equal(t, "Felix", f.person(t).FirstName)
	default:
		t.Fatalf("unexpected final status %q", decided.Status)
	}
}

func TestMasterDataReview_ListPendingEnrichesStudentNames(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	f.insert(t, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Maximilian"`)
	f.insert(t, userModels.DataChangeTargetPerson, "last_name", `"Schneider"`, `"Müller"`)

	var items []*careplan.MasterDataReviewItem
	require.NoError(t, f.inTx(t, func(ctx context.Context) error {
		var err error
		items, _, err = f.queue(t, repositories.SchoolWideReviewScope).ListPending(ctx, careplan.RequestQueueFilter{})
		return err
	}))
	require.Len(t, items, 2)
	for _, item := range items {
		assert.Equal(t, "Felix", item.FirstName)
		assert.Equal(t, "Schneider", item.LastName)
		assert.Equal(t, f.chain.StudentID, item.Request.StudentID)
	}
}

func TestMasterDataReview_ListPendingEmptyAndInvalidRequestID(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	svc := f.decisions(t)

	err := f.inTx(t, func(ctx context.Context) error {
		items, _, listErr := f.queue(t, repositories.SchoolWideReviewScope).ListPending(ctx, careplan.RequestQueueFilter{})
		require.NoError(t, listErr)
		assert.Empty(t, items)
		_, decideErr := svc.Decide(ctx, masterdatarequests.DecideInput{RequestID: 0, Approve: true})
		return decideErr
	})
	assert.ErrorIs(t, err, masterdatarequests.ErrReviewNotFound)

	_, err = f.decide(t, svc, masterdatarequests.DecideInput{RequestID: 999_999_999, Approve: true})
	assert.ErrorIs(t, err, masterdatarequests.ErrReviewNotFound)
}

func TestMasterDataReview_ApproveAppliesDepartureModes(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	audit := userService.NewStudentAuditService(testpkg.RequestAuditActor, repositories.NewStudentAudit(f.db))
	svc := f.decisions(t, func(o *services.MasterDataDecisionTestOptions) { o.Audit = audit })
	row := f.insert(t, userModels.DataChangeTargetDeparture, "allowed_departure_modes", `{}`, `{"mon":["bus"],"wed":["pickup"]}`)

	_, err := f.decide(t, svc, masterdatarequests.DecideInput{RequestID: row.ID, Approve: true, ReviewedBy: f.chain.AccountID})
	require.NoError(t, err)

	student, err := f.repos.Student.FindByID(testpkg.WithPackageTenantRuntime(context.Background()), f.chain.StudentID)
	require.NoError(t, err)
	assert.Equal(t, []userModels.DepartureMode{userModels.DepartureBus}, student.AllowedDepartureModes[userModels.PickupDayMonday])
	assert.Equal(t, []userModels.DepartureMode{userModels.DeparturePickup}, student.AllowedDepartureModes[userModels.PickupDayWednesday])

	history, err := audit.GetChangeHistory(tenant.WithTenantID(context.Background(), f.chain.TenantID), f.chain.StudentID)
	require.NoError(t, err)
	require.NotEmpty(t, history)
	var departureEditFound bool
	for _, edit := range history {
		if edit.FieldName == "departure_days" {
			departureEditFound = true
			assert.Equal(t, f.chain.AccountID, edit.EditedBy)
		}
	}
	assert.True(t, departureEditFound)
}

func TestMasterDataReview_StalePersonApprovalConflicts(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	row := f.insert(t, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)

	person := f.person(t)
	person.FirstName = "StaffEdit"
	require.NoError(t, f.repos.Person.Update(context.Background(), person))

	_, err := f.decide(t, f.decisions(t), masterdatarequests.DecideInput{RequestID: row.ID, Approve: true})
	assert.ErrorIs(t, err, masterdatarequests.ErrReviewStaleValue)
	assert.Equal(t, "StaffEdit", f.person(t).FirstName)
}

func TestMasterDataReview_StaleDepartureApprovalConflicts(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	tenantCtx := testpkg.TenantContext(f.chain.TenantID)
	row := f.insert(t, userModels.DataChangeTargetDeparture, "allowed_departure_modes", `{}`, `{"mon":["bus"]}`)

	student, err := f.repos.Student.FindByID(tenantCtx, f.chain.StudentID)
	require.NoError(t, err)
	student.AllowedDepartureModes = userModels.AllowedDepartureModes{
		userModels.PickupDayTuesday: []userModels.DepartureMode{userModels.DeparturePickup},
	}
	require.NoError(t, f.repos.Student.Update(tenantCtx, student))

	_, err = f.decide(t, f.decisions(t), masterdatarequests.DecideInput{RequestID: row.ID, Approve: true})
	assert.ErrorIs(t, err, masterdatarequests.ErrReviewStaleValue)

	student, err = f.repos.Student.FindByID(tenantCtx, f.chain.StudentID)
	require.NoError(t, err)
	assert.Equal(t, []userModels.DepartureMode{userModels.DeparturePickup}, student.AllowedDepartureModes[userModels.PickupDayTuesday])
	assert.Empty(t, student.AllowedDepartureModes[userModels.PickupDayMonday])
}

func TestMasterDataReview_ApprovalBroadcastsStudentUpdatedAfterCommit(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := f.decisions(t, func(o *services.MasterDataDecisionTestOptions) { o.Broadcaster = broadcaster })
	row := f.insert(t, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)

	require.NoError(t, f.inTx(t, func(ctx context.Context) error {
		_, err := svc.Decide(ctx, masterdatarequests.DecideInput{RequestID: row.ID, Approve: true, ReviewedBy: f.chain.AccountID})
		assert.Empty(t, broadcaster.CallsByMethod("tenant"), "broadcast must wait until the transaction commits")
		return err
	}))
	tenantCalls := broadcaster.CallsByMethod("tenant")
	require.Len(t, tenantCalls, 1)
	assert.Equal(t, realtime.EventStudentUpdated, tenantCalls[0].Event.Type)
}

func TestMasterDataReview_RejectLeavesRecordUnchanged(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	row := f.insert(t, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Maximilian"`)

	_, err := f.decide(t, f.decisions(t), masterdatarequests.DecideInput{RequestID: row.ID, Approve: false, Reason: "Bitte Nachweis"})
	require.NoError(t, err)
	assert.Equal(t, "Felix", f.person(t).FirstName, "rejected change must not touch the record")
}

func TestMasterDataReview_DecideNonPendingRejected(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	svc := f.decisions(t)
	row := f.insert(t, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Maximilian"`)

	// First decision approves; the second must fail as not-pending.
	_, err := f.decide(t, svc, masterdatarequests.DecideInput{RequestID: row.ID, Approve: true})
	require.NoError(t, err)
	_, err = f.decide(t, svc, masterdatarequests.DecideInput{RequestID: row.ID, Approve: true})
	assert.ErrorIs(t, err, masterdatarequests.ErrReviewNotPending)
	assert.Equal(t, masterdatarequests.ErrReviewNotPending.Error(), err.Error(), "the lost race keeps the sentinel's own text")
}

func TestMasterDataReview_ApproveInvalidRowsRejected(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		target string
		field  string
		value  string
		want   error
	}{
		{name: "invalid target", target: userModels.DataChangeTargetStudent, field: "health_info", value: `"x"`, want: masterdatarequests.ErrReviewInvalidTarget},
		{name: "invalid person value", target: userModels.DataChangeTargetPerson, field: "first_name", value: `123`, want: masterdatarequests.ErrReviewInvalidValue},
		{name: "invalid birthday", target: userModels.DataChangeTargetPerson, field: "birthday", value: `"bad-date"`, want: masterdatarequests.ErrReviewInvalidValue},
		{name: "invalid departure field", target: userModels.DataChangeTargetDeparture, field: "pickup_status", value: `{}`, want: masterdatarequests.ErrReviewInvalidTarget},
		{name: "invalid departure value", target: userModels.DataChangeTargetDeparture, field: "allowed_departure_modes", value: `123`, want: masterdatarequests.ErrReviewInvalidValue},
		{name: "unknown departure mode", target: userModels.DataChangeTargetDeparture, field: "allowed_departure_modes", value: `{"mon":["spaceship"]}`, want: masterdatarequests.ErrReviewInvalidValue},
		{name: "unsupported accompanied mode", target: userModels.DataChangeTargetDeparture, field: "allowed_departure_modes", value: `{"mon":["accompanied"]}`, want: masterdatarequests.ErrReviewInvalidValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
			row := f.insert(t, tt.target, tt.field, `null`, tt.value)
			_, err := f.decide(t, f.decisions(t), masterdatarequests.DecideInput{RequestID: row.ID, Approve: true})
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

// requestStatusPill returns the last request_status pill the decision wrote
// into the (child, guardian) thread, or nil when none was written.
func (f masterDataFixture) requestStatusPill(t *testing.T, guardianAccountID int64) *userModels.ParentMessage {
	t.Helper()
	var pill *userModels.ParentMessage
	require.NoError(t, f.inTx(t, func(ctx context.Context) error {
		thread, err := f.repos.ParentMessageThread.FindByStudentGuardian(ctx, f.chain.StudentID, guardianAccountID)
		if err != nil || thread == nil {
			return err
		}
		msgs, err := f.repos.ParentMessage.ListByThread(ctx, thread.ID, 50)
		if err != nil {
			return err
		}
		for _, m := range msgs {
			if m.EventType == userModels.ParentMessageEventRequestStatus {
				pill = m
			}
		}
		return nil
	}))
	return pill
}

// decisionsWithEmitter wires a REAL emitter, so the after-commit decision pill
// actually lands in the thread.
func (f masterDataFixture) decisionsWithEmitter(t *testing.T, shares careplanCompose.ShareVisibility) *careplanCompose.MasterDataDecisions {
	t.Helper()
	broadcaster := testpkg.NewRecordingBroadcaster()
	emitter := communicationtest.NewParentEventEmitter(f.db, testpkg.TenantRuntime(t, f.db), f.repos.ParentMessageThread, f.repos.ParentMessage,
		messagingOnSettings{}, broadcaster, slog.Default())
	return f.decisions(t, func(o *services.MasterDataDecisionTestOptions) {
		o.Emitter, o.Broadcaster, o.Shares = emitter, broadcaster, shares
	})
}

func TestMasterDataReview_ApproveEmitsDecisionPill(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	row := f.insert(t, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Maximilian"`)

	_, err := f.decide(t, f.decisionsWithEmitter(t, nil), masterdatarequests.DecideInput{RequestID: row.ID, Approve: true, ReviewedBy: f.chain.AccountID})
	require.NoError(t, err)

	pill := f.requestStatusPill(t, f.chain.AccountID)
	require.NotNil(t, pill, "an approved master-data request must drop a decision pill")
	assert.Equal(t, userModels.ParentMessageRequestStatusDone, pill.RequestStatus)
	assert.Equal(t, "Anfrage bestätigt, Stammdaten übernommen", pill.Body)
	assert.Equal(t, userModels.ParentMessageSenderStaff, pill.EventActorKind)
}

func TestMasterDataReview_RejectEmitsPillWithReason(t *testing.T) {
	t.Parallel()
	f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
	row := f.insert(t, userModels.DataChangeTargetPerson, "last_name", `"Schneider"`, `"Müller"`)

	_, err := f.decide(t, f.decisionsWithEmitter(t, nil), masterdatarequests.DecideInput{RequestID: row.ID, Approve: false, Reason: "Nachweis fehlt", ReviewedBy: f.chain.AccountID})
	require.NoError(t, err)

	pill := f.requestStatusPill(t, f.chain.AccountID)
	require.NotNil(t, pill, "a rejected request must drop a decision pill")
	assert.Equal(t, userModels.ParentMessageRequestStatusRejected, pill.RequestStatus)
	assert.Equal(t, "Anfrage abgelehnt: Nachweis fehlt", pill.Body)
	assert.Equal(t, "Schneider", f.person(t).LastName, "the rejection must not touch the live record")
}

// TestMasterDataDecisionTellsTheOtherGuardian is story 47 for the Stammdaten
// domain: the other guardian learns that the child's data changed, without the
// reason or the author — unless the submitter shared the request with them.
func TestMasterDataDecisionTellsTheOtherGuardian(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		shared      bool
		wantBody    string
		wantReason  string
		wantRequest string
	}{
		{name: "not shared: neutral line", wantBody: "Betreuungsstand geändert: Stammdaten"},
		{
			name: "shared: the recipient gets the full pill", shared: true,
			wantBody: "Anfrage bestätigt, Stammdaten übernommen", wantReason: "Passt so",
			wantRequest: userModels.ParentMessageRequestStatusDone,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newMasterDataFixture(t, testpkg.SetupTestDB(t))
			other := testpkg.CreateTestCoGuardianForStudent(t, f.db, f.chain.StudentID, "Klaus", "Zweitelternteil")
			var sharedWith []int64
			if tc.shared {
				sharedWith = []int64{other.AccountID}
			}
			row := f.insert(t, userModels.DataChangeTargetPerson, "first_name", `"Felix"`, `"Maximilian"`)
			_, err := f.decide(t, f.decisionsWithEmitter(t, staticShareVisibility{recipients: sharedWith}), masterdatarequests.DecideInput{
				RequestID: row.ID, Approve: true, Reason: "Passt so", ReviewedBy: f.chain.AccountID,
			})
			require.NoError(t, err)

			pill := f.requestStatusPill(t, other.AccountID)
			require.NotNil(t, pill, "the other guardian must hear that the child's data changed")
			assert.Equal(t, tc.wantBody, pill.Body)
			assert.Equal(t, tc.wantReason, pill.DecisionReason)
			assert.Equal(t, tc.wantRequest, pill.RequestStatus)
		})
	}
}
