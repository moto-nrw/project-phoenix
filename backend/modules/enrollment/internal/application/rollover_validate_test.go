package application

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// --- validateCreateRequest ------------------------------------------------

func validRolloverReq() enrollment.CreatePhaseFromSourceRequest {
	return enrollment.CreatePhaseFromSourceRequest{
		SourcePhaseID:    42,
		Name:             "Schuljahr 2027/28",
		ServiceStartDate: calendar.NewDate(2027, 9, 1),
		ServiceEndDate:   calendar.NewDate(2028, 7, 31),
		RolloverDeadline: time.Date(2027, 7, 1, 0, 0, 0, 0, time.UTC),
		RolloverMode:     enrollment.PhaseRolloverModeOptOut,
	}
}

func TestValidateCreateRequest_HappyPath(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateCreateRequest(validRolloverReq()))
}

func TestValidateCreateRequest_RequiresSourcePhaseID(t *testing.T) {
	t.Parallel()

	r := validRolloverReq()
	r.SourcePhaseID = 0
	err := validateCreateRequest(r)
	require.Error(t, err)
	assert.ErrorIs(t, err, enrollment.ErrRolloverInvalidRequest)
	assert.Contains(t, err.Error(), "source_phase_id")
}

func TestValidateCreateRequest_RejectsNegativeSourcePhaseID(t *testing.T) {
	t.Parallel()

	r := validRolloverReq()
	r.SourcePhaseID = -1
	err := validateCreateRequest(r)
	require.Error(t, err)
}

func TestValidateCreateRequest_RequiresName(t *testing.T) {
	t.Parallel()

	r := validRolloverReq()
	r.Name = ""
	err := validateCreateRequest(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestValidateCreateRequest_RequiresServiceStartDate(t *testing.T) {
	t.Parallel()

	r := validRolloverReq()
	r.ServiceStartDate = calendar.Date("")
	err := validateCreateRequest(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service dates")
}

func TestValidateCreateRequest_RequiresServiceEndDate(t *testing.T) {
	t.Parallel()

	r := validRolloverReq()
	r.ServiceEndDate = calendar.Date("")
	err := validateCreateRequest(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service dates")
}

func TestValidateCreateRequest_RejectsEndBeforeStart(t *testing.T) {
	t.Parallel()

	r := validRolloverReq()
	r.ServiceStartDate = calendar.NewDate(2028, 1, 1)
	r.ServiceEndDate = calendar.NewDate(2027, 12, 31)
	err := validateCreateRequest(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service_end_date")
}

func TestValidateCreateRequest_AcceptsEqualStartAndEnd(t *testing.T) {
	t.Parallel()

	// Single-day rollover phase is legal — "Before" is strict, not "<=".
	r := validRolloverReq()
	same := calendar.NewDate(2027, 9, 1)
	r.ServiceStartDate = same
	r.ServiceEndDate = same
	assert.NoError(t, validateCreateRequest(r))
}

func TestValidateCreateRequest_RequiresRolloverDeadline(t *testing.T) {
	t.Parallel()

	r := validRolloverReq()
	r.RolloverDeadline = time.Time{}
	err := validateCreateRequest(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollover_deadline")
}

func TestValidateCreateRequest_RejectsUnknownRolloverMode(t *testing.T) {
	t.Parallel()

	r := validRolloverReq()
	r.RolloverMode = "weird"
	err := validateCreateRequest(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollover_mode")
}

func TestValidateCreateRequest_AcceptsBothKnownModes(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{enrollment.PhaseRolloverModeOptIn, enrollment.PhaseRolloverModeOptOut} {
		r := validRolloverReq()
		r.RolloverMode = mode
		assert.NoError(t, validateCreateRequest(r),
			"mode %q must validate", mode)
	}
}

func TestValidateCreateRequest_RejectsEmptyRolloverMode(t *testing.T) {
	t.Parallel()

	r := validRolloverReq()
	r.RolloverMode = ""
	err := validateCreateRequest(r)
	require.Error(t, err)
}

// --- isUniqueViolationOn -------------------------------------------------

// fakePgError builds a synthetic pgdriver.Error with the SQLSTATE +
// constraint-name fields the detector reads. We can't use
// pgdriver.Error{} literally because it relies on internal field
// indexing; instead, we wrap it in a wider error to confirm
// errors.As fails out gracefully when the wrapped type isn't a
