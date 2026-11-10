package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The erasure of 1.15.887 is the erasure of 1.15.883 whose statement for the
// mails of a registration also deletes the closure notice (#6434), and
// nothing else changed.
func TestHolidayClosureErasureIsTheErasureOf883WithTheNotice(t *testing.T) {
	t.Parallel()
	require.Equal(t, 1, strings.Count(demoVisitorErasureCareNoticeLettersSQL, holidayClosureErasureBefore),
		"the statement for the mails of a registration is in the erasure of 1.15.883 exactly once")
	assert.Equal(t, strings.Replace(demoVisitorErasureCareNoticeLettersSQL, holidayClosureErasureBefore, holidayClosureErasureAfter, 1),
		demoVisitorErasureHolidayClosureSQL)
	assert.Contains(t, demoVisitorErasureHolidayClosureSQL, "'holiday_closure_notice'")
}
