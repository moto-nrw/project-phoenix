package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Pin the wire values for the smaller enrollment-domain models that
// have no validators or behavior — only table-name + status/key
// constants the rest of the system string-compares against.

// --- Request -------------------------------------------------------------

func TestRequestStatus_StableValues(t *testing.T) {
	t.Parallel()

	// Derived statuses — the request service computes one of these from
	// the per-child set. The admin list page filters on them by string,
	// so a rename here would silently break the queue filter.
	assert.Equal(t, "submitted", RequestStatusSubmitted)
	assert.Equal(t, "under_review", RequestStatusUnderReview)
	assert.Equal(t, "partial", RequestStatusPartial)
	assert.Equal(t, "finalized", RequestStatusFinalized)
	assert.Equal(t, "withdrawn", RequestStatusWithdrawn)
}

// --- Phase rollover-mode constants --------------------------------------

func TestPhaseRolloverMode_StableValues(t *testing.T) {
	t.Parallel()

	// The frontend renders Rollover-Modus dropdown options with these
	// string values; backend rolloverService.validateCreateRequest
	// rejects anything else. Renaming here is a cross-stack break.
	assert.Equal(t, "opt_in", PhaseRolloverModeOptIn)
	assert.Equal(t, "opt_out", PhaseRolloverModeOptOut)
}

func TestPhaseKind_StableValues(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "school_year", PhaseKindSchoolYear)
	assert.Equal(t, "holiday", PhaseKindHoliday)
	assert.Equal(t, "custom", PhaseKindCustom)
}

func TestPhaseCareOverflowMode_StableValues(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "waitlist", PhaseCareOverflowWaitlist)
	assert.Equal(t, "reject", PhaseCareOverflowReject)
	assert.Equal(t, "allow", PhaseCareOverflowAllow)
}

// --- CareOffering days-of-week-mode constants ---------------------------

func TestDaysOfWeekMode_StableValues(t *testing.T) {
	t.Parallel()

	// "fixed" = admin-pinned days; "parent_choice" = parent picks from
	// AvailableDays on submission. The frontend submit form branches on
	// this string to show the day picker; the backend submit service's
	// resolveManualSelectedDays enforces it.
	assert.Equal(t, "fixed", DaysOfWeekModeFixed)
	assert.Equal(t, "parent_choice", DaysOfWeekModeParentChoice)
}
