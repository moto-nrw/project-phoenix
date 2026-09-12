package compose

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestPickupReviewQueriesBoundedAndTenantScoped(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module := buildModule(t, db)
	fixture := newOwnedActivityInstanceFixture(t, db, "pickup-review")
	before := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-01", "13:00:00", "Before pickup")
	after := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-01", "15:00:00", "After pickup")
	student := testpkg.CreateTestStudent(t, db, "Pickup", "Review", "3a")
	createOwnedInstanceStudent(t, module, ctx, before.ID, student.ID, timetable.InstanceAttendanceExpected)
	createOwnedInstanceStudent(t, module, ctx, after.ID, student.ID, timetable.InstanceAttendanceExpected)
	var observations []Observation
	query, err := NewPickupReviewQueries(db, func(value Observation) { observations = append(observations, value) })
	require.NoError(t, err)
	input := timetable.PickupReviewInput{StudentID: student.ID, Date: "2027-11-01", From: time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC)}
	blocks, err := query.PreviewPickupBlocks(ctx, input)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	require.Equal(t, after.ID, blocks[0].ID)
	require.EqualValues(t, 1, observations[0].Stats.Queries)
	input.Enrolled = true
	blocks, err = query.PreviewPickupBlocks(ctx, input)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	require.EqualValues(t, 2, observations[1].Stats.Queries, "only bounded attendance and enrollment reads are needed")
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	blocks, err = query.PreviewPickupBlocks(tenant.WithTenantID(ctx, otherTenant), input)
	require.NoError(t, err)
	require.Empty(t, blocks)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = query.PreviewPickupBlocks(canceled, input)
	require.ErrorIs(t, err, context.Canceled)
	input.Date = "2027-02-30"
	_, err = query.PreviewPickupBlocks(ctx, input)
	require.Error(t, err)
	_, err = NewPickupReviewQueries(nil, func(Observation) {})
	require.Error(t, err)
}
