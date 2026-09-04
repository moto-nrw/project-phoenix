package enrollment_test

import (
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnrollmentListChildOfferingsQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	smallID := createBudgetRequest(t, env.rolloverTestEnv, env.sourcePhase.ID, "child-small", 3, enrollmentModels.ChildStatusSubmitted, nil)
	largeID := createBudgetRequest(t, env.rolloverTestEnv, env.sourcePhase.ID, "child-large", 8, enrollmentModels.ChildStatusSubmitted, nil)
	counter := testpkg.CaptureQueries(t, env.db)
	run := func(requestID int64) []string {
		counter.Reset()
		_, err := env.decision.ListChildOfferings(ctx, requestID)
		require.NoError(t, err)
		return counter.Operation("SELECT")
	}
	small, large := run(smallID), run(largeID)
	t.Logf("query budget: 3 children → %d reads, 8 children → %d reads", len(small), len(large))
	assert.Equal(t, len(small), len(large))
	testpkg.AssertQueryBudget(t, "services.enrollment.list_child_offerings.reads", large)
}

func TestEnrollmentOfferingSourceOptionsQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	for i := range 3 {
		createSourceOffering(t, env, fmt.Sprintf("Budget small %d", i), nil)
	}
	lister := env.decision.(enrollmentService.OfferingSourceOptionLister)
	counter := testpkg.CaptureQueries(t, env.db)
	run := func() []string {
		counter.Reset()
		_, err := lister.ListOfferingSourceOptions(ctx, nil)
		require.NoError(t, err)
		return counter.Operation("SELECT")
	}
	small := run()
	for i := 3; i < 8; i++ {
		createSourceOffering(t, env, fmt.Sprintf("Budget large %d", i), nil)
	}
	large := run()
	t.Logf("query budget: 3 offerings → %d reads, 8 offerings → %d reads", len(small), len(large))
	assert.Equal(t, len(small), len(large))
	testpkg.AssertQueryBudget(t, "services.enrollment.offering_source_options.reads", large)
}

func TestEnrollmentRolloverReviewQueueQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	createBudgetReviewRows(t, env, 3)
	counter := testpkg.CaptureQueries(t, env.db)
	run := func() []string {
		counter.Reset()
		_, err := env.rolloverSvc.ListReviewQueue(ctx, env.sourcePhase.ID)
		require.NoError(t, err)
		return counter.Operation("SELECT")
	}
	small := run()
	createBudgetReviewRows(t, env, 5)
	large := run()
	t.Logf("query budget: 3 review rows → %d reads, 8 review rows → %d reads", len(small), len(large))
	assert.Equal(t, len(small), len(large))
	testpkg.AssertQueryBudget(t, "services.enrollment.rollover_review_queue.reads", large)
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
	request := &enrollmentModels.Request{
		PhaseID: phaseID, GuardianFirstName: "Budget", GuardianLastName: "Test",
		GuardianEmail: prefix + "@example.test", ConsentFlags: map[string]any{}, CustomData: map[string]any{},
		SubmissionSource: enrollmentModels.RequestSourcePublic, SourceMetadata: map[string]any{}, StatusToken: prefix,
	}
	require.NoError(t, env.repos.Request.Create(testpkg.Ctx(t), request))
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
	child := &enrollmentModels.RequestChild{
		RequestID: requestID, FirstName: fmt.Sprintf("Child%d", sortOrder), LastName: prefix,
		DateOfBirth: timezone.NewDate(2018, 4, 15), CustomData: map[string]any{}, Status: status,
		ActivationMode: enrollmentModels.ChildActivationScheduled, SortOrder: sortOrder, RolloverSourceChildID: sourceChildID,
	}
	require.NoError(t, env.repos.RequestChild.Create(testpkg.Ctx(t), child))
}

func createBudgetReviewRows(t *testing.T, env *rolloverTestEnv, count int) {
	t.Helper()
	for i := range count {
		prefix := fmt.Sprintf("review-%d-%d", count, i)
		sourceRequestID := createBudgetRequest(t, env, env.sourcePhase.ID, prefix+"-source", 1, enrollmentModels.ChildStatusApproved, nil)
		sources, err := env.repos.RequestChild.ListByRequestID(testpkg.Ctx(t), sourceRequestID)
		require.NoError(t, err)
		require.Len(t, sources, 1)
		sourceID := sources[0].ID
		createBudgetRequest(t, env, env.sourcePhase.ID, prefix+"-target", 1, enrollmentModels.ChildStatusPendingAdminReview, &sourceID)
	}
}
