package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/workflows/shiftplansync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The cascade is fail-closed: a composition that forgets a dependency must be
// rejected here, because a cascade that silently does nothing lets a sick
// report commit without its plan effects (#1843).
func TestNewSickCascadeRejectsAnIncompleteComposition(t *testing.T) {
	t.Parallel()

	cascade, err := NewSickCascade(SickCascadeDependencies{})
	require.Error(t, err)
	assert.Nil(t, cascade)
	assert.EqualError(t, err, "shift plan sync compose: sick cascade needs "+
		"InstanceStaff, Instances, LockStaffShifts, Planning, TimetableData, Workforce")
}

func TestNewSubstitutionRejectsAnIncompleteComposition(t *testing.T) {
	t.Parallel()

	substitution, err := NewSubstitution(SubstitutionDependencies{})
	require.Error(t, err)
	assert.Nil(t, substitution)
	assert.EqualError(t, err, "shift plan sync compose: schedule substitution needs "+
		"ActivityInstances, InstanceStaff, Instances, Staff")
}

// An unresolved deferred binding is an error on every method, never a silent
// no-op: the absence service is constructed before the cascade exists, and a
// composition that never assigns it must fail the absence write.
func TestDeferredSickCascadeFailsClosedWhileUnbound(t *testing.T) {
	t.Parallel()

	for name, cascade := range map[string]shiftplansync.SickCascade{
		"no resolver":       DeferredSickCascade(nil),
		"nothing to return": DeferredSickCascade(func() shiftplansync.SickCascade { return nil }),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			assert.ErrorIs(t, cascade.MarkSickForRange(ctx, workforce.SickCascadeInput{}), errSickCascadeUnbound)
			assert.ErrorIs(t, cascade.ClearSickForRange(ctx, workforce.SickCascadeInput{}), errSickCascadeUnbound)
			assert.ErrorIs(t, cascade.ReconcileSickRange(ctx, workforce.SickCascadeInput{}, workforce.SickCascadeInput{}), errSickCascadeUnbound)
			assert.ErrorIs(t, cascade.ReassignSickStamps(ctx, 1, 2), errSickCascadeUnbound)
		})
	}
}

func TestDeferredSickCascadeForwardsToTheResolvedCascade(t *testing.T) {
	t.Parallel()

	recorder := &recordingCascade{}
	cascade := DeferredSickCascade(func() shiftplansync.SickCascade { return recorder })
	ctx := context.Background()

	require.NoError(t, cascade.MarkSickForRange(ctx, workforce.SickCascadeInput{AbsenceID: 7}))
	require.NoError(t, cascade.ClearSickForRange(ctx, workforce.SickCascadeInput{AbsenceID: 8}))
	require.NoError(t, cascade.ReconcileSickRange(ctx, workforce.SickCascadeInput{AbsenceID: 9}, workforce.SickCascadeInput{AbsenceID: 10}))
	require.NoError(t, cascade.ReassignSickStamps(ctx, 11, 12))

	assert.Equal(t, []int64{7}, recorder.marked)
	assert.Equal(t, []int64{8}, recorder.cleared)
	assert.Equal(t, [][2]int64{{9, 10}}, recorder.reconciled)
	assert.Equal(t, [][2]int64{{11, 12}}, recorder.reassigned)
}

type recordingCascade struct {
	marked     []int64
	cleared    []int64
	reconciled [][2]int64
	reassigned [][2]int64
}

func (r *recordingCascade) MarkSickForRange(_ context.Context, in workforce.SickCascadeInput) error {
	r.marked = append(r.marked, in.AbsenceID)
	return nil
}

func (r *recordingCascade) ClearSickForRange(_ context.Context, in workforce.SickCascadeInput) error {
	r.cleared = append(r.cleared, in.AbsenceID)
	return nil
}

func (r *recordingCascade) ReconcileSickRange(_ context.Context, before, after workforce.SickCascadeInput) error {
	r.reconciled = append(r.reconciled, [2]int64{before.AbsenceID, after.AbsenceID})
	return nil
}

func (r *recordingCascade) ReassignSickStamps(_ context.Context, fromAbsenceID, toAbsenceID int64) error {
	r.reassigned = append(r.reassigned, [2]int64{fromAbsenceID, toAbsenceID})
	return nil
}
