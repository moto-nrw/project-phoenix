package contracttest_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type weeklyApprovalBoundary struct {
	owner    careplan.Capability
	staffID  int64
	modes    map[string][]string
	saved    map[string][]string
	booked   bool
	auditErr error
	calls    []string
}

func (b *weeklyApprovalBoundary) TrackCompanionChanges(ctx context.Context) (context.Context, func() bool) {
	return ctx, func() bool { return true }
}
func (b *weeklyApprovalBoundary) ActingStaffID(context.Context) (int64, error) {
	b.calls = append(b.calls, "staff")
	return b.staffID, nil
}
func (b *weeklyApprovalBoundary) LockDepartureModes(context.Context, int64) (map[string][]string, error) {
	b.calls = append(b.calls, "lock")
	return b.modes, nil
}
func (b *weeklyApprovalBoundary) SaveDepartureModes(_ context.Context, _ int64, modes map[string][]string) error {
	b.calls = append(b.calls, "save")
	b.saved = modes
	return nil
}
func (b *weeklyApprovalBoundary) AuditDepartureModes(context.Context, int64, int64) error {
	b.calls = append(b.calls, "audit")
	return b.auditErr
}
func (b *weeklyApprovalBoundary) HasBookedOfferingPickupForWeekday(context.Context, int64, int) (bool, error) {
	return b.booked, nil
}
func (b *weeklyApprovalBoundary) GetStudentArrivalSchedules(ctx context.Context, id int64) ([]*careplan.ArrivalSchedule, error) {
	rows, err := b.owner.ListArrivalSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}})
	result := make([]*careplan.ArrivalSchedule, len(rows))
	for i := range rows {
		result[i] = &rows[i]
	}
	return result, err
}
func (b *weeklyApprovalBoundary) GetStudentPickupSchedules(ctx context.Context, id int64) ([]*careplan.PickupSchedule, error) {
	rows, err := b.owner.ListPickupSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}})
	result := make([]*careplan.PickupSchedule, len(rows))
	for i := range rows {
		result[i] = &rows[i]
	}
	return result, err
}
func (b *weeklyApprovalBoundary) UpsertStudentArrivalSchedule(ctx context.Context, row *careplan.ArrivalSchedule) error {
	_, err := b.owner.UpsertArrivalSchedule(ctx, *row)
	return err
}
func (b *weeklyApprovalBoundary) UpsertStudentPickupSchedule(ctx context.Context, row *careplan.PickupSchedule) error {
	_, err := b.owner.UpsertPickupSchedule(ctx, *row)
	return err
}
func (b *weeklyApprovalBoundary) DeleteStudentArrivalSchedule(ctx context.Context, id int64) error {
	return b.owner.DeleteArrivalSchedule(ctx, id)
}
func (b *weeklyApprovalBoundary) DeleteStudentPickupSchedule(ctx context.Context, id int64) error {
	return b.owner.DeletePickupSchedule(ctx, id)
}

func TestNativeWeeklyApprovalMergesAndRollsBackOnAuditFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	owner := careplantest.NewCarePlan(t, db)
	student := testpkg.CreateTestStudent(t, db, "Weekly", "Approval", "1a")
	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Weekly", "Reviewer")
	clock := time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC)
	monday, err := owner.CreatePickupSchedule(ctx, careplan.PickupSchedule{StudentID: student.ID, Weekday: 1, PickupTime: clock, CreatedBy: staff.ID})
	require.NoError(t, err)
	friday, err := owner.CreatePickupSchedule(ctx, careplan.PickupSchedule{StudentID: student.ID, Weekday: 5, PickupTime: clock, CreatedBy: staff.ID})
	require.NoError(t, err)
	b := &weeklyApprovalBoundary{owner: owner, staffID: staff.ID, modes: map[string][]string{"mon": {"bus", "pickup"}, "fri": {"pickup"}}, auditErr: errors.New("audit unavailable")}
	service, err := compose.NewWeeklyApprovals(b, b, b)
	require.NoError(t, err)
	request := &carerequests.Request{StudentID: student.ID, Payload: json.RawMessage(`{"weekdays":[{"weekday":5,"mode":"alone","arrival":"08:00","pickup":"14:00"}]}`)}
	apply := func() (bool, error) {
		var changed bool
		err := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			var applyErr error
			changed, applyErr = service.ApplyWeekly(txCtx, request, account.ID)
			return applyErr
		})
		return changed, err
	}
	_, err = apply()
	require.ErrorIs(t, err, b.auditErr)
	arrivals, err := b.GetStudentArrivalSchedules(ctx, student.ID)
	require.NoError(t, err)
	require.Empty(t, arrivals)
	unchanged, err := owner.FindPickupSchedule(ctx, friday.ID)
	require.NoError(t, err)
	require.Equal(t, "15:00", unchanged.PickupTime.Format("15:04"))
	require.Equal(t, []string{"pickup"}, b.modes["fri"], "merge must not mutate the locked snapshot")
	b.auditErr, b.calls = nil, nil
	changed, err := apply()
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []string{"staff", "lock", "save", "audit"}, b.calls)
	require.Equal(t, map[string][]string{"mon": {"bus", "pickup"}, "fri": {"alone"}}, b.saved)
	updated, err := owner.FindPickupSchedule(ctx, friday.ID)
	require.NoError(t, err)
	require.Equal(t, "14:00", updated.PickupTime.Format("15:04"))
	untouched, err := owner.FindPickupSchedule(ctx, monday.ID)
	require.NoError(t, err)
	require.Equal(t, "15:00", untouched.PickupTime.Format("15:04"))
	request.Payload = json.RawMessage(`{"weekdays":[{"weekday":5,"scheduled":false}]}`)
	b.booked = true
	_, err = apply()
	require.ErrorIs(t, err, carerequests.ErrCareDayManagedByBooking)
	_, err = owner.FindPickupSchedule(ctx, friday.ID)
	require.NoError(t, err, "booking guard precedes schedule deletion")
	b.booked = false
	_, err = apply()
	require.NoError(t, err)
	require.NotContains(t, b.saved, "fri")
	_, err = owner.FindPickupSchedule(ctx, friday.ID)
	require.ErrorIs(t, err, careplan.ErrStudentScheduleNotFound)
}
