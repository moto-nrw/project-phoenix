package legacy

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
)

// The per-row urgency and past flags use the day the projection resolved
// once for the whole call, not a second clock reading, so the rows and the
// queue phase cannot fall on different days around midnight.
func TestReviewDayPrefersTheResolvedUrgencyDate(t *testing.T) {
	t.Parallel()
	clock := func() timezone.Date { return timezone.NewDate(2026, 9, 12) }

	resolved := reviewDay(requestreview.QueueFilter{UrgentDate: "2026-09-11"}, clock)
	assert.Equal(t, timezone.NewDate(2026, 9, 11), resolved)

	assert.Equal(t, clock(), reviewDay(requestreview.QueueFilter{}, clock), "a filter without a resolved day falls back to the clock")
}
