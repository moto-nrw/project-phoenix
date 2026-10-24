package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// sickReportCalls records the writes and the cascade steps in order.
type sickReportCalls struct {
	calls      []string
	plan       workforce.SickCascadePlan
	writeErr   error
	cascadeErr error
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

func inSickReportTx() context.Context {
	return tenant.WithTransactionForTest(context.Background(), struct{}{})
}

func TestSickReportsApplyThePlanInOrderAfterTheWrite(t *testing.T) {
	t.Parallel()
	calls := &sickReportCalls{plan: workforce.SickCascadePlan{
		{Kind: workforce.SickCascadeReassign, FromAbsenceID: 43, ToAbsenceID: 42},
		{Kind: workforce.SickCascadeMark, Report: workforce.SickCascadeInput{DateStart: "2026-02-10"}},
	}}
	reports := NewSickReports(calls, calls)

	created, err := reports.CreateAbsenceFor(inSickReportTx(), 11, 12, nil, workforce.CreateAbsenceRequest{})
	require.NoError(t, err)
	assert.Equal(t, int64(42), created.ID)
	assert.Equal(t, []string{"create for", "reassign", "mark", "2026-02-10"}, calls.calls)

	calls.calls, calls.plan = nil, workforce.SickCascadePlan{{Kind: workforce.SickCascadeReconcile,
		Before: workforce.SickCascadeInput{DateEnd: "2026-02-11"}, Report: workforce.SickCascadeInput{DateEnd: "2026-02-13"}}}
	_, err = reports.UpdateAbsence(inSickReportTx(), 11, nil, 42, workforce.UpdateAbsenceRequest{})
	require.NoError(t, err)
	assert.Equal(t, []string{"update", "reconcile", "2026-02-11", "2026-02-13"}, calls.calls)

	calls.calls, calls.plan = nil, workforce.SickCascadePlan{{Kind: workforce.SickCascadeClear, Report: workforce.SickCascadeInput{DateStart: "2026-02-10"}}}
	require.NoError(t, reports.DeleteAbsenceFor(inSickReportTx(), 11, 12, nil, 42))
	require.NoError(t, reports.DeleteOwnAbsence(inSickReportTx(), 11, nil, 42))
	_, err = reports.CreateOwnAbsence(inSickReportTx(), 11, nil, workforce.CreateAbsenceRequest{})
	require.NoError(t, err)
	assert.Equal(t, []string{"delete for", "clear", "2026-02-10", "delete own", "clear", "2026-02-10", "create own", "clear", "2026-02-10"}, calls.calls)
}

func TestSickReportsFailClosed(t *testing.T) {
	t.Parallel()
	refused := errors.New("absence refused")
	calls := &sickReportCalls{writeErr: refused, plan: workforce.SickCascadePlan{{Kind: workforce.SickCascadeMark}}}
	reports := NewSickReports(calls, calls)
	_, err := reports.CreateAbsenceFor(inSickReportTx(), 11, 12, nil, workforce.CreateAbsenceRequest{})
	require.ErrorIs(t, err, refused)
	require.ErrorIs(t, reports.DeleteAbsenceFor(inSickReportTx(), 11, 12, nil, 42), refused)
	assert.Equal(t, []string{"create for", "delete for"}, calls.calls, "a refused write applies no plan")

	broken := errors.New("plan write failed")
	for _, tc := range []struct {
		kind    workforce.SickCascadeStepKind
		message string
	}{
		{workforce.SickCascadeMark, "sick report saved nothing — plan cascade failed: plan write failed"},
		{workforce.SickCascadeClear, "absence not deleted — plan cascade reversal failed: plan write failed"},
		{workforce.SickCascadeReconcile, "absence update saved nothing — plan cascade reconciliation failed: plan write failed"},
		{workforce.SickCascadeReassign, "failed to merge absence — plan stamp reassignment failed: plan write failed"},
	} {
		calls := &sickReportCalls{cascadeErr: broken, plan: workforce.SickCascadePlan{{Kind: tc.kind}, {Kind: workforce.SickCascadeMark}}}
		_, err := NewSickReports(calls, calls).CreateAbsenceFor(inSickReportTx(), 11, 12, nil, workforce.CreateAbsenceRequest{})
		require.ErrorIs(t, err, broken)
		assert.EqualError(t, err, tc.message)
		assert.Len(t, calls.calls, 1+map[workforce.SickCascadeStepKind]int{
			workforce.SickCascadeMark: 2, workforce.SickCascadeClear: 2, workforce.SickCascadeReconcile: 3, workforce.SickCascadeReassign: 1,
		}[tc.kind], "the first failed step stops the plan")
	}

	calls = &sickReportCalls{plan: workforce.SickCascadePlan{{Kind: "unknown"}}}
	_, err = NewSickReports(calls, calls).CreateAbsenceFor(inSickReportTx(), 11, 12, nil, workforce.CreateAbsenceRequest{})
	require.EqualError(t, err, `sick cascade: unknown plan step "unknown"`)
}
