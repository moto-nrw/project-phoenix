package enrollment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// The retention statement first narrows to the periods that ended more than
// the health retention ago and looks for expired registrations only among
// them. A registration that keeps fewer days would stay too long.
func TestNoHolidayRegistrationKeepsLessThanItsHealthInformation(t *testing.T) {
	t.Parallel()
	assert.GreaterOrEqual(t, enrollment.HolidayRetentionDaysMin, enrollment.HolidayHealthRetentionDays)
	assert.GreaterOrEqual(t, enrollment.HolidayLegacyRetentionDays, enrollment.HolidayHealthRetentionDays)
}
