package enrollment_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/stretchr/testify/assert"
)

func windowPhase() *enrollment.Phase {
	return &enrollment.Phase{
		Name:             "Schuljahr 2026/27",
		Kind:             enrollment.PhaseKindSchoolYear,
		ServiceStartDate: "2026-09-01",
		ServiceEndDate:   "2027-07-31",
		CareOverflowMode: enrollment.PhaseCareOverflowWaitlist,
	}
}

func TestEnrollmentWindowOpen_NoBoundsAlwaysOpen(t *testing.T) {
	t.Parallel()

	p := windowPhase()
	assert.True(t, p.EnrollmentWindowOpen(time.Now()))
}

func TestEnrollmentWindowOpen_BeforeOpenIsClosed(t *testing.T) {
	t.Parallel()

	open := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	p := windowPhase()
	p.EnrollmentOpenAt = &open
	assert.False(t, p.EnrollmentWindowOpen(time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)))
}

func TestEnrollmentWindowOpen_AfterOpenIsOpen(t *testing.T) {
	t.Parallel()

	open := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	p := windowPhase()
	p.EnrollmentOpenAt = &open
	assert.True(t, p.EnrollmentWindowOpen(time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)))
}

func TestEnrollmentWindowOpen_AtCloseIsClosed(t *testing.T) {
	t.Parallel()

	// Half-open semantics — close moment is excluded from the window.
	closed := time.Date(2026, 8, 31, 23, 59, 0, 0, time.UTC)
	p := windowPhase()
	p.EnrollmentCloseAt = &closed
	assert.False(t, p.EnrollmentWindowOpen(closed))
}

func TestEnrollmentWindowOpen_BetweenBoundsIsOpen(t *testing.T) {
	t.Parallel()

	open := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	closed := time.Date(2026, 8, 31, 23, 59, 0, 0, time.UTC)
	p := windowPhase()
	p.EnrollmentOpenAt = &open
	p.EnrollmentCloseAt = &closed
	assert.True(t, p.EnrollmentWindowOpen(time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)))
}

func TestEnrollmentWindowOpen_NilPhase(t *testing.T) {
	t.Parallel()

	var missing *enrollment.Phase
	assert.False(t, missing.EnrollmentWindowOpen(time.Now()))
}
