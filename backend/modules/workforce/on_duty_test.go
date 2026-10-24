package workforce_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

func TestOnDutyStaffIDsCountsPresenceAndHomeOffice(t *testing.T) {
	t.Parallel()

	onDuty := workforce.OnDutyStaffIDs(map[int64]string{
		30: workforce.WorkSessionStatusHomeOffice,
		10: workforce.WorkSessionStatusPresent,
		20: "checked_out",
		40: "",
	})

	assert.Equal(t, map[int64]struct{}{10: {}, 30: {}}, onDuty, "present and home office are on duty")
	assert.Empty(t, workforce.OnDutyStaffIDs(nil), "nobody checked in is nobody on duty")
}
