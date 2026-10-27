package contracttest_test

import (
	"context"
	"encoding/json"

	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"

	timetable "github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"

	"testing"
	"time"

	"github.com/uptrace/bun"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Post-enrollment offering change requests (#1665). The service is exercised
// against the real test DB; the emitter is left nil (chat pills are covered by
// the messaging tests and are best-effort by contract).

// offeringChangeAdminContext makes the staff-side service calls explicit about
// their authority. TenantContext alone deliberately carries no JWT permissions.
func offeringChangeAdminContext(t *testing.T) context.Context {
	return context.WithValue(testpkg.Ctx(t), sessiontoken.CtxPermissions, []string{"admin:*"})
}

func newOfferingChangeServiceForTest(
	t *testing.T,
	env *decisionTestEnv,
) careplan.OfferingChangeCapability {
	return newOfferingChangeServiceForTestWithCatalog(t, env, nil)
}

// newOfferingChangeServiceForTestWithCatalog composes Care Plan's
// offering-change review (#3561) over the suite's settings; approvals apply
// through the booking materialization the decision service drives. A
// catalog intercepts the care-offering reads.
func newOfferingChangeServiceForTestWithCatalog(
	t *testing.T,
	env *decisionTestEnv,
	catalog careplanCompose.OfferingChangeCatalog,
) careplan.OfferingChangeCapability {
	t.Helper()
	env.settings.boolValues[enrollmentKeys.OfferingChangesEnabled] = true
	env.settings.boolValues[enrollmentKeys.ParentCourseRequestsEnabled] = true
	env.settings.stringValues[enrollmentKeys.OfferingChangesLeadDays] = "14"
	options := []api.OfferingChangeOption{
		// Mirrors newDecisionServiceForTest's fixed clock: the lead-day and
		// earliest-date assertions in this file compare against this date.
		api.WithOfferingChangeToday(func() calendar.Date { return calendar.NewDate(2026, 8, 24) }),
	}
	if catalog != nil {
		options = append(options, api.WithOfferingChangeCatalog(catalog))
	}
	return newOfferingChanges(t, env.db, env.settings, env.bookings, options...)
}

// offeringReviewQueueForTest is the staff review queue the office decides
// from, on the suite's fixed day.
func offeringReviewQueueForTest(t *testing.T, env *decisionTestEnv) careplan.OfferingReviewQuery {
	t.Helper()
	return newOfferingReviewQuery(t, env.db, func() calendar.Date { return calendar.NewDate(2026, 8, 24) })
}

// offeringChangeFixture sets up an approved child booked into oldOffering, plus
// a second offering it could switch to.
type offeringChangeFixture struct {
	requestID    int64
	childID      int64
	studentID    int64
	oldOffering  *offeringContract.CareOfferingRecord
	newOffering  *offeringContract.CareOfferingRecord
	oldGroupID   int64
	newGroupID   int64
	switchDate   calendar.Date
	pastSwitchAt calendar.Date
}

func setupOfferingChangeFixture(
	t *testing.T,
	env *decisionTestEnv,
	label string,
) *offeringChangeFixture {
	t.Helper()
	oldGroup := testpkg.CreateTestActivityGroup(t, env.db, "OfferChangeOld"+label)
	newGroup := testpkg.CreateTestActivityGroup(t, env.db, "OfferChangeNew"+label)
	oldOffering := createAdjustmentCareOfferingWith(t, env, "Bisher "+label, func(o *offeringContract.CareOfferingRecord) {
		o.ActivityGroupID = &oldGroup.ID
		o.SortOrder = 201
	})
	newOffering := createAdjustmentCareOfferingWith(t, env, "Neu "+label, func(o *offeringContract.CareOfferingRecord) {
		o.ActivityGroupID = &newGroup.ID
		o.SortOrder = 202
	})
	requestID, childID, studentID := submitApprovedAdjustmentChild(
		t, env, "offer-change-"+label+"@example.com", "OfferChange"+label,
		[]*offeringContract.CareOfferingRecord{oldOffering},
	)
	return &offeringChangeFixture{
		requestID:    requestID,
		childID:      childID,
		studentID:    studentID,
		oldOffering:  oldOffering,
		newOffering:  newOffering,
		oldGroupID:   oldGroup.ID,
		newGroupID:   newGroup.ID,
		switchDate:   calendar.Date(env.sourcePhase.ServiceStartDate).AddDays(150),
		pastSwitchAt: calendar.NewDate(2026, 8, 24).AddDays(-1),
	}
}

func TestOfferingChangeRequestService_Create_StoresPendingRequest(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Create")

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Note:          "Neuer Arbeitsbeginn",
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	assert.Equal(t, offeringContract.OfferingChangeStatusPending, row.Status)
	assert.Equal(t, fx.childID, row.RequestChildID)
	assert.Equal(t, fx.switchDate, calendar.Date(row.EffectiveFrom))
	require.NotNil(t, row.ParentNote)
	assert.Equal(t, "Neuer Arbeitsbeginn", *row.ParentNote)

	// The read view carries the request plus a diff naming both sides.
	view, err := svc.GetForStudent(ctx, fx.studentID)
	require.NoError(t, err)
	require.NotNil(t, view)
	labels := make([]string, 0, len(view.Diff))
	for _, entry := range view.Diff {
		labels = append(labels, entry.Label)
	}
	assert.Contains(t, labels, fx.oldOffering.Name, "dropped offering must appear in the diff")
	assert.Contains(t, labels, fx.newOffering.Name, "added offering must appear in the diff")
}

func TestCourseCatalogQueryBudget(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "CourseCatalogBudget")
	group := repositoryActivityGroup(t, ctx, env, "CourseCatalogBudget")
	group.Type = timetable.GroupTypeActivity
	require.NoError(t, timetableCompose.NativeGroupRecords(env.timetable.Timetable).Update(ctx, group))
	zero := 0
	course := createAdjustmentCareOfferingWith(t, env, "Kurs CourseCatalogBudget", func(o *offeringContract.CareOfferingRecord) {
		o.ActivityGroupID = &group.ID
		o.Capacity = &zero
		o.SortOrder = 203
		o.DaysOfWeekMode = offeringContract.DaysOfWeekModeFixed
		o.AvailableDays = []string{"mon"}
	})
	_, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.CreateCourseRequest, careplan.CreateCourseRequestInput{
		StudentID: fx.studentID, AccountID: env.creatorID, OfferingID: course.ID,
	})
	require.NoError(t, err)

	counter := testpkg.CaptureQueriesForContext(t, env.db)
	ctx = counter.Context(ctx)
	var catalog *careplan.CourseCatalog
	err = tenant.WithTenantTx(ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		var catalogErr error
		catalog, catalogErr = svc.CourseCatalog(txCtx, fx.studentID, env.creatorID)
		return catalogErr
	})
	require.NoError(t, err)
	var courseItem *careplan.CourseCatalogItem
	for i := range catalog.Items {
		if catalog.Items[i].OfferingID == course.ID {
			courseItem = &catalog.Items[i]
			break
		}
	}
	require.NotNil(t, courseItem)
	assert.True(t, courseItem.Waitlisted)
	t.Logf("course catalog issued %d queries", counter.Total())
	testpkg.AssertQueryBudget(t, "api.parent.child_courses", counter.Queries())
}

func TestOfferingChangeRequestService_Create_PayloadExcludesAutomaticOfferings(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "PayloadAuto")
	automatic := createAdjustmentCareOfferingWith(t, env, "Automatisch PayloadAuto", func(o *offeringContract.CareOfferingRecord) {
		o.AutoAddTriggerOfferingIDs = []int64{fx.newOffering.ID}
		o.SortOrder = 203
	})

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{{
			OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"},
		}},
	})
	require.NoError(t, err)

	offerings, ok := offeringChangePayload(t, row)["offerings"].([]any)
	require.True(t, ok)
	require.Len(t, offerings, 1, "the payload must record only the guardian's explicit choice")

	automatic.AutoAddTriggerOfferingIDs = nil
	require.NoError(t, newCareOfferingFixtureRecords(env.repos.CarePlan()).Update(ctx, automatic))
	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), decidesOfferingChange(svc), careplan.OfferingChangeDecisionInput{
		RequestID: row.ID, ReviewedBy: env.creatorID, Approve: true,
	}))
	links, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), env.reposEnrollment.Enrollment().RequestChildOfferingsAtDate, fx.childID, enrollmentTest.Date(fx.switchDate))
	require.NoError(t, err)
	for _, link := range links {
		assert.NotEqual(t, automatic.ID, link.CareOfferingID,
			"an offering whose automatic trigger was removed must not become a manual booking")
	}
}

func TestOfferingChangeRequestService_Create_StripsChangedCurrentAutomaticOffering(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "ChangedCurrentAuto")
	automatic := createAdjustmentCareOfferingWith(t, env, "Automatisch ChangedCurrentAuto", func(o *offeringContract.CareOfferingRecord) {
		o.SortOrder = 203
	})

	// This row represents an offering materialized from another selection. A
	// crafted request must not be able to change its days and persist it as a
	// manual booking.
	require.NoError(t, api.NewEnrollmentBookingFixtureWithin(testpkg.WithinCurrentTenant).InsertRequestChildOffering(ctx, &enrollmentTest.RequestChildOffering{
		RequestChildID:        fx.childID,
		CareOfferingID:        automatic.ID,
		SelectedDays:          []string{"mon"},
		AutomaticSelectedDays: []string{"mon"},
	}))

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
			{OfferingID: automatic.ID, SelectedDays: []string{"tue"}},
		},
	})
	require.NoError(t, err)

	offerings, ok := offeringChangePayload(t, row)["offerings"].([]any)
	require.True(t, ok)
	require.Len(t, offerings, 1, "automatic offerings must never enter the manual request payload")
	entry, ok := offerings[0].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, fx.newOffering.ID, entry["offering_id"])
}

func TestOfferingChangeRequestService_Create_RejectsSecondPendingRequest(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Second")

	_, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	_, err = testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.oldOffering.ID, SelectedDays: []string{"tue"}},
		},
	})
	require.ErrorIs(t, err, careplan.ErrOfferingChangeAlreadyPending)
}

func TestOfferingChangeRequestService_Create_UsesSelectionAtEffectiveDate(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "EffectiveSelection")

	first, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{{
			OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"},
		}},
	})
	require.NoError(t, err)
	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), decidesOfferingChange(svc), careplan.OfferingChangeDecisionInput{
		RequestID: first.ID, Approve: true, ReviewedBy: env.creatorID,
	}))

	// The first approved switch is already the booking at this later date.
	// Repeating it must be rejected instead of being compared with today's old
	// selection and creating a no-op pending request.
	_, err = svc.SubmitOfferingChange(ctx, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate.AddDays(10),
		Selections: []careplan.OfferingChangeSelection{{
			OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"},
		}},
	})
	require.ErrorIs(t, err, careplan.ErrOfferingChangeInvalid)
}

func TestOfferingChangeRequestService_Create_RejectsEffectiveDateInsideNoticePeriod(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Notice")

	_, err := svc.SubmitOfferingChange(ctx, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.pastSwitchAt,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.ErrorIs(t, err, careplan.ErrOfferingChangeInvalid)
}

func TestOfferingChangeRequestService_Create_RejectsOfferingOutsideThePhase(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Foreign")

	_, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID + 100_000, SelectedDays: []string{"mon"}},
		},
	})
	require.ErrorIs(t, err, careplan.ErrOfferingChangeInvalid)
}

func TestOfferingChangeRequestService_Create_RefusedWhenSchoolDisabledIt(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Off")
	env.settings.boolValues[enrollmentKeys.OfferingChangesEnabled] = false

	_, err := svc.SubmitOfferingChange(ctx, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.ErrorIs(t, err, careplan.ErrOfferingChangeDisabled)
}

func TestOfferingChangeRequestService_Decide_ApprovalAppliesTheDatedSwitch(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Approve")

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), decidesOfferingChange(svc), careplan.OfferingChangeDecisionInput{
		RequestID:  row.ID,
		Approve:    true,
		ReviewedBy: env.creatorID,
	}))

	decided, err := enrollmentTest.NewOfferingChangeRecords(env.repos.CarePlan()).FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, offeringContract.OfferingChangeStatusApproved, decided.Status)
	require.NotNil(t, decided.AppliedAt)

	// The old group is capped at the switch date instead of deleted, the new one
	// starts there — the whole point of the dated path.
	rows := listStudentEnrollmentRowsForDecisionTest(t, env, fx.studentID)
	byGroup := map[int64]timetable.StudentEnrollment{}
	for _, enrollment := range rows {
		byGroup[enrollment.ActivityGroupID] = enrollment
	}
	oldRow, ok := byGroup[fx.oldGroupID]
	require.True(t, ok, "the attended group must survive as history")
	require.NotNil(t, oldRow.ValidUntil)
	assert.Equal(t, string(fx.switchDate), *oldRow.ValidUntil)
	newRow, ok := byGroup[fx.newGroupID]
	require.True(t, ok, "the requested group must be materialized")
	assert.Equal(t, string(fx.switchDate), newRow.ValidFrom)

	// The queue is empty afterwards.
	pending, _, err := offeringReviewQueueForTest(t, env).ListPending(ctx, careplan.RequestQueueFilter{})
	require.NoError(t, err)
	assert.Empty(t, pending)
}

func TestOfferingChangeRequestService_Decide_RefusesApprovalAfterPlannedCareEnd(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "CareEnd")

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}}},
	})
	require.NoError(t, err)
	_, err = env.db.NewUpdate().TableExpr("users.student_school_memberships").
		Set("enrolled_until = ?", fx.switchDate.AddDays(-1)).Where("student_profile_id = ? AND deleted_at IS NULL", fx.studentID).Exec(ctx)
	require.NoError(t, err)

	err = testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), decidesOfferingChange(svc), careplan.OfferingChangeDecisionInput{RequestID: row.ID, Approve: true, ReviewedBy: env.creatorID})
	require.ErrorIs(t, err, careplan.ErrOfferingChangeInvalid)
}

func TestOfferingChangeRequestService_Decide_CapsRebookingAtPlannedCareEnd(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "CareEndCap")
	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}}},
	})
	require.NoError(t, err)
	_, err = env.db.NewUpdate().TableExpr("users.student_school_memberships").
		Set("enrolled_until = ?", fx.switchDate).Where("student_profile_id = ? AND deleted_at IS NULL", fx.studentID).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), decidesOfferingChange(svc), careplan.OfferingChangeDecisionInput{
		RequestID: row.ID, Approve: true, ReviewedBy: env.creatorID,
	}))
	exclusiveEnd := fx.switchDate.AddDays(1)
	links, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), env.reposEnrollment.Enrollment().RequestChildOfferingsAtDate, fx.childID, enrollmentTest.Date(fx.switchDate))
	require.NoError(t, err)
	require.NotEmpty(t, links)
	for _, link := range links {
		require.NotNil(t, link.ValidUntil)
		assert.False(t, enrollmentTest.Date(exclusiveEnd).Before(*link.ValidUntil), "source booking must not outlive the child care interval")
	}
	for _, enrollment := range listStudentEnrollmentRowsForDecisionTest(t, env, fx.studentID) {
		if enrollment.ActivityGroupID != fx.newGroupID {
			continue
		}
		require.NotNil(t, enrollment.ValidUntil)
		assert.False(t, calendar.Date(*enrollment.ValidUntil).After(exclusiveEnd), "derived booking must not be recreated past care end")
	}
}

func TestOfferingChangeRequestService_Decide_RebookingObsoletesWithdrawalWithoutGap(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "RebookWithdrawal")
	studentID := fx.studentID
	actorID := env.creatorID
	repo := env.reposPeople.CareWithdrawal
	require.NoError(t, api.SeedPendingCareWithdrawal(ctx, repo, &careplan.WithdrawalCompletion{
		StudentID: &studentID, FirstBookinglessDay: fx.switchDate,
		Trigger:               careplan.WithdrawalTriggerDirectSchool,
		WithdrawalConfirmedBy: &actorID, WithdrawalConfirmedRole: "admin",
		WithdrawalConfirmedAt: time.Now(),
	}))
	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}}},
	})
	require.NoError(t, err)

	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), decidesOfferingChange(svc), careplan.OfferingChangeDecisionInput{
		RequestID: row.ID, Approve: true, ReviewedBy: env.creatorID,
	}))
	pending, err := pendingWithdrawalForStudent(ctx, repo, fx.studentID)
	require.NoError(t, err)
	assert.Nil(t, pending, "approved parent rebooking must remove the stale complete-withdrawal task")
}

// A pending request outlives edits to its care period. When the period's
// service start moves past the requested date, an approval applies at the new
// period start — the queue has to name that date instead of today, or the
// office approves a switch believing it takes effect immediately.
func TestOfferingChangeRequestService_ListPending_ReportsDateClampedToThePhaseStart(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "QueueClamp")

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	// The period start moves behind the requested date while the request waits.
	phaseStart := calendar.NewDate(2026, 8, 24).AddDays(30)
	env.sourcePhase.ServiceStartDate = enrollmentTest.Date(phaseStart)
	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), env.reposEnrollment.Enrollment().UpdatePhase, ownerPhaseForTest(env.sourcePhase)))
	require.NoError(t, env.repos.CarePlan().UpdateOfferingChangeEffectiveFrom(ctx, row.ID, string(offeringContract.OfferingChangeDate(fx.pastSwitchAt))))

	pending, _, err := offeringReviewQueueForTest(t, env).ListPending(ctx, careplan.RequestQueueFilter{})
	require.NoError(t, err)
	var queued *careplan.OfferingReviewItem
	for _, view := range pending {
		if view.Request != nil && view.Request.ID == row.ID {
			queued = view
		}
	}
	require.NotNil(t, queued, "the pending request must stay in the queue")
	assert.Equal(t, phaseStart, calendar.Date(queued.Request.EffectiveFrom),
		"the queue must show the date an approval would actually apply")
	assert.Equal(t, fx.pastSwitchAt.String(), queued.RequestedEffectiveFrom,
		"the queue must keep the date the family asked for, so the card can name it")
}

// Ein Datum, das die OGS selbst gewählt hat, überschreibt das der Eltern in der
// Anfragezeile. Ohne die Zeile im Änderungsprotokoll bliebe von deren Wunsch
// nichts übrig (#2484).
func TestOfferingChangeRequestService_Decide_LogsTheDateTheFamilyAskedFor(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "LogWish")

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	confirmed := fx.switchDate.AddDays(7)
	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), decidesOfferingChange(svc), careplan.OfferingChangeDecisionInput{
		RequestID:     row.ID,
		Approve:       true,
		ReviewedBy:    env.creatorID,
		EffectiveFrom: &confirmed,
	}))

	adjustments, err := env.decision.ListOfferingAdjustments(ctx, fx.requestID, fx.childID)
	require.NoError(t, err)
	require.Len(t, adjustments, 1)
	assert.Contains(t, adjustments[0].Reason,
		"gültig ab "+confirmed.Format("02.01.2006"))
	assert.Contains(t, adjustments[0].Reason,
		"Wunschdatum der Eltern war "+fx.switchDate.Format("02.01.2006"))
}

// Bleibt es beim Wunsch der Eltern, hat das Protokoll nichts zu ergänzen.
func TestOfferingChangeRequestService_Decide_LogsNoWishWhenTheDateIsKept(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "LogNoWish")

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), decidesOfferingChange(svc), careplan.OfferingChangeDecisionInput{
		RequestID:     row.ID,
		Approve:       true,
		ReviewedBy:    env.creatorID,
		EffectiveFrom: &fx.switchDate,
	}))

	adjustments, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), env.decision.ListOfferingAdjustments, fx.requestID, fx.childID)
	require.NoError(t, err)
	require.Len(t, adjustments, 1)
	assert.NotContains(t, adjustments[0].Reason, "Wunschdatum der Eltern")
}

func TestOfferingChangeRequestService_Decide_ApprovalRejectsWhenCareOfferingsAreDisabled(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "DisabledApproval")

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	env.settings.boolValues[enrollmentKeys.CareOfferingsEnabled] = false
	err = testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), decidesOfferingChange(svc), careplan.OfferingChangeDecisionInput{
		RequestID:  row.ID,
		Approve:    true,
		ReviewedBy: env.creatorID,
	})
	require.ErrorIs(t, err, careplan.ErrCareOfferingsDisabled)

	pending, err := enrollmentTest.NewOfferingChangeRecords(env.repos.CarePlan()).FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, offeringContract.OfferingChangeStatusPending, pending.Status)
}

func TestOfferingChangeRequestService_Decide_RejectionNeedsAReasonAndChangesNothing(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Reject")

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	err = decideOfferingChange(ctx, svc, careplan.OfferingChangeDecisionInput{
		RequestID:  row.ID,
		Approve:    false,
		ReviewedBy: env.creatorID,
	})
	require.ErrorIs(t, err, careplan.ErrOfferingChangeInvalid)

	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), decidesOfferingChange(svc), careplan.OfferingChangeDecisionInput{
		RequestID:  row.ID,
		Approve:    false,
		Reason:     "Kein Platz in der Gruppe",
		ReviewedBy: env.creatorID,
	}))

	decided, err := enrollmentTest.NewOfferingChangeRecords(env.repos.CarePlan()).FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, offeringContract.OfferingChangeStatusRejected, decided.Status)
	assert.Nil(t, decided.AppliedAt, "a rejection must not stamp applied_at")

	// The booking is untouched: still exactly the original group.
	rows := listStudentEnrollmentRowsForDecisionTest(t, env, fx.studentID)
	require.Len(t, rows, 1)
	assert.Equal(t, fx.oldGroupID, rows[0].ActivityGroupID)
}

func TestOfferingChangeRequestService_Decide_RefusesApprovalWhenOfferingIsFull(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Full")

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	// The offering fills up between submission and decision.
	zeroCapacity := 0
	fx.newOffering.Capacity = &zeroCapacity
	require.NoError(t, newCareOfferingFixtureRecords(env.repos.CarePlan()).Update(ctx, fx.newOffering))

	err = testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), decidesOfferingChange(svc), careplan.OfferingChangeDecisionInput{
		RequestID:  row.ID,
		Approve:    true,
		ReviewedBy: env.creatorID,
	})
	require.ErrorIs(t, err, careplan.ErrOfferingChangeCapacityFull)

	// Still pending, so the office can talk to the family instead of finding a
	// request marked done that never applied.
	stillPending, err := enrollmentTest.NewOfferingChangeRecords(env.repos.CarePlan()).FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, offeringContract.OfferingChangeStatusPending, stillPending.Status)
	rows := listStudentEnrollmentRowsForDecisionTest(t, env, fx.studentID)
	require.Len(t, rows, 1, "a refused approval must not change the booking")
}

func TestOfferingChangeRequestService_Decide_AllowsLeavingOverCapacityOffering(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "LeaveFull")

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	// Capacity can be reduced after enrollment. That must never trap an
	// already-booked child in the now-over-capacity offering.
	zeroCapacity := 0
	fx.oldOffering.Capacity = &zeroCapacity
	require.NoError(t, newCareOfferingFixtureRecords(env.repos.CarePlan()).Update(ctx, fx.oldOffering))

	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), decidesOfferingChange(svc), careplan.OfferingChangeDecisionInput{
		RequestID:  row.ID,
		Approve:    true,
		ReviewedBy: env.creatorID,
	}))

	decided, err := enrollmentTest.NewOfferingChangeRecords(env.repos.CarePlan()).FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, offeringContract.OfferingChangeStatusApproved, decided.Status)
}

func TestOfferingChangeRequestService_Decide_AllowsRetainingOverCapacityOffering(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "KeepFull")

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.oldOffering.ID, SelectedDays: []string{"mon"}},
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	zeroCapacity := 0
	fx.oldOffering.Capacity = &zeroCapacity
	require.NoError(t, newCareOfferingFixtureRecords(env.repos.CarePlan()).Update(ctx, fx.oldOffering))

	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), decidesOfferingChange(svc), careplan.OfferingChangeDecisionInput{
		RequestID: row.ID, Approve: true, ReviewedBy: env.creatorID,
	}))

	decided, err := enrollmentTest.NewOfferingChangeRecords(env.repos.CarePlan()).FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, offeringContract.OfferingChangeStatusApproved, decided.Status)
}

func TestOfferingChangeRequestService_Catalog_MarksCurrentBookingAndCapacity(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Catalog")

	capacity := 3
	fx.newOffering.Capacity = &capacity
	require.NoError(t, newCareOfferingFixtureRecords(env.repos.CarePlan()).Update(ctx, fx.newOffering))

	catalog, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.Catalog, fx.studentID)
	require.NoError(t, err)
	assert.Equal(t, calendar.Date(env.sourcePhase.ServiceEndDate), catalog.LatestEffectiveFrom)
	assert.False(t, catalog.EarliestEffectiveFrom.Before(calendar.NewDate(2026, 8, 24)),
		"a switch must not be offered for a date already gone")

	byID := map[int64]careplan.OfferingChangeCatalogItem{}
	for _, item := range catalog.Items {
		byID[item.OfferingID] = item
	}
	current, ok := byID[fx.oldOffering.ID]
	require.True(t, ok)
	assert.True(t, current.Selected, "the current booking must open prefilled")
	assert.Equal(t, []string{"mon"}, current.SelectedDays)

	other, ok := byID[fx.newOffering.ID]
	require.True(t, ok)
	assert.False(t, other.Selected)
	require.NotNil(t, other.FreeSlots)
	assert.Equal(t, 3, *other.FreeSlots)
}

func TestOfferingChangeRequestService_GetForStudent_ReportsRecentDecision(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Decision")

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), decidesOfferingChange(svc), careplan.OfferingChangeDecisionInput{
		RequestID:  row.ID,
		Approve:    false,
		Reason:     "Gruppe ist voll",
		ReviewedBy: env.creatorID,
	}))

	view, err := svc.GetForStudent(ctx, fx.studentID)
	require.NoError(t, err)
	require.NotNil(t, view, "a decided request must still be reported")
	assert.Nil(t, view.Request, "no request is open anymore")
	require.NotNil(t, view.LastDecision)
	assert.Equal(t, offeringContract.OfferingChangeStatusRejected, view.LastDecision.Status)
	assert.Equal(t, "Gruppe ist voll", view.LastDecision.Reason)
	assert.Equal(t, fx.switchDate, view.LastDecision.EffectiveFrom)
	// The request itself is read back from the payload: a rejection is only
	// understandable next to what was asked for.
	require.Len(t, view.LastDecision.Requested, 1)
	assert.Equal(t, fx.newOffering.Name, view.LastDecision.Requested[0].Name)
	assert.Equal(t, []string{"mon"}, view.LastDecision.Requested[0].Days)

	// Aged past the recency window it drops out, so the card does not turn into
	// a history list.
	_, err = env.db.NewRaw(`
		UPDATE enrollment.offering_change_requests
		SET reviewed_at = NOW() - INTERVAL '20 days'
		WHERE id = ?
	`, row.ID).Exec(ctx)
	require.NoError(t, err)

	view, err = svc.GetForStudent(ctx, fx.studentID)
	require.NoError(t, err)
	assert.Nil(t, view, "an old decision is no longer reported")
}

func TestOfferingChangeRequestService_Decide_AppliesTheConfirmedDate(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "ConfirmDate")

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	confirmed := fx.switchDate.AddDays(7)
	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), decidesOfferingChange(svc), careplan.OfferingChangeDecisionInput{
		RequestID:     row.ID,
		Approve:       true,
		ReviewedBy:    env.creatorID,
		EffectiveFrom: &confirmed,
	}))

	decided, err := enrollmentTest.NewOfferingChangeRecords(env.repos.CarePlan()).FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, offeringContract.OfferingChangeStatusApproved, decided.Status)
	assert.Equal(t, confirmed, calendar.Date(decided.EffectiveFrom),
		"the request must record the date the office confirmed, not the parents' wish")

	rows := listStudentEnrollmentRowsForDecisionTest(t, env, fx.studentID)
	byGroup := map[int64]timetable.StudentEnrollment{}
	for _, enrollment := range rows {
		byGroup[enrollment.ActivityGroupID] = enrollment
	}
	oldRow, ok := byGroup[fx.oldGroupID]
	require.True(t, ok, "the attended group must survive as history")
	require.NotNil(t, oldRow.ValidUntil)
	assert.Equal(t, string(confirmed), *oldRow.ValidUntil)
	newRow, ok := byGroup[fx.newGroupID]
	require.True(t, ok, "the requested group must be materialized")
	assert.Equal(t, string(confirmed), newRow.ValidFrom)
}

// A confirmed date before today would rewrite weeks that already happened.
// Refused outright: a silent shift to today would hand the office a different
// switch than the one it confirmed.
func TestOfferingChangeRequestService_Decide_RefusesAConfirmedDateBeforeToday(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "PastDate")

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	confirmed := calendar.NewDate(2026, 8, 24).AddDays(-1)
	err = testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), decidesOfferingChange(svc), careplan.OfferingChangeDecisionInput{
		RequestID:     row.ID,
		Approve:       true,
		ReviewedBy:    env.creatorID,
		EffectiveFrom: &confirmed,
	})
	require.ErrorIs(t, err, careplan.ErrOfferingChangeDateOutOfRange)

	pending, err := enrollmentTest.NewOfferingChangeRecords(env.repos.CarePlan()).FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, offeringContract.OfferingChangeStatusPending, pending.Status,
		"a refused date leaves the request open")
}

// The switch cannot start after the care period it belongs to has ended.
func TestOfferingChangeRequestService_Decide_RefusesAConfirmedDateAfterTheCarePeriod(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "AfterPeriod")

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	confirmed := calendar.Date(env.sourcePhase.ServiceEndDate).AddDays(1)
	err = testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), decidesOfferingChange(svc), careplan.OfferingChangeDecisionInput{
		RequestID:     row.ID,
		Approve:       true,
		ReviewedBy:    env.creatorID,
		EffectiveFrom: &confirmed,
	})
	require.ErrorIs(t, err, careplan.ErrOfferingChangeDateOutOfRange)

	pending, err := enrollmentTest.NewOfferingChangeRecords(env.repos.CarePlan()).FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, offeringContract.OfferingChangeStatusPending, pending.Status)
}

// Nothing takes effect on a rejection, so a date on one is a caller mistake.
func TestOfferingChangeRequestService_Decide_RefusesAConfirmedDateOnARejection(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "RejectDate")

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	confirmed := fx.switchDate
	err = testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), decidesOfferingChange(svc), careplan.OfferingChangeDecisionInput{
		RequestID:     row.ID,
		Approve:       false,
		Reason:        "Kein Platz",
		ReviewedBy:    env.creatorID,
		EffectiveFrom: &confirmed,
	})
	require.ErrorIs(t, err, careplan.ErrOfferingChangeInvalid)
}

// The card bounds its date field from the queue, so staff cannot pick a date
// the approval would refuse.
func TestOfferingChangeRequestService_ListPending_ReportsTheSelectableDateRange(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "DateRange")

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	pending, _, err := offeringReviewQueueForTest(t, env).ListPending(ctx, careplan.RequestQueueFilter{})
	require.NoError(t, err)
	var queued *careplan.OfferingReviewItem
	for _, view := range pending {
		if view.Request != nil && view.Request.ID == row.ID {
			queued = view
		}
	}
	require.NotNil(t, queued)
	earliest := calendar.Date(env.sourcePhase.ServiceStartDate)
	if today := decisionTestToday; earliest.Before(today) {
		earliest = today
	}
	assert.Equal(t, earliest.String(), queued.EarliestEffectiveFrom,
		"the office may switch from today, but never before the care period starts")
	assert.Equal(t, calendar.Date(env.sourcePhase.ServiceEndDate).String(), queued.LatestEffectiveFrom)
}

// The preview is what the office decides from. A date the approval would refuse
// must fail here too, or the card offers a switch that cannot happen.
func TestOfferingChangeRequestService_PreviewDecision_RefusesADateOutOfRange(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "PreviewRange")

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	confirmed := calendar.NewDate(2026, 8, 24).AddDays(-1)
	_, err = testpkg.TenantQuery3(t, ctx, testpkg.SetupTestDB(t), svc.PreviewDecision, row.ID, nil, &confirmed)
	require.ErrorIs(t, err, careplan.ErrOfferingChangeDateOutOfRange)
}

type planningOccurrenceOpts struct {
	status       string
	spontaneous  bool
	materialized bool
	unplanned    bool
	notScheduled bool
}

func createPlanningOccurrence(
	t *testing.T,
	env *decisionTestEnv,
	groupID, studentID, roomID, periodID int64,
	date calendar.Date,
	title string,
	opts planningOccurrenceOpts,
) {
	t.Helper()
	status := opts.status
	if status == "" {
		status = timetable.InstanceStatusPlanned
	}
	var calendarPeriodID *int64
	if opts.materialized {
		calendarPeriodID = &periodID
	}
	instance := testpkg.CreateTestActivityInstance(t, env.db, date, roomID, testpkg.ActivityInstanceOpts{
		ActivityGroupID:  &groupID,
		CalendarPeriodID: calendarPeriodID,
		Title:            title,
		Status:           status,
		IsSpontaneous:    opts.spontaneous,
	})
	attendance := testpkg.CreateTestInstanceStudent(t, env.db, instance.ID, studentID, "", testpkg.InstanceStudentOpts{
		NotScheduled: opts.notScheduled,
	})
	if opts.unplanned {
		stored, err := testpkg.TenantQuery1(t, testpkg.
			Ctx(
				t), env.db,
			env.timetable.NativeParticipants().
				FindByID, attendance.ID)

		require.NoError(t, err)
		stored.IsUnplanned = true
		require.NoError(t, env.timetable.NativeParticipants().Update(testpkg.Ctx(t), stored))
	}
}

func TestOfferingChangeRequestService_PreviewDecision_ReportsOnlyUncoveredManualPlanning(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	env.settings.boolValues[enrollmentKeys.BookingsAuthoritative] = true
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "PreviewManualPlanning")
	fx.newOffering.DaysOfWeekMode = offeringContract.DaysOfWeekModeFixed
	fx.newOffering.AvailableDays = []string{"mon"}
	require.NoError(t, newCareOfferingFixtureRecords(env.repos.CarePlan()).Update(ctx, fx.newOffering))
	fx.oldOffering.DaysOfWeekMode = offeringContract.DaysOfWeekModeFixed
	fx.oldOffering.AvailableDays = []string{"mon", "wed", "thu"}
	fx.oldOffering.CountsAsCare = true
	fx.oldOffering.CountsAsCareSet = true
	require.NoError(t, newCareOfferingFixtureRecords(env.repos.CarePlan()).Update(ctx, fx.oldOffering))

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID},
		},
	})
	require.NoError(t, err)

	manualGroup := repositoryActivityGroup(t, ctx, env, "ManualPlanningPreview")
	manualGroup.Type = timetable.GroupTypeCare
	manualGroup.IsTemplate = true
	manualGroup.TargetGroupType = timetable.TargetGroupTypeOffering
	require.NoError(t, timetableCompose.NativeGroupRecords(env.timetable.Timetable).Update(ctx, manualGroup))

	sourcedGroup := repositoryActivityGroup(t, ctx, env, "OfferingPlanningPreview")
	sourcedGroup.Type = timetable.GroupTypeCare
	sourcedGroup.IsTemplate = true
	sourcedGroup.TargetGroupType = timetable.TargetGroupTypeOffering
	sourcedGroup.SourceCareOfferingIDs = []int64{fx.newOffering.ID}
	require.NoError(t, timetableCompose.NativeGroupRecords(env.timetable.Timetable).Update(ctx, sourcedGroup))

	legacySourcedGroup, err := timetableCompose.NativeGroupRecords(env.timetable.Timetable).FindByID(ctx, fx.oldGroupID)
	require.NoError(t, err)
	legacySourcedGroup.Type = timetable.GroupTypeCare
	legacySourcedGroup.IsTemplate = true
	legacySourcedGroup.TargetGroupType = timetable.TargetGroupTypeNone
	require.NoError(t, timetableCompose.NativeGroupRecords(env.timetable.Timetable).Update(ctx, legacySourcedGroup))
	parentChoiceGroup := repositoryActivityGroup(t, ctx, env, "EmptyParentChoicePlanning")
	parentChoiceGroup.Type = timetable.GroupTypeCare
	parentChoiceGroup.IsTemplate = true
	require.NoError(t, timetableCompose.NativeGroupRecords(env.timetable.Timetable).Update(ctx, parentChoiceGroup))
	parentChoiceOffering := createAdjustmentCareOfferingWith(t, env, "Leere Tagesauswahl", func(o *offeringContract.CareOfferingRecord) {
		o.ActivityGroupID = &parentChoiceGroup.ID
		o.CountsAsCare, o.CountsAsCareSet = true, true
	})
	require.NoError(t, api.NewEnrollmentBookingFixtureWithin(testpkg.WithinCurrentTenant).InsertRequestChildOffering(ctx, &enrollmentTest.RequestChildOffering{
		RequestChildID: fx.childID,
		CareOfferingID: parentChoiceOffering.ID,
		SelectedDays:   []string{},
	}))

	period := testpkg.CreateTestCalendarPeriod(
		t, env.db, "Manual planning preview "+t.Name(), calendar.Date(env.sourcePhase.ServiceStartDate), calendar.Date(env.sourcePhase.ServiceEndDate),
	)
	room := testpkg.CreateTestRoom(t, env.db, "Manual planning preview")
	dateFor := func(weekday time.Weekday) calendar.Date {
		date := fx.switchDate
		for date.Weekday() != weekday {
			date = date.AddDays(1)
		}
		return date
	}
	_, err = env.db.NewRaw(`
		UPDATE enrollment.care_offering_bookings
		SET valid_until = ?
		WHERE tenant_id = ? AND request_child_id = ? AND care_offering_id = ?
	`, dateFor(time.Thursday), testpkg.Tenant(t), fx.childID, fx.oldOffering.ID).Exec(ctx)
	require.NoError(t, err)
	materialized := planningOccurrenceOpts{materialized: true}
	createPlanningOccurrence(t, env, manualGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Monday), "Montags gedeckt", materialized)
	createPlanningOccurrence(t, env, manualGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Tuesday), "Dienstags ohne Buchung", materialized)
	createPlanningOccurrence(t, env, manualGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Tuesday).AddDays(7), "Dienstags erneut", materialized)
	createPlanningOccurrence(t, env, manualGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Wednesday), "Mittwochs ohne Buchung", materialized)
	createPlanningOccurrence(t, env, manualGroup.ID, fx.studentID, room.ID, period.ID,
		fx.switchDate.AddDays(-1), "Vor dem Wechsel", materialized)
	createPlanningOccurrence(t, env, sourcedGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Tuesday), "Wird automatisch abgeglichen", materialized)
	createPlanningOccurrence(t, env, legacySourcedGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Monday), "Von alter Buchung gedeckt", materialized)
	createPlanningOccurrence(t, env, legacySourcedGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Tuesday), "Alte Angebots-Verknüpfung", materialized)
	createPlanningOccurrence(t, env, legacySourcedGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Wednesday), "Abgelaufene Angebots-Verknüpfung", materialized)
	createPlanningOccurrence(t, env, legacySourcedGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Thursday), "Nicht zählendes Angebot", materialized)
	createPlanningOccurrence(t, env, parentChoiceGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Tuesday), "Leere Tagesauswahl", materialized)
	createPlanningOccurrence(t, env, manualGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Friday), "Nicht gebucht", planningOccurrenceOpts{materialized: true, notScheduled: true})
	createPlanningOccurrence(t, env, manualGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Thursday), "Spontan", planningOccurrenceOpts{materialized: true, spontaneous: true})
	createPlanningOccurrence(t, env, manualGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Thursday).AddDays(7), "Nicht materialisiert", planningOccurrenceOpts{})
	createPlanningOccurrence(t, env, manualGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Thursday).AddDays(14), "Abgesagt", planningOccurrenceOpts{
			materialized: true,
			status:       timetable.InstanceStatusCancelled,
		})
	createPlanningOccurrence(t, env, manualGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Thursday).AddDays(21), "Ungeplant", planningOccurrenceOpts{materialized: true, unplanned: true})

	preview, err := testpkg.TenantQuery3(t, ctx, testpkg.SetupTestDB(t), svc.PreviewDecision, row.ID, nil, nil)
	require.NoError(t, err)
	require.Len(t, preview.ManualPlanningConflicts, 3)
	conflictsByGroupID := make(map[int64]careplan.ManualPlanningConflict, len(preview.ManualPlanningConflicts))
	for _, item := range preview.ManualPlanningConflicts {
		conflictsByGroupID[item.ActivityGroupID] = item
	}
	conflict := conflictsByGroupID[manualGroup.ID]
	assert.Equal(t, manualGroup.ID, conflict.ActivityGroupID)
	assert.Equal(t, manualGroup.Name, conflict.ActivityGroupName)
	assert.Equal(t, []string{"tue", "wed"}, conflict.Days)
	assert.Equal(t, dateFor(time.Tuesday), conflict.FirstDate)
	assert.Equal(t, 3, conflict.OccurrenceCount)
	legacyConflict := conflictsByGroupID[legacySourcedGroup.ID]
	assert.Equal(t, legacySourcedGroup.ID, legacyConflict.ActivityGroupID)
	assert.Equal(t, []string{"tue", "wed", "thu"}, legacyConflict.Days)
	assert.Equal(t, dateFor(time.Tuesday), legacyConflict.FirstDate)
	assert.Equal(t, 3, legacyConflict.OccurrenceCount)
	parentChoiceConflict := conflictsByGroupID[parentChoiceGroup.ID]
	assert.Equal(t, parentChoiceGroup.ID, parentChoiceConflict.ActivityGroupID)
	assert.Equal(t, []string{"tue"}, parentChoiceConflict.Days)
	assert.Equal(t, dateFor(time.Tuesday), parentChoiceConflict.FirstDate)
	assert.Equal(t, 1, parentChoiceConflict.OccurrenceCount)
	assert.True(t, preview.ArrivalExpectationsFollowBookings)
	laterEffectiveFrom := dateFor(time.Thursday).AddDays(22)
	laterPreview, err := svc.PreviewDecision(ctx, row.ID, nil, &laterEffectiveFrom)
	require.NoError(t, err)
	assert.Empty(t, laterPreview.ManualPlanningConflicts,
		"the chosen effective date must bound the planning consequences")
	env.settings.boolValues[enrollmentKeys.BookingsAuthoritative] = false
	preview, err = svc.PreviewDecision(ctx, row.ID, nil, nil)
	require.NoError(t, err)
	assert.False(t, preview.ArrivalExpectationsFollowBookings)

	otherTenantCtx := testpkg.TenantContext(testpkg.Tenant(t) + 1)
	foreignRows, err := testpkg.TenantQuery3(t, otherTenantCtx, testpkg.SetupTestDB(t), (owners.PlanningQuery{DB: testpkg.NewDatabase(t, env.db)}).ListManualPlanningOccurrences, fx.studentID, fx.switchDate.String(), string(env.sourcePhase.ServiceEndDate))
	require.NoError(t, err)
	assert.Empty(t, foreignRows, "the projection must not leak planning rows across tenants")
}

func TestOfferingChangeRequestService_PreviewDecision_ReportsManualPlanningForFullWithdrawal(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	env.settings.boolValues[enrollmentKeys.BookingsAuthoritative] = false
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "PreviewFullWithdrawal")
	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
	})
	require.NoError(t, err)

	manualGroup := repositoryActivityGroup(t, ctx, env, "ManualFullWithdrawal")
	manualGroup.Type, manualGroup.IsTemplate = timetable.GroupTypeCare, true
	require.NoError(t, timetableCompose.NativeGroupRecords(env.timetable.Timetable).Update(ctx, manualGroup))
	period := testpkg.CreateTestCalendarPeriod(
		t, env.db, "Manual full withdrawal "+t.Name(), calendar.Date(env.sourcePhase.ServiceStartDate), calendar.Date(env.sourcePhase.ServiceEndDate),
	)
	room := testpkg.CreateTestRoom(t, env.db, "Manual full withdrawal")
	createPlanningOccurrence(t, env, manualGroup.ID, fx.studentID, room.ID, period.ID,
		fx.switchDate, "Bleibt manuell geplant", planningOccurrenceOpts{materialized: true})

	preview, err := testpkg.TenantQuery3(t, ctx, testpkg.SetupTestDB(t), svc.PreviewDecision, row.ID, nil, nil)
	require.NoError(t, err)
	require.Len(t, preview.ManualPlanningConflicts, 1)
	assert.Equal(t, manualGroup.ID, preview.ManualPlanningConflicts[0].ActivityGroupID)
	assert.Equal(t, 1, preview.ManualPlanningConflicts[0].OccurrenceCount)
}

// The confirmed date is checked against the same capacity the parents' date
// would be: the office must not be able to move a switch onto a day the
// offering has no room on.
func TestOfferingChangeRequestService_Decide_RefusesAConfirmedDateWithoutCapacity(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "ConfirmedFull")

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	zeroCapacity := 0
	fx.newOffering.Capacity = &zeroCapacity
	require.NoError(t, newCareOfferingFixtureRecords(env.repos.CarePlan()).Update(ctx, fx.newOffering))

	confirmed := fx.switchDate.AddDays(7)
	err = testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), decidesOfferingChange(svc), careplan.OfferingChangeDecisionInput{
		RequestID:     row.ID,
		Approve:       true,
		ReviewedBy:    env.creatorID,
		EffectiveFrom: &confirmed,
	})
	require.ErrorIs(t, err, careplan.ErrOfferingChangeCapacityFull)

	stillPending, err := enrollmentTest.NewOfferingChangeRecords(env.repos.CarePlan()).FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, offeringContract.OfferingChangeStatusPending, stillPending.Status)
	rows := listStudentEnrollmentRowsForDecisionTest(t, env, fx.studentID)
	require.Len(t, rows, 1, "a refused approval must not change the booking")
}

// A care period that starts in the future is the lower bound, not today: a
// switch cannot begin before the period it belongs to.
func TestOfferingChangeRequestService_Decide_RefusesAConfirmedDateBeforeTheCarePeriod(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "BeforePeriod")

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	// The period start moves into the future while the request waits.
	phaseStart := calendar.NewDate(2026, 8, 24).AddDays(30)
	env.sourcePhase.ServiceStartDate = enrollmentTest.Date(phaseStart)
	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), env.reposEnrollment.Enrollment().UpdatePhase, ownerPhaseForTest(env.sourcePhase)))

	// Tomorrow is after today but still before the period begins.
	confirmed := calendar.NewDate(2026, 8, 24).AddDays(1)
	err = decideOfferingChange(ctx, svc, careplan.OfferingChangeDecisionInput{
		RequestID:     row.ID,
		Approve:       true,
		ReviewedBy:    env.creatorID,
		EffectiveFrom: &confirmed,
	})
	require.ErrorIs(t, err, careplan.ErrOfferingChangeDateOutOfRange)

	stillPending, err := enrollmentTest.NewOfferingChangeRecords(env.repos.CarePlan()).FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, offeringContract.OfferingChangeStatusPending, stillPending.Status)
}

func TestManualPlanningReaderRejectsInvalidCalendarWindows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Planning", "Validation", "2a")
	reader := owners.PlanningQuery{}
	for _, tc := range []struct {
		name, from, to string
	}{
		{"missing start", "", "2026-09-30"},
		{"missing end", "2026-09-01", ""},
		{"invalid day", "2026-02-30", "2026-09-30"},
		{"timestamp instead of date", "2026-09-01T00:00:00Z", "2026-09-30"},
		{"reversed window", "2026-09-30", "2026-09-01"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := reader.ListManualPlanningOccurrences(testpkg.Ctx(t), student.ID, tc.from, tc.to)
			require.EqualError(t, err, "valid planning date range is required")
			assert.Nil(t, rows)
		})
	}
	rows, err := reader.ListManualPlanningOccurrences(testpkg.Ctx(t), 0, "2026-09-01", "2026-09-30")
	require.EqualError(t, err, "student id must be positive")
	assert.Nil(t, rows)
}

// offeringChangePayload decodes the stored payload of a request row.
func offeringChangePayload(t *testing.T, row *careplan.OfferingChangeRequest) map[string]any {
	t.Helper()
	var payload map[string]any
	require.NoError(t, json.Unmarshal(row.Payload, &payload))
	return payload
}

// repositoryActivityGroup creates an activity fixture and reads it back as the
// owner row the activity repository updates.
func repositoryActivityGroup(t *testing.T, ctx context.Context, env *decisionTestEnv, name string) *timetable.Group {
	t.Helper()
	group, err := timetableCompose.NativeGroupRecords(env.timetable.Timetable).FindByID(ctx, testpkg.CreateTestActivityGroup(t, env.db, name).ID)
	require.NoError(t, err)
	return group
}
