package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/require"
)

// sickCascadeStub stands in for the shift-plan-sync cascade; the constructor
// checks below never call it.
type sickCascadeStub struct{}

func (sickCascadeStub) MarkSickForRange(context.Context, workforce.SickCascadeInput) error {
	return nil
}

func (sickCascadeStub) ClearSickForRange(context.Context, workforce.SickCascadeInput) error {
	return nil
}

func (sickCascadeStub) ReconcileSickRange(context.Context, workforce.SickCascadeInput, workforce.SickCascadeInput) error {
	return nil
}

func (sickCascadeStub) ReassignSickStamps(context.Context, int64, int64) error {
	return nil
}

func TestWorkSessionSideTakesTheSickCascadeOnlyAtCompletion(t *testing.T) {
	t.Parallel()

	_, err := NewWorkSessionSide(nil, TimeTrackingPorts{ShiftPlanSync: sickCascadeStub{}}, nil)
	require.EqualError(t, err, "time tracking: the sick cascade is passed to WithSickCascade, not in the ports")
}

func TestWorkSessionSideRequiresTheCapabilityAndTheStaffLookup(t *testing.T) {
	t.Parallel()

	_, err := NewWorkSessionSide(nil, TimeTrackingPorts{}, nil)
	require.EqualError(t, err, "time tracking: the Workforce capability and the staff lookup are required")
}

func TestWithSickCascadeRequiresTheCascade(t *testing.T) {
	t.Parallel()

	_, err := (&WorkSessionSide{}).WithSickCascade(nil)
	require.EqualError(t, err, "time tracking: the sick cascade is required")
}
