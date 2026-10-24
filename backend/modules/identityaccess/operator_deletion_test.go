package identityaccess_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

func TestOperatorDeletionDueIsTheSecondNewYearAfterTheDeactivation(t *testing.T) {
	t.Parallel()
	newYear2028 := time.Date(2028, time.January, 1, 0, 0, 0, 0, calendar.Berlin)
	assert.True(t, newYear2028.Equal(identityaccess.OperatorDeletionDue(time.Date(2026, time.March, 1, 10, 0, 0, 0, calendar.Berlin))))
	assert.True(t, newYear2028.Equal(identityaccess.OperatorDeletionDue(time.Date(2026, time.December, 31, 23, 59, 0, 0, calendar.Berlin))))
	assert.True(t, newYear2028.Equal(identityaccess.OperatorDeletionDue(time.Date(2025, time.December, 31, 23, 30, 0, 0, time.UTC))),
		"23:30 UTC on 31 December is already New Year in Berlin")
}
