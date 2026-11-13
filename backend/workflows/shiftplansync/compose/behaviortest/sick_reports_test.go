package compose_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/workflows/shiftplansync"
)

// inTx records that the entry ran its write inside the unit of work.
func inTx(calls *sickReportCalls) func(context.Context, func(context.Context) error) error {
	return func(ctx context.Context, fn func(context.Context) error) error {
		calls.calls = append(calls.calls, "tx")
		return fn(ctx)
	}
}

func newSickReports(t *testing.T, calls *sickReportCalls) *shiftplansync.SickReports {
	t.Helper()
	reports, err := shiftplansync.NewSickReports(calls, calls, inTx(calls))
	require.NoError(t, err)
	return reports
}

// sickReportCalls records the writes and the cascade steps in order.
type sickReportCalls struct {
	calls      []string
	plan       workforce.SickCascadePlan
	lockErr    error
	writeErr   error
	cascadeErr error
}

func (c *sickReportCalls) ApproveAbsence(context.Context, int64, int64, int64, string) (*workforce.StaffAbsenceResponse, workforce.SickCascadePlan, error) {
	c.calls = append(c.calls, "approve")
	return c.absence()
}

func (c *sickReportCalls) CancelAbsence(context.Context, int64, int64, int64) (workforce.SickCascadePlan, error) {
	c.calls = append(c.calls, "cancel")
	return c.plan, c.writeErr
}

func (c *sickReportCalls) CreateOwnAbsence(context.Context, int64, *int64, workforce.CreateAbsenceRequest) (*workforce.StaffAbsenceResponse, workforce.SickCascadePlan, error) {
	c.calls = append(c.calls, "create own")
	return c.absence()
}

func (c *sickReportCalls) CreateAbsenceFor(context.Context, int64, int64, *int64, workforce.CreateAbsenceRequest) (*workforce.StaffAbsenceResponse, workforce.SickCascadePlan, error) {
	c.calls = append(c.calls, "create for")
	return c.absence()
}

func (c *sickReportCalls) UpdateAbsence(context.Context, int64, *int64, int64, workforce.UpdateAbsenceRequest) (*workforce.StaffAbsenceResponse, workforce.SickCascadePlan, error) {
	c.calls = append(c.calls, "update")
	return c.absence()
}

func (c *sickReportCalls) DeleteOwnAbsence(context.Context, int64, *int64, int64) (workforce.SickCascadePlan, error) {
	c.calls = append(c.calls, "delete own")
	return c.plan, c.writeErr
}

func (c *sickReportCalls) DeleteAbsenceFor(context.Context, int64, int64, *int64, int64) (workforce.SickCascadePlan, error) {
	c.calls = append(c.calls, "delete for")
	return c.plan, c.writeErr
}

func (c *sickReportCalls) absence() (*workforce.StaffAbsenceResponse, workforce.SickCascadePlan, error) {
	if c.writeErr != nil {
		return nil, nil, c.writeErr
	}
	return &workforce.StaffAbsenceResponse{StaffAbsence: &workforce.StaffAbsence{ID: 42}}, c.plan, nil
}

func (c *sickReportCalls) MarkSickForRange(_ context.Context, in workforce.SickCascadeInput) error {
	c.calls = append(c.calls, "mark", in.DateStart)
	return c.cascadeErr
}

func (c *sickReportCalls) ClearSickForRange(_ context.Context, in workforce.SickCascadeInput) error {
	c.calls = append(c.calls, "clear", in.DateStart)
	return c.cascadeErr
}

func (c *sickReportCalls) ReconcileSickRange(_ context.Context, before, after workforce.SickCascadeInput) error {
	c.calls = append(c.calls, "reconcile", before.DateEnd, after.DateEnd)
	return c.cascadeErr
}

func (c *sickReportCalls) ReassignSickStamps(context.Context, int64, int64) error {
	c.calls = append(c.calls, "reassign")
	return c.cascadeErr
}

func (c *sickReportCalls) LockSickReportWrite(_ context.Context, staffID, absenceID int64, _, dateStart, dateEnd string) error {
	c.calls = append(c.calls, "lock", fmt.Sprint(staffID, "/", absenceID, " ", dateStart, "..", dateEnd))
	return c.lockErr
}

func (c *sickReportCalls) AbsencePlanGaps(context.Context) ([]workforce.AbsencePlanGap, error) {
	c.calls = append(c.calls, "gaps")
	return nil, nil
}

func (c *sickReportCalls) MarkAbsenceInForce(_ context.Context, absenceID, _ int64, _ *int64) (bool, error) {
	c.calls = append(c.calls, fmt.Sprint("mark-in-force ", absenceID))
	return true, nil
}

func inSickReportTx() context.Context { return context.Background() }

func TestSickReportsApplyThePlanInOrderAfterTheWrite(t *testing.T) {
	t.Parallel()
	calls := &sickReportCalls{plan: workforce.SickCascadePlan{
		{Kind: workforce.SickCascadeReassign, FromAbsenceID: 43, ToAbsenceID: 42},
		{Kind: workforce.SickCascadeMark, Report: workforce.SickCascadeInput{DateStart: "2026-02-10"}},
	}}
	reports := newSickReports(t, calls)

	created, err := reports.CreateAbsenceFor(inSickReportTx(), 11, 12, nil, workforce.CreateAbsenceRequest{DateStart: "2026-02-10", DateEnd: "2026-02-12"})
	require.NoError(t, err)
	assert.Equal(t, int64(42), created.ID)
	assert.Equal(t, []string{"tx", "lock", "11/0 2026-02-10..2026-02-12", "create for", "reassign", "mark", "2026-02-10"}, calls.calls,
		"the locks, the write and its plan run in one unit of work, the locks first (#6232)")

	calls.calls, calls.plan = nil, workforce.SickCascadePlan{{Kind: workforce.SickCascadeReconcile,
		Before: workforce.SickCascadeInput{DateEnd: "2026-02-11"}, Report: workforce.SickCascadeInput{DateEnd: "2026-02-13"}}}
	end := "2026-02-13"
	_, err = reports.UpdateAbsence(inSickReportTx(), 11, nil, 42, workforce.UpdateAbsenceRequest{DateEnd: &end})
	require.NoError(t, err)
	assert.Equal(t, []string{"tx", "lock", "11/42 ..2026-02-13", "update", "reconcile", "2026-02-11", "2026-02-13"}, calls.calls)

	calls.calls, calls.plan = nil, workforce.SickCascadePlan{{Kind: workforce.SickCascadeClear, Report: workforce.SickCascadeInput{DateStart: "2026-02-10"}}}
	require.NoError(t, reports.DeleteAbsenceFor(inSickReportTx(), 11, 12, nil, 42))
	require.NoError(t, reports.DeleteOwnAbsence(inSickReportTx(), 11, nil, 42))
	_, err = reports.CreateOwnAbsence(inSickReportTx(), 11, nil, workforce.CreateAbsenceRequest{})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"tx", "lock", "11/42 ..", "delete for", "clear", "2026-02-10",
		"tx", "lock", "11/42 ..", "delete own", "clear", "2026-02-10",
		"tx", "lock", "11/0 ..", "create own", "clear", "2026-02-10",
	}, calls.calls)
}

// An approval and a withdrawal take plan effect since #6847: both run with
// their plan in one unit of work behind the locks. A manager's approval names
// no subject; the lock reads it from the stored absence (staff 0).
func TestSickReportsApplyTheApprovalAndTheWithdrawalPlans(t *testing.T) {
	t.Parallel()
	calls := &sickReportCalls{plan: workforce.SickCascadePlan{
		{Kind: workforce.SickCascadeMark, Report: workforce.SickCascadeInput{DateStart: "2026-03-02"}},
	}}
	reports := newSickReports(t, calls)

	approved, err := reports.ApproveAbsence(inSickReportTx(), 42, 7, 12, "ok")
	require.NoError(t, err)
	assert.Equal(t, int64(42), approved.ID)

	calls.plan = workforce.SickCascadePlan{{Kind: workforce.SickCascadeClear, Report: workforce.SickCascadeInput{DateStart: "2026-03-02"}}}
	require.NoError(t, reports.CancelAbsence(inSickReportTx(), 11, 7, 42))

	assert.Equal(t, []string{
		"tx", "lock", "0/42 ..", "approve", "mark", "2026-03-02",
		"tx", "lock", "11/42 ..", "cancel", "clear", "2026-03-02",
	}, calls.calls)
}

func TestSickReportsFailClosed(t *testing.T) {
	t.Parallel()
	refused := errors.New("absence refused")
	calls := &sickReportCalls{writeErr: refused, plan: workforce.SickCascadePlan{{Kind: workforce.SickCascadeMark}}}
	reports := newSickReports(t, calls)
	_, err := reports.CreateAbsenceFor(inSickReportTx(), 11, 12, nil, workforce.CreateAbsenceRequest{})
	require.ErrorIs(t, err, refused)
	require.ErrorIs(t, reports.DeleteAbsenceFor(inSickReportTx(), 11, 12, nil, 42), refused)
	assert.Equal(t, []string{"tx", "lock", "11/0 ..", "create for", "tx", "lock", "11/42 ..", "delete for"}, calls.calls, "a refused write applies no plan")

	lockFailed := errors.New("lock refused")
	calls = &sickReportCalls{lockErr: lockFailed, plan: workforce.SickCascadePlan{{Kind: workforce.SickCascadeMark}}}
	_, err = newSickReports(t, calls).CreateAbsenceFor(inSickReportTx(), 11, 12, nil, workforce.CreateAbsenceRequest{})
	require.ErrorIs(t, err, lockFailed)
	require.ErrorIs(t, newSickReports(t, calls).DeleteAbsenceFor(inSickReportTx(), 11, 12, nil, 42), lockFailed)
	assert.Equal(t, []string{"tx", "lock", "11/0 ..", "tx", "lock", "11/42 .."}, calls.calls, "a failed lock writes nothing")

	broken := errors.New("plan write failed")
	for _, tc := range []struct {
		kind    workforce.SickCascadeStepKind
		message string
	}{
		{workforce.SickCascadeMark, "absence saved nothing — plan cascade failed: plan write failed"},
		{workforce.SickCascadeClear, "absence not deleted — plan cascade reversal failed: plan write failed"},
		{workforce.SickCascadeReconcile, "absence update saved nothing — plan cascade reconciliation failed: plan write failed"},
		{workforce.SickCascadeReassign, "failed to merge absence — plan stamp reassignment failed: plan write failed"},
	} {
		calls := &sickReportCalls{cascadeErr: broken, plan: workforce.SickCascadePlan{{Kind: tc.kind}, {Kind: workforce.SickCascadeMark}}}
		_, err := newSickReports(t, calls).CreateAbsenceFor(inSickReportTx(), 11, 12, nil, workforce.CreateAbsenceRequest{})
		require.ErrorIs(t, err, broken)
		assert.EqualError(t, err, tc.message)
		assert.Len(t, calls.calls, 4+map[workforce.SickCascadeStepKind]int{
			workforce.SickCascadeMark: 2, workforce.SickCascadeClear: 2, workforce.SickCascadeReconcile: 3, workforce.SickCascadeReassign: 1,
		}[tc.kind], "the first failed step stops the plan")
	}

	calls = &sickReportCalls{plan: workforce.SickCascadePlan{{Kind: "unknown"}}}
	_, err = newSickReports(t, calls).CreateAbsenceFor(inSickReportTx(), 11, 12, nil, workforce.CreateAbsenceRequest{})
	require.EqualError(t, err, `sick cascade: unknown plan step "unknown"`)
}

func TestNewSickReportsRequiresTheWritesTheCascadeAndTheUnitOfWork(t *testing.T) {
	t.Parallel()
	calls := &sickReportCalls{}
	_, err := shiftplansync.NewSickReports(nil, calls, inTx(calls))
	require.Error(t, err)
	_, err = shiftplansync.NewSickReports(calls, nil, inTx(calls))
	require.Error(t, err)
	_, err = shiftplansync.NewSickReports(calls, calls, nil)
	require.Error(t, err)
}

// The take-over of an absence in force (#7021) runs its locks before the
// cascade in one unit of work; a lock failure writes nothing.
func TestSickReportsSyncAnAbsencePlanBehindItsLocks(t *testing.T) {
	t.Parallel()
	calls := &sickReportCalls{}
	reports := newSickReports(t, calls)

	applied, err := reports.SyncAbsencePlan(inSickReportTx(), 42, 12, nil)
	require.NoError(t, err)
	assert.True(t, applied)
	assert.Equal(t, []string{"tx", "lock", "0/42 ..", "mark-in-force 42"}, calls.calls,
		"the stored absence names its subject; the lock reads it (staff 0)")

	calls.calls, calls.lockErr = nil, errors.New("lock timeout")
	_, err = reports.SyncAbsencePlan(inSickReportTx(), 42, 12, nil)
	require.ErrorIs(t, err, calls.lockErr)
	assert.Equal(t, []string{"tx", "lock", "0/42 .."}, calls.calls, "no cascade without its locks")
}
