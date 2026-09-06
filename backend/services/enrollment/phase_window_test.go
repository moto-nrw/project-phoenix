package enrollment_test

import (
	"testing"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/stretchr/testify/assert"
)

func windowPhase() *enrollmentModels.Phase {
	return &enrollmentModels.Phase{
		Name:             "Schuljahr 2026/27",
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: "2026-09-01",
		ServiceEndDate:   "2027-07-31",
		CareOverflowMode: enrollmentModels.PhaseCareOverflowWaitlist,
	}
}

func TestIsEnrollmentWindowOpen_NoBoundsAlwaysOpen(t *testing.T) {
	t.Parallel()

	p := windowPhase()
	assert.True(t, enrollmentService.IsEnrollmentWindowOpen(p, time.Now()))
}

func TestIsEnrollmentWindowOpen_BeforeOpenIsClosed(t *testing.T) {
	t.Parallel()

	open := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	p := windowPhase()
	p.EnrollmentOpenAt = &open
	assert.False(t, enrollmentService.IsEnrollmentWindowOpen(p, time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)))
}

func TestIsEnrollmentWindowOpen_AfterOpenIsOpen(t *testing.T) {
	t.Parallel()

	open := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	p := windowPhase()
	p.EnrollmentOpenAt = &open
	assert.True(t, enrollmentService.IsEnrollmentWindowOpen(p, time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)))
}

func TestIsEnrollmentWindowOpen_AtCloseIsClosed(t *testing.T) {
	t.Parallel()

	// Half-open semantics — close moment is excluded from the window.
	closed := time.Date(2026, 8, 31, 23, 59, 0, 0, time.UTC)
	p := windowPhase()
	p.EnrollmentCloseAt = &closed
	assert.False(t, enrollmentService.IsEnrollmentWindowOpen(p, closed))
}

func TestIsEnrollmentWindowOpen_BetweenBoundsIsOpen(t *testing.T) {
	t.Parallel()

	open := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	closed := time.Date(2026, 8, 31, 23, 59, 0, 0, time.UTC)
	p := windowPhase()
	p.EnrollmentOpenAt = &open
	p.EnrollmentCloseAt = &closed
	assert.True(t, enrollmentService.IsEnrollmentWindowOpen(p, time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)))
}

func TestIsEnrollmentWindowOpen_NilPhase(t *testing.T) {
	t.Parallel()

	assert.False(t, enrollmentService.IsEnrollmentWindowOpen(nil, time.Now()))
}
