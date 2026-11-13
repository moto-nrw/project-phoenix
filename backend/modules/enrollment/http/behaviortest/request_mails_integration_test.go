package enrollmenthttp_test

import (
	"context"
	"testing"

	enrollmentNative "github.com/moto-ogs/project-phoenix/modules/enrollment"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A school reads the mails of its own application only (#5934).
func TestRequestMailOutcomesStayInsideTheSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	schoolA := newDeletionTestFixture(t, db, "mails-a")
	schoolB := newDeletionTestFixture(t, db, "mails-b")
	requestB := schoolB.request("mails-target", nil)
	enqueueTestEnrollmentEmail(t, db, schoolB.scope.TenantID, requestB.ID, "mails-b-1", map[string]any{"recipient": "b@example.com"})

	_, err := tenantCall(t, db, schoolA.scope.TenantID, func(ctx context.Context) ([]enrollmentNative.RequestMailOutcome, error) {
		return schoolA.service(nil, nil).RequestMailOutcomes(ctx, requestB.ID)
	})
	require.ErrorIs(t, err, enrollmentNative.ErrEnrollmentDeletionNotFound)

	own, err := tenantCall(t, db, schoolB.scope.TenantID, func(ctx context.Context) ([]enrollmentNative.RequestMailOutcome, error) {
		return schoolB.service(nil, nil).RequestMailOutcomes(ctx, requestB.ID)
	})
	require.NoError(t, err)
	require.Len(t, own, 1)
	assert.Equal(t, "queued", own[0].Outcome, "a waiting mail is no delivery")
}

func TestRequestMailOutcomesQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	counter := testpkg.CaptureQueriesForContext(t, db)
	school := newDeletionTestFixture(t, db, "mails-budget")
	request := school.request("mails-budget", nil)
	read := func() int {
		counter.Reset()
		_, err := tenantCall(t, db, school.scope.TenantID, func(ctx context.Context) ([]enrollmentNative.RequestMailOutcome, error) {
			return school.service(nil, nil).RequestMailOutcomes(ctx, request.ID)
		})
		require.NoError(t, err)
		return len(counter.Queries())
	}
	enqueueTestEnrollmentEmail(t, db, school.scope.TenantID, request.ID, "budget-1", map[string]any{"recipient": "a@example.com"})
	first := read()
	querybudgets.AssertQueryBudget(t, "api.enrollment.request_mails", counter.Measurement(counter.Queries()))
	enqueueTestEnrollmentEmail(t, db, school.scope.TenantID, request.ID, "budget-2", map[string]any{"recipient": "a@example.com"})
	enqueueTestEnrollmentEmail(t, db, school.scope.TenantID, request.ID, "budget-3", map[string]any{"recipient": "a@example.com"})
	assert.Equal(t, first, read(), "a further mail adds no statement")
}
