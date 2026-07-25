package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

func windowPhase(kind string, start, end timezone.Date) *enrollmentModels.Phase {
	return &enrollmentModels.Phase{
		Kind:             kind,
		ServiceStartDate: start,
		ServiceEndDate:   end,
	}
}

func datePtr(d timezone.Date) *timezone.Date { return &d }

// A school-year phase IS the child's new master window: it replaces whatever
// the previous year left behind, in both directions.
func TestRenewedEnrollmentWindow_SchoolYearReplaces(t *testing.T) {
	phase := windowPhase(enrollmentModels.PhaseKindSchoolYear,
		timezone.NewDate(2027, 9, 1), timezone.NewDate(2028, 7, 31))

	from, until := renewedEnrollmentWindow(phase,
		datePtr(timezone.NewDate(2026, 9, 1)), datePtr(timezone.NewDate(2027, 7, 31)))

	assert.Equal(t, timezone.NewDate(2027, 9, 1), from)
	assert.Equal(t, timezone.NewDate(2028, 7, 31), until)
}

// #1663: a holiday (or custom) phase describes a limited care period, not the
// child's school membership. Overwriting an annually enrolled child's window
// with it would let the deactivation scheduler mark them inactive when the
// holiday ends.
func TestRenewedEnrollmentWindow_HolidayNeverTruncatesActiveWindow(t *testing.T) {
	phase := windowPhase(enrollmentModels.PhaseKindHoliday,
		timezone.NewDate(2027, 10, 11), timezone.NewDate(2027, 10, 22))

	from, until := renewedEnrollmentWindow(phase,
		datePtr(timezone.NewDate(2027, 9, 1)), datePtr(timezone.NewDate(2028, 7, 31)))

	assert.Equal(t, timezone.NewDate(2027, 9, 1), from, "must keep the earlier school-year start")
	assert.Equal(t, timezone.NewDate(2028, 7, 31), until, "must keep the later school-year end")
}

// The window is still WIDENED where the phase reaches beyond it, so a child
// whose window already expired is covered for the booked period.
func TestRenewedEnrollmentWindow_CustomWidensExpiredWindow(t *testing.T) {
	phase := windowPhase(enrollmentModels.PhaseKindCustom,
		timezone.NewDate(2027, 8, 1), timezone.NewDate(2027, 8, 20))

	from, until := renewedEnrollmentWindow(phase,
		datePtr(timezone.NewDate(2026, 9, 1)), datePtr(timezone.NewDate(2027, 7, 31)))

	assert.Equal(t, timezone.NewDate(2026, 9, 1), from)
	assert.Equal(t, timezone.NewDate(2027, 8, 20), until, "phase end reaches past the old window")
}

// A legacy student without a stored window takes the phase's dates whatever
// the kind is — there is nothing to preserve.
func TestRenewedEnrollmentWindow_MissingBoundsTakePhaseDates(t *testing.T) {
	phase := windowPhase(enrollmentModels.PhaseKindHoliday,
		timezone.NewDate(2027, 10, 11), timezone.NewDate(2027, 10, 22))

	from, until := renewedEnrollmentWindow(phase, nil, nil)

	assert.Equal(t, timezone.NewDate(2027, 10, 11), from)
	assert.Equal(t, timezone.NewDate(2027, 10, 22), until)
}
