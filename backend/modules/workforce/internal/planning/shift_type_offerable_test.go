package planning

import (
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/require"
)

// TestShiftTypeOfferable: an archived type (#6845) is refused for a new
// assignment before the inactive check, so the client names the right reason.
func TestShiftTypeOfferable(t *testing.T) {
	t.Parallel()
	archivedAt := time.Date(2026, time.November, 13, 9, 0, 0, 0, time.UTC)

	require.NoError(t, shiftTypeOfferable(&workforce.ShiftTypeRecord{IsActive: true}))
	require.ErrorIs(t, shiftTypeOfferable(&workforce.ShiftTypeRecord{IsActive: false}), ErrShiftTypeInactive)
	require.ErrorIs(t, shiftTypeOfferable(&workforce.ShiftTypeRecord{IsActive: true, ArchivedAt: &archivedAt}), ErrShiftTypeArchived)
	require.ErrorIs(t, shiftTypeOfferable(&workforce.ShiftTypeRecord{IsActive: false, ArchivedAt: &archivedAt}), ErrShiftTypeArchived)
	require.ErrorIs(t, mapPlanningError(ErrShiftTypeArchived), workforce.ErrShiftTypeArchived)
}
