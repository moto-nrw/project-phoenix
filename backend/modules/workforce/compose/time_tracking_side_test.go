package compose

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorkSessionSideRequiresTheCapabilityAndTheStaffLookup(t *testing.T) {
	t.Parallel()

	_, err := NewWorkSessionSide(nil, TimeTrackingPorts{}, nil)
	require.EqualError(t, err, "time tracking: the Workforce capability and the staff lookup are required")
}

func TestTimeTrackingRequiresTheClock(t *testing.T) {
	t.Parallel()

	_, err := (&WorkSessionSide{}).TimeTracking()
	require.EqualError(t, err, "time tracking: the clock is required")
}
