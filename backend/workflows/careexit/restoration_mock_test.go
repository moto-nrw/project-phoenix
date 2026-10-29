package careexit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/careexit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type unitKey struct{}

// restorationLifecycle answers the restoring commands and records whether
// they ran inside the workflow's unit.
type restorationLifecycle struct {
	err        error
	reapplied  []int64
	insideUnit bool
}

func (l *restorationLifecycle) Cancel(ctx context.Context, ids []int64, _ int64) (int, error) {
	l.insideUnit = ctx.Value(unitKey{}) != nil
	return len(ids), l.err
}

func (l *restorationLifecycle) Confirm(ctx context.Context, _ string, _ careplan.CareExitInput, _ int64) (*careplan.CareExitResult, error) {
	l.insideUnit = ctx.Value(unitKey{}) != nil
	if l.err != nil {
		return nil, l.err
	}
	return &careplan.CareExitResult{StudentsEnded: 2, ReappliedStudentIDs: l.reapplied}, nil
}

func (l *restorationLifecycle) ConfirmWithdrawalCareEnd(ctx context.Context, _ int64, token string, input careplan.CareExitInput, actor int64) (*careplan.CareExitResult, error) {
	return l.Confirm(ctx, token, input, actor)
}

type restorationResync struct {
	err   error
	calls [][]int64
	from  []calendar.Date
}

func (r *restorationResync) ResyncStudentSourcedRosters(ctx context.Context, ids []int64, from calendar.Date) error {
	if ctx.Value(unitKey{}) == nil {
		return errors.New("resync outside the unit")
	}
	r.calls = append(r.calls, ids)
	r.from = append(r.from, from)
	return r.err
}

func newTestRestoration(t *testing.T, lifecycle *restorationLifecycle, resync *restorationResync, today calendar.Date) *careexit.PlanRestoration {
	t.Helper()
	unit := func(ctx context.Context, fn func(context.Context) error) error {
		return fn(context.WithValue(ctx, unitKey{}, true))
	}
	return careexit.NewPlanRestoration(lifecycle, resync, unit, func() calendar.Date { return today })
}

func TestPlanRestorationCancelReplansEveryCancelledChildInTheUnit(t *testing.T) {
	t.Parallel()
	today := calendar.NewDate(2026, 10, 29)
	lifecycle, resync := &restorationLifecycle{}, &restorationResync{}
	cancelled, err := newTestRestoration(t, lifecycle, resync, today).Cancel(context.Background(), []int64{9, 4, 9}, 7)
	require.NoError(t, err)
	assert.Equal(t, 3, cancelled, "the lifecycle's count is returned unchanged")
	assert.True(t, lifecycle.insideUnit)
	assert.Equal(t, [][]int64{{4, 9}}, resync.calls)
	assert.Equal(t, []calendar.Date{today}, resync.from, "history before today is never re-planned")
}

func TestPlanRestorationConfirmReplansOnlyReappliedChildren(t *testing.T) {
	t.Parallel()
	today := calendar.NewDate(2026, 10, 29)
	for name, confirm := range map[string]func(*careexit.PlanRestoration) (*careplan.CareExitResult, error){
		"confirm": func(p *careexit.PlanRestoration) (*careplan.CareExitResult, error) {
			return p.Confirm(context.Background(), "token", careplan.CareExitInput{}, 7)
		},
		"withdrawal": func(p *careexit.PlanRestoration) (*careplan.CareExitResult, error) {
			return p.ConfirmWithdrawalCareEnd(context.Background(), 3, "token", careplan.CareExitInput{}, 7)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			lifecycle, resync := &restorationLifecycle{reapplied: []int64{5}}, &restorationResync{}
			result, err := confirm(newTestRestoration(t, lifecycle, resync, today))
			require.NoError(t, err)
			assert.Equal(t, 2, result.StudentsEnded)
			assert.True(t, lifecycle.insideUnit)
			assert.Equal(t, [][]int64{{5}}, resync.calls)

			first, resyncFirst := &restorationLifecycle{}, &restorationResync{}
			_, err = confirm(newTestRestoration(t, first, resyncFirst, today))
			require.NoError(t, err)
			assert.Equal(t, [][]int64{nil}, resyncFirst.calls, "a first exit hands no child to the re-plan")
		})
	}
}

func TestPlanRestorationFailsWithTheResyncAndSkipsItAfterARefusal(t *testing.T) {
	t.Parallel()
	today := calendar.NewDate(2026, 10, 29)
	resyncErr := errors.New("roster unavailable")
	cancelled, err := newTestRestoration(t, &restorationLifecycle{}, &restorationResync{err: resyncErr}, today).
		Cancel(context.Background(), []int64{4}, 7)
	require.ErrorIs(t, err, resyncErr, "a failed re-plan fails the unit so the restore rolls back")
	assert.Zero(t, cancelled)

	resync := &restorationResync{}
	_, err = newTestRestoration(t, &restorationLifecycle{err: careplan.ErrCareExitNotPlanned}, resync, today).
		Cancel(context.Background(), []int64{4}, 7)
	require.ErrorIs(t, err, careplan.ErrCareExitNotPlanned)
	assert.Empty(t, resync.calls, "a refused cancellation re-plans nothing")
}

func TestNewPlanRestorationRequiresItsCollaborators(t *testing.T) {
	t.Parallel()
	unit := func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
	assert.Panics(t, func() { careexit.NewPlanRestoration(nil, &restorationResync{}, unit, nil) })
	assert.Panics(t, func() { careexit.NewPlanRestoration(&restorationLifecycle{}, nil, unit, nil) })
	assert.Panics(t, func() { careexit.NewPlanRestoration(&restorationLifecycle{}, &restorationResync{}, nil, nil) })
}
