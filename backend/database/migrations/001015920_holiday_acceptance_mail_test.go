package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHolidayAcceptanceErasureIsTheErasureOf918WithTheMail keeps the erasure
// of 1.15.920 the one of 1.15.918 whose statement for the mails of a
// registration also names the acceptance mail of #6672.
func TestHolidayAcceptanceErasureIsTheErasureOf918WithTheMail(t *testing.T) {
	t.Parallel()
	require.Equal(t, 1, strings.Count(demoVisitorErasureEmptyRunsSQL, holidayAcceptanceErasureBefore),
		"the statement for the mails of a registration is in the erasure of 1.15.918 exactly once")
	assert.Equal(t, strings.Replace(demoVisitorErasureEmptyRunsSQL, holidayAcceptanceErasureBefore, holidayAcceptanceErasureAfter, 1),
		demoVisitorErasureHolidayAcceptanceSQL)
	assert.Contains(t, demoVisitorErasureHolidayAcceptanceSQL, "'holiday_registration_accepted'")
}

// TestHolidayAcceptanceRollbackRestoresTheCheckOf887 keeps the CHECK a
// rollback writes back the one of 1.15.887.
func TestHolidayAcceptanceRollbackRestoresTheCheckOf887(t *testing.T) {
	t.Parallel()
	assert.Equal(t, holidayClosureEmailOutboxTraegerKindCheckSQL, holidayAcceptanceRestoredKindCheckSQL)
	assert.Equal(t, validateHolidayClosureKindCheckSQL, validateHolidayAcceptanceKindCheckSQL)
}
