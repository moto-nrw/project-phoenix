package enrollment

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pure tests for Phase.Validate. The whole
// admin-create-phase flow runs through Validate before the DB write,
// so each branch is worth covering — broken validation here means
// either a bad row lands in the DB (CHECK constraints are a backstop,
// not the primary defence) or a legitimate row is rejected at the
// UI before the user knows why.

func validPhase() *Phase {
	return &Phase{
		Name:             "Schuljahr 2026/27",
		Kind:             PhaseKindSchoolYear,
		ServiceStartDate: timezone.NewDate(2026, 9, 1),
		ServiceEndDate:   timezone.NewDate(2027, 7, 31),
		CareOverflowMode: PhaseCareOverflowWaitlist,
	}
}

func TestPhase_Validate_HappyPath(t *testing.T) {
	assert.NoError(t, validPhase().Validate())
}

func TestPhase_Validate_RequiresName(t *testing.T) {
	p := validPhase()
	p.Name = "   "
	err := p.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestPhase_Validate_TrimsName(t *testing.T) {
	p := validPhase()
	p.Name = "  Schuljahr  "
	require.NoError(t, p.Validate())
	assert.Equal(t, "Schuljahr", p.Name)
}

func TestPhase_Validate_DefaultsKindToSchoolYear(t *testing.T) {
	p := validPhase()
	p.Kind = ""
	require.NoError(t, p.Validate())
	assert.Equal(t, PhaseKindSchoolYear, p.Kind)
}

func TestPhase_Validate_RejectsUnknownKind(t *testing.T) {
	p := validPhase()
	p.Kind = "weird"
	err := p.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kind")
}

func TestPhase_Validate_AcceptsAllKnownKinds(t *testing.T) {
	for _, kind := range []string{PhaseKindSchoolYear, PhaseKindHoliday, PhaseKindCustom} {
		p := validPhase()
		p.Kind = kind
		assert.NoError(t, p.Validate(), "kind %q must validate", kind)
	}
}

func TestPhase_Validate_RequiresServiceStart(t *testing.T) {
	p := validPhase()
	p.ServiceStartDate = timezone.Date{}
	err := p.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service_start_date")
}

func TestPhase_Validate_RequiresServiceEnd(t *testing.T) {
	p := validPhase()
	p.ServiceEndDate = timezone.Date{}
	err := p.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service_end_date")
}

func TestPhase_Validate_RejectsEndBeforeStart(t *testing.T) {
	p := validPhase()
	p.ServiceStartDate = timezone.NewDate(2027, 1, 1)
	p.ServiceEndDate = timezone.NewDate(2026, 12, 31)
	err := p.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service_end_date")
}

func TestPhase_Validate_AcceptsEqualStartAndEnd(t *testing.T) {
	// One-day phase (e.g. an info day) is legal — `Before` is strict.
	p := validPhase()
	same := timezone.NewDate(2026, 9, 1)
	p.ServiceStartDate = same
	p.ServiceEndDate = same
	assert.NoError(t, p.Validate())
}

func TestPhase_Validate_RejectsCloseBeforeOrEqualOpen(t *testing.T) {
	p := validPhase()
	open := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	closed := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	p.EnrollmentOpenAt = &open
	p.EnrollmentCloseAt = &closed
	err := p.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enrollment_close_at")
}

func TestPhase_Validate_AcceptsCloseAfterOpen(t *testing.T) {
	p := validPhase()
	open := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	closed := time.Date(2026, 8, 31, 23, 59, 0, 0, time.UTC)
	p.EnrollmentOpenAt = &open
	p.EnrollmentCloseAt = &closed
	assert.NoError(t, p.Validate())
}

func TestPhase_Validate_DefaultsCareOverflowMode(t *testing.T) {
	p := validPhase()
	p.CareOverflowMode = ""
	require.NoError(t, p.Validate())
	assert.Equal(t, PhaseCareOverflowWaitlist, p.CareOverflowMode)
}

func TestPhase_Validate_RejectsUnknownCareOverflowMode(t *testing.T) {
	p := validPhase()
	p.CareOverflowMode = "garbage"
	err := p.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "care_overflow_mode")
}

func TestPhase_Validate_AcceptsAllCareOverflowModes(t *testing.T) {
	for _, mode := range []string{PhaseCareOverflowWaitlist, PhaseCareOverflowReject, PhaseCareOverflowAllow} {
		p := validPhase()
		p.CareOverflowMode = mode
		assert.NoError(t, p.Validate(), "overflow mode %q must validate", mode)
	}
}

func TestPhase_Validate_RejectsUnknownRolloverMode(t *testing.T) {
	p := validPhase()
	mode := "weird"
	src := int64(1)
	p.RolloverMode = &mode
	p.RolloverSourcePhaseID = &src
	err := p.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollover_mode")
}

func TestPhase_Validate_AcceptsAllRolloverModes(t *testing.T) {
	src := int64(1)
	for _, mode := range []string{PhaseRolloverModeOptIn, PhaseRolloverModeOptOut} {
		p := validPhase()
		m := mode
		p.RolloverMode = &m
		p.RolloverSourcePhaseID = &src
		assert.NoError(t, p.Validate(), "rollover mode %q must validate", mode)
	}
}

func TestPhase_Validate_RolloverModeWithoutSourceRejected(t *testing.T) {
	p := validPhase()
	mode := PhaseRolloverModeOptOut
	p.RolloverMode = &mode
	// SourcePhaseID intentionally nil
	err := p.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollover_source_phase_id and rollover_mode")
}

func TestPhase_Validate_RolloverSourceWithoutModeRejected(t *testing.T) {
	p := validPhase()
	src := int64(42)
	p.RolloverSourcePhaseID = &src
	// RolloverMode intentionally nil
	err := p.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollover_source_phase_id and rollover_mode")
}

func TestPhase_Validate_RequireSchoolClassWithoutClassesRejected(t *testing.T) {
	p := validPhase()
	p.RequireSchoolClass = true
	// AvailableSchoolClasses intentionally empty (also covers a list of
	// blank entries after normalization).
	p.AvailableSchoolClasses = []string{"  "}
	err := p.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "require_school_class")
}

func TestPhase_Validate_RequireSchoolClassWithClassesAccepted(t *testing.T) {
	p := validPhase()
	p.RequireSchoolClass = true
	p.AvailableSchoolClasses = []string{"2a", "2b"}
	require.NoError(t, p.Validate())
}
