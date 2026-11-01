package enrollmenthttp_test

import (
	"context"
	"fmt"
	"testing"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"

	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	"github.com/moto-nrw/project-phoenix/modules/careplan"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestEnrollmentListChildOfferingsQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	smallID := createBudgetRequest(t, env.rolloverTestEnv, env.sourcePhase.ID, "child-small", 3, enrollmentNative.ChildStatusSubmitted, nil)
	largeID := createBudgetRequest(t, env.rolloverTestEnv, env.sourcePhase.ID, "child-large", 8, enrollmentNative.ChildStatusSubmitted, nil)
	counter := testpkg.CaptureQueries(t, env.db)
	run := func(requestID int64) []string {
		var queries []string
		err := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			counter.Reset()
			_, err := env.decision.ListChildOfferings(txCtx, requestID)
			queries = counter.Operation("SELECT")
			return err
		})
		require.NoError(t, err)
		return queries
	}
	small, large := run(smallID), run(largeID)
	t.Logf("query budget: 3 children → %d reads, 8 children → %d reads", len(small), len(large))
	assert.Equal(t, len(small), len(large))
	querybudgets.AssertQueryBudget(t, "services.enrollment.list_child_offerings.reads", large)
}

func TestEnrollmentOfferingSourceOptionsQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	for i := range 3 {
		createSourceOffering(t, env, fmt.Sprintf("Budget small %d", i), nil)
	}
	lister := env.bookings.(careplan.OfferingSourceEditor)
	counter := testpkg.CaptureQueries(t, env.db)
	run := func() []string {
		var queries []string
		err := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			counter.Reset()
			_, err := lister.ListOfferingSourceOptions(txCtx, nil)
			queries = counter.Operation("SELECT")
			return err
		})
		require.NoError(t, err)
		return queries
	}
	small := run()
	for i := 3; i < 8; i++ {
		createSourceOffering(t, env, fmt.Sprintf("Budget large %d", i), nil)
	}
	large := run()
	t.Logf("query budget: 3 offerings → %d reads, 8 offerings → %d reads", len(small), len(large))
	assert.Equal(t, len(small), len(large))
	querybudgets.AssertQueryBudget(t, "services.enrollment.offering_source_options.reads", large)
}

func TestTemplateRosterMaintenanceFeedsQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	period := offeringSourcePeriod(t, env)
	source := createSourceOffering(t, env, "Teilnehmerpflege Budget", nil)
	lister := env.bookings.(careplan.OfferingSourceEditor)
	queries := make([]careplan.TemplateRosterFeedQuery, 0, 8)
	addTemplates := func(count int) {
		for range count {
			template := createSourcedTemplate(t, env, "Teilnehmerpflege Budget", source.ID, nil, period)
			queries = append(queries, careplan.TemplateRosterFeedQuery{
				TemplateID:            template.ID,
				CalendarPeriodID:      &period.ID,
				SourceCareOfferingIDs: []int64{source.ID},
			})
		}
	}
	counter := testpkg.CaptureQueries(t, env.db)
	run := func() []string {
		var reads []string
		err := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			counter.Reset()
			_, err := lister.TemplateRosterMaintenanceFeeds(txCtx, queries)
			reads = counter.Operation("SELECT")
			return err
		})
		require.NoError(t, err)
		return reads
	}

	addTemplates(3)
	small := run()
	addTemplates(5)
	large := run()

	t.Logf("query budget: 3 templates → %d reads, 8 templates → %d reads", len(small), len(large))
	assert.Equal(t, len(small), len(large))
	querybudgets.AssertQueryBudget(t, "services.enrollment.template_roster_maintenance_feeds.reads", large)
}

func TestEnrollmentRolloverReviewQueueQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	env := setupRolloverTest(t)
	ctx := testpkg.Ctx(t)
	createBudgetReviewRows(t, env, 3)
	counter := testpkg.CaptureQueries(t, env.db)
	run := func() []string {
		var queries []string
		err := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			counter.Reset()
			_, err := env.rolloverSvc.ListReviewQueue(txCtx, env.sourcePhase.ID)
			queries = counter.Operation("SELECT")
			return err
		})
		require.NoError(t, err)
		return queries
	}
	small := run()
	createBudgetReviewRows(t, env, 5)
	large := run()
	t.Logf("query budget: 3 review rows → %d reads, 8 review rows → %d reads", len(small), len(large))
	assert.Equal(t, len(small), len(large))
	querybudgets.AssertQueryBudget(t, "services.enrollment.rollover_review_queue.reads", large)
}

func createBudgetRequest(
	t *testing.T,
	env *rolloverTestEnv,
	phaseID int64,
	prefix string,
	childCount int,
	status string,
	sourceChildID *int64,
) int64 {
	t.Helper()
	request := &enrollmentAPI.Request{
		PhaseID: phaseID, GuardianFirstName: "Budget", GuardianLastName: "Test",
		GuardianEmail: prefix + "@example.test", ConsentFlags: map[string]any{}, CustomData: map[string]any{},
		SubmissionSource: enrollmentNative.RequestSourcePublic, SourceMetadata: map[string]any{}, StatusToken: prefix,
	}
	require.NoError(t, enrollmentAPI.InsertOwnerRequestForTest(testpkg.Ctx(t), env.repos.Enrollment(), request))
	for i := range childCount {
		createBudgetRequestChild(t, env, request.ID, prefix, i, status, sourceChildID)
	}
	return request.ID
}

func createBudgetRequestChild(
	t *testing.T,
	env *rolloverTestEnv,
	requestID int64,
	prefix string,
	sortOrder int,
	status string,
	sourceChildID *int64,
) {
	t.Helper()
	child := &enrollmentAPI.RequestChild{
		RequestID: requestID, FirstName: fmt.Sprintf("Child%d", sortOrder), LastName: prefix,
		DateOfBirth: "2018-04-15", CustomData: map[string]any{}, Status: status,
		ActivationMode: enrollmentNative.ChildActivationScheduled, SortOrder: sortOrder, RolloverSourceChildID: sourceChildID,
	}
	require.NoError(t, enrollmentAPI.InsertOwnerChildForTest(testpkg.Ctx(t), env.repos.Enrollment(), child))
}

func createBudgetReviewRows(t *testing.T, env *rolloverTestEnv, count int) {
	t.Helper()
	for i := range count {
		prefix := fmt.Sprintf("review-%d-%d", count, i)
		sourceRequestID := createBudgetRequest(t, env, env.sourcePhase.ID, prefix+"-source", 1, enrollmentNative.ChildStatusApproved, nil)
		sources, err := enrollmentAPI.ReadOwnerRequestChildrenForTest(testpkg.Ctx(t), env.repos.Enrollment(), sourceRequestID)
		require.NoError(t, err)
		require.Len(t, sources, 1)
		sourceID := sources[0].ID
		createBudgetRequest(t, env, env.sourcePhase.ID, prefix+"-target", 1, enrollmentNative.ChildStatusPendingAdminReview, &sourceID)
	}
}

// TestOfferingWeekdayBookingsListsTheBookedWeekdays: the Regeltermin hint
// (#5103) reads each approved child of an active offering with the weekdays
// its booking plans.
func TestOfferingWeekdayBookingsListsTheBookedWeekdays(t *testing.T) {
	t.Parallel()
	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	source := createAdjustmentCareOffering(t, env, uniqueSchemaName("Randstunde Hinweis"))
	_, _, studentID := submitApprovedAdjustmentChild(t, env, "hinweis@example.com", "Hinweis", []*offeringContract.CareOfferingRecord{source})
	lister := env.bookings.(careplan.OfferingSourceEditor)

	var bookings []careplan.OfferingWeekdayBooking
	err := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		var err error
		bookings, err = lister.OfferingWeekdayBookings(txCtx, nil, calendar.Date(env.sourcePhase.ServiceStartDate))
		return err
	})
	require.NoError(t, err)

	require.Len(t, bookings, 1)
	assert.Equal(t, source.ID, bookings[0].OfferingID)
	assert.Equal(t, source.Name, bookings[0].OfferingName)
	assert.Equal(t, source.PhaseID, bookings[0].PhaseID, "a combined source stays within one phase (#5139)")
	assert.False(t, bookings[0].HolidayPhase, "a school-year phase is no Ferienbetreuung phase (#5139)")
	assert.Equal(t, studentID, bookings[0].StudentID)
	assert.Equal(t, []int{1}, bookings[0].Weekdays, "the booking plans Monday")
	assert.Equal(t, source.CountsAsCare, bookings[0].CountsAsCare, "the catalog flags travel with the booking")
	assert.Equal(t, source.IncludesLunch, bookings[0].IncludesLunch)
	require.Len(t, bookings[0].Windows, 1)
	require.NotNil(t, bookings[0].Windows[0].ValidFrom)
	assert.Equal(t, calendar.Date(env.sourcePhase.ServiceStartDate), *bookings[0].Windows[0].ValidFrom)
	require.NotNil(t, bookings[0].Windows[0].ValidUntil)
	assert.Equal(t, calendar.Date(env.sourcePhase.ServiceEndDate).AddDays(1), *bookings[0].Windows[0].ValidUntil,
		"an open-ended booking ends with the phase's inclusive last service day")
}

// TestOfferingWeekdayBookingsCarryTheClassDayFacts: the offering-gap check
// (#5343) reads from each booking whether the Klassentag applies, which the
// school-year phase allows unless the offering includes holiday care, and
// the offering's pickup times.
func TestOfferingWeekdayBookingsCarryTheClassDayFacts(t *testing.T) {
	t.Parallel()
	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	pickups := map[string]string{"mon": "12:45", "tue": "12:45", "wed": "12:45", "thu": "12:45", "fri": "12:45"}
	schoolDays := createAdjustmentCareOfferingWith(t, env, uniqueSchemaName("Randstunde Klassentag"), func(offering *offeringContract.CareOfferingRecord) {
		offering.CountsAsCare, offering.PickupTimes = true, pickups
	})
	withHolidays := createAdjustmentCareOfferingWith(t, env, uniqueSchemaName("Randstunde Ferien"), func(offering *offeringContract.CareOfferingRecord) {
		offering.CountsAsCare, offering.PickupTimes, offering.IncludesHolidayCare = true, pickups, true
	})
	submitApprovedAdjustmentChild(t, env, "klassentag@example.com", "Klassentag", []*offeringContract.CareOfferingRecord{schoolDays, withHolidays})
	lister := env.bookings.(careplan.OfferingSourceEditor)

	var bookings []careplan.OfferingWeekdayBooking
	err := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		var err error
		bookings, err = lister.OfferingWeekdayBookings(txCtx, nil, calendar.Date(env.sourcePhase.ServiceStartDate))
		return err
	})
	require.NoError(t, err)

	byOffering := map[int64]careplan.OfferingWeekdayBooking{}
	for _, booking := range bookings {
		byOffering[booking.OfferingID] = booking
	}
	require.Contains(t, byOffering, schoolDays.ID)
	require.Contains(t, byOffering, withHolidays.ID)
	assert.True(t, byOffering[schoolDays.ID].ClassDayApplies, "a school-year phase follows the school days")
	assert.False(t, byOffering[withHolidays.ID].ClassDayApplies, "holiday care runs on days without school")
	assert.Equal(t, map[int]string{1: "12:45", 2: "12:45", 3: "12:45", 4: "12:45", 5: "12:45"}, byOffering[schoolDays.ID].PickupTimes)
}

// TestOfferingTemplateGapDecisionTravelsWithTheBookings: the school's
// decision "Kein eigener Regeltermin nötig" (#5140) is stored on the
// offering for one planning period, with its classes trimmed, deduplicated
// and sorted. It travels with every booking read for that period, never
// with a read for another period, and can be withdrawn. Each booking carries
// the validity of its link. Another school neither sees nor changes the
// offering.
func TestOfferingTemplateGapDecisionTravelsWithTheBookings(t *testing.T) {
	t.Parallel()
	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	source := createAdjustmentCareOffering(t, env, uniqueSchemaName("Randstunde Entscheidung"))
	submitApprovedAdjustmentChild(t, env, "entscheidung@example.com", "Entscheidung", []*offeringContract.CareOfferingRecord{source})
	decided := createCareOfferingTestPeriod(t, env.db, "gap-decided", calendar.NewDate(2026, 8, 1), calendar.NewDate(2027, 8, 31))
	other := createCareOfferingTestPeriod(t, env.db, "gap-other", calendar.NewDate(2026, 8, 1), calendar.NewDate(2027, 8, 31))
	lister := env.bookings.(careplan.OfferingSourceEditor)
	inTenant := func(tenantID int64, fn func(context.Context) error) error {
		base := ctx
		if tenantID != testpkg.Tenant(t) {
			base = context.Background()
		}
		return testpkg.WithTenantTx(t, base, env.db, tenantID, func(txCtx context.Context, _ bun.Tx) error { return fn(txCtx) })
	}
	booked := func(periodID int64) careplan.OfferingWeekdayBooking {
		var bookings []careplan.OfferingWeekdayBooking
		require.NoError(t, inTenant(testpkg.Tenant(t), func(txCtx context.Context) error {
			var err error
			bookings, err = lister.OfferingWeekdayBookings(txCtx, &periodID, calendar.Date(env.sourcePhase.ServiceStartDate))
			return err
		}))
		require.Len(t, bookings, 1)
		return bookings[0]
	}
	decide := func(tenantID int64, classes []string) error {
		return inTenant(tenantID, func(txCtx context.Context) error {
			return env.offeringCatalog.DecideOfferingTemplateGap(txCtx, source.ID, decided.ID, classes)
		})
	}

	first := booked(decided.ID)
	assert.Empty(t, first.TemplateGapAckClasses, "no decision before the school decides")
	require.Len(t, first.Windows, 1, "the link's validity travels with the booking")
	assert.Equal(t, first.Weekdays, first.Windows[0].Weekdays)

	require.NoError(t, decide(testpkg.Tenant(t), []string{"4a ", "3a", "", "3a"}))
	assert.Equal(t, []string{"", "3a", "4a"}, booked(decided.ID).TemplateGapAckClasses)
	assert.Empty(t, booked(other.ID).TemplateGapAckClasses, "a decision holds for its planning period only")

	otherSchool, _ := testpkg.CreateTestTenant(t, env.db)
	err := decide(otherSchool, nil)
	require.ErrorIs(t, err, careplan.ErrCareOfferingNotFound, "another school cannot reach the offering")
	assert.Equal(t, []string{"", "3a", "4a"}, booked(decided.ID).TemplateGapAckClasses)

	require.NoError(t, decide(testpkg.Tenant(t), nil))
	assert.Empty(t, booked(decided.ID).TemplateGapAckClasses, "withdrawing removes the decision")
}

func TestOfferingWeekdayBookingsQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	source := createAdjustmentCareOffering(t, env, uniqueSchemaName("Hinweis Budget"))
	lister := env.bookings.(careplan.OfferingSourceEditor)
	approved := 0
	approve := func(count int) {
		for range count {
			submitApprovedAdjustmentChild(t, env, fmt.Sprintf("hinweis-budget-%d@example.com", approved),
				fmt.Sprintf("HinweisBudget%d", approved), []*offeringContract.CareOfferingRecord{source})
			approved++
		}
	}
	counter := testpkg.CaptureQueries(t, env.db)
	run := func() []string {
		var reads []string
		err := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			counter.Reset()
			bookings, err := lister.OfferingWeekdayBookings(txCtx, nil, calendar.Date(env.sourcePhase.ServiceStartDate))
			reads = counter.Operation("SELECT")
			require.Len(t, bookings, approved)
			return err
		})
		require.NoError(t, err)
		return reads
	}

	approve(3)
	small := run()
	approve(5)
	large := run()

	t.Logf("query budget: 3 children → %d reads, 8 children → %d reads", len(small), len(large))
	assert.Equal(t, len(small), len(large))
	querybudgets.AssertQueryBudget(t, "services.enrollment.offering_weekday_bookings.reads", large)
}

// TestOfferingPhasesOutsidePeriodQueryBudget (#5344): the phases a planning
// period leaves out are read with the setting, the offerings, the phases and
// the period, flat in the number of offerings. The source phase ends after
// the period, so it is named with its active offerings.
func TestOfferingPhasesOutsidePeriodQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	period := createCareOfferingTestPeriod(t, env.db, "outside-budget",
		calendar.Date(env.sourcePhase.ServiceStartDate), calendar.Date(env.sourcePhase.ServiceEndDate).AddDays(-1))
	lister := env.bookings.(careplan.OfferingSourceEditor)
	counter := testpkg.CaptureQueries(t, env.db)
	run := func(wantOfferings int) []string {
		var reads []string
		err := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			counter.Reset()
			outside, err := lister.OfferingPhasesOutsidePeriod(txCtx, period.ID)
			reads = counter.Operation("SELECT")
			require.Len(t, outside, 1)
			assert.Equal(t, env.sourcePhase.ID, outside[0].Phase.ID)
			assert.Equal(t, wantOfferings, outside[0].OfferingCount)
			return err
		})
		require.NoError(t, err)
		return reads
	}
	for i := range 3 {
		createSourceOffering(t, env, fmt.Sprintf("Outside small %d", i), nil)
	}
	small := run(3)
	for i := 3; i < 8; i++ {
		createSourceOffering(t, env, fmt.Sprintf("Outside large %d", i), nil)
	}
	large := run(8)

	t.Logf("query budget: 3 offerings → %d reads, 8 offerings → %d reads", len(small), len(large))
	assert.Equal(t, len(small), len(large))
	querybudgets.AssertQueryBudget(t, "services.enrollment.offering_phases_outside_period.reads", large)
}
