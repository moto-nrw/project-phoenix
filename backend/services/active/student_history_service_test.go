package active_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A nil slot repository (tests without a timetable) must answer the
// tenant-wide care-plan signal with false instead of panicking.
func TestStudentHistoryService_HasPlannedSlotsInRange_NilSlotRepo(t *testing.T) {
	t.Parallel()

	svc := activeService.NewStudentHistoryService(nil, nil, nil, nil)

	has, err := svc.HasPlannedSlotsInRange(context.Background(), timezone.NewDate(2032, 3, 2), timezone.NewDate(2032, 3, 6))
	require.NoError(t, err)
	assert.False(t, has, "without a slot repository there is no care-plan signal")
}
