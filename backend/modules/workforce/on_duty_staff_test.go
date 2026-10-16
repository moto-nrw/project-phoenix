package workforce_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// Only the two real statuses count as on duty: the presence map also carries
// a synthetic "checked out" value, and an unknown status must not silently
// count as working.
func TestOnDutyStaffIDsMatchesTheWorkingStatuses(t *testing.T) {
	t.Parallel()

	onDuty := workforce.OnDutyStaffIDs(map[int64]string{
		1: workforce.WorkSessionStatusPresent,
		2: workforce.WorkSessionStatusHomeOffice,
		3: "checked_out",
		4: "some_future_status",
	})

	assert.Equal(t, map[int64]struct{}{1: {}, 2: {}}, onDuty)
	assert.NotNil(t, workforce.OnDutyStaffIDs(nil), "nobody clocked in is an empty set, not \"no restriction\"")
	assert.Empty(t, workforce.OnDutyStaffIDs(nil))
}
