package enrollment

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// --- validateCreateRequest ------------------------------------------------

func validRolloverReq() CreatePhaseFromSourceRequest {
	return CreatePhaseFromSourceRequest{
		SourcePhaseID:    42,
		Name:             "Schuljahr 2027/28",
		ServiceStartDate: timezone.NewDate(2027, 9, 1),
		ServiceEndDate:   timezone.NewDate(2028, 7, 31),
		RolloverDeadline: time.Date(2027, 7, 1, 0, 0, 0, 0, time.UTC),
		RolloverMode:     enrollmentModels.PhaseRolloverModeOptOut,
	}
}

func newSvc() *rolloverService { return &rolloverService{} }

func TestValidateCreateRequest_HappyPath(t *testing.T) {
	t.Parallel()

	require.NoError(t, newSvc().validateCreateRequest(validRolloverReq()))
}

func TestValidateCreateRequest_RequiresSourcePhaseID(t *testing.T) {
	t.Parallel()

	r := validRolloverReq()
	r.SourcePhaseID = 0
	err := newSvc().validateCreateRequest(r)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRolloverInvalidRequest)
	assert.Contains(t, err.Error(), "source_phase_id")
}

func TestValidateCreateRequest_RejectsNegativeSourcePhaseID(t *testing.T) {
	t.Parallel()

	r := validRolloverReq()
	r.SourcePhaseID = -1
	err := newSvc().validateCreateRequest(r)
	require.Error(t, err)
}

func TestValidateCreateRequest_RequiresName(t *testing.T) {
	t.Parallel()

	r := validRolloverReq()
	r.Name = ""
	err := newSvc().validateCreateRequest(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestValidateCreateRequest_RequiresServiceStartDate(t *testing.T) {
	t.Parallel()

	r := validRolloverReq()
	r.ServiceStartDate = timezone.Date{}
	err := newSvc().validateCreateRequest(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service dates")
}

func TestValidateCreateRequest_RequiresServiceEndDate(t *testing.T) {
	t.Parallel()

	r := validRolloverReq()
	r.ServiceEndDate = timezone.Date{}
	err := newSvc().validateCreateRequest(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service dates")
}

func TestValidateCreateRequest_RejectsEndBeforeStart(t *testing.T) {
	t.Parallel()

	r := validRolloverReq()
	r.ServiceStartDate = timezone.NewDate(2028, 1, 1)
	r.ServiceEndDate = timezone.NewDate(2027, 12, 31)
	err := newSvc().validateCreateRequest(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service_end_date")
}

func TestValidateCreateRequest_AcceptsEqualStartAndEnd(t *testing.T) {
	t.Parallel()

	// Single-day rollover phase is legal — "Before" is strict, not "<=".
	r := validRolloverReq()
	same := timezone.NewDate(2027, 9, 1)
	r.ServiceStartDate = same
	r.ServiceEndDate = same
	assert.NoError(t, newSvc().validateCreateRequest(r))
}

func TestValidateCreateRequest_RequiresRolloverDeadline(t *testing.T) {
	t.Parallel()

	r := validRolloverReq()
	r.RolloverDeadline = time.Time{}
	err := newSvc().validateCreateRequest(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollover_deadline")
}

func TestValidateCreateRequest_RejectsUnknownRolloverMode(t *testing.T) {
	t.Parallel()

	r := validRolloverReq()
	r.RolloverMode = "weird"
	err := newSvc().validateCreateRequest(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollover_mode")
}

func TestValidateCreateRequest_AcceptsBothKnownModes(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{enrollmentModels.PhaseRolloverModeOptIn, enrollmentModels.PhaseRolloverModeOptOut} {
		r := validRolloverReq()
		r.RolloverMode = mode
		assert.NoError(t, newSvc().validateCreateRequest(r),
			"mode %q must validate", mode)
	}
}

func TestValidateCreateRequest_RejectsEmptyRolloverMode(t *testing.T) {
	t.Parallel()

	r := validRolloverReq()
	r.RolloverMode = ""
	err := newSvc().validateCreateRequest(r)
	require.Error(t, err)
}

// --- isUniqueViolationOn -------------------------------------------------

// fakePgError builds a synthetic pgdriver.Error with the SQLSTATE +
// constraint-name fields the detector reads. We can't use
// pgdriver.Error{} literally because it relies on internal field
// indexing; instead, we wrap it in a wider error to confirm
// errors.As fails out gracefully when the wrapped type isn't a
// pgdriver.Error at all.

func TestIsUniqueViolationOn_NilError(t *testing.T) {
	t.Parallel()

	assert.False(t, base.IsUniqueViolationOn(nil, "anything"))
}

func TestIsUniqueViolationOn_NonPGError(t *testing.T) {
	t.Parallel()

	// errors.As against a plain error must fail cleanly — no panic, no
	// false positive.
	assert.False(t, base.IsUniqueViolationOn(errors.New("synthetic"), "anything"))
}

func TestIsUniqueViolationOn_WrappedNonPGError(t *testing.T) {
	t.Parallel()

	wrapped := errors.New("synthetic")
	wrapped2 := errors.Join(wrapped, errors.New("layer"))
	assert.False(t, base.IsUniqueViolationOn(wrapped2, "anything"))
}

// Composite test for the two named-constraint helpers — they're
// trivial wrappers but ensure the detector wiring is correct.

func TestIsPhaseDuplicateName_NilFalse(t *testing.T) {
	t.Parallel()

	assert.False(t, isPhaseDuplicateName(nil))
}

func TestIsPhaseDuplicateName_NonPGFalse(t *testing.T) {
	t.Parallel()

	assert.False(t, isPhaseDuplicateName(errors.New("synthetic")))
}

func TestIsRolloverSourceAlreadyRolled_NilFalse(t *testing.T) {
	t.Parallel()

	assert.False(t, isRolloverSourceAlreadyRolled(nil))
}

func TestIsRolloverSourceAlreadyRolled_NonPGFalse(t *testing.T) {
	t.Parallel()

	assert.False(t, isRolloverSourceAlreadyRolled(errors.New("synthetic")))
}

// pgdriver.Error has unexported fields and isn't directly constructable
// with arbitrary metadata, but we can confirm a pgdriver.Error with the
// *wrong* code doesn't match by sentinel comparison — using
// errors.As lookup against a wrapped chain.
func TestIsUniqueViolationOn_PgErrorAsTypeAssertHandled(t *testing.T) {
	t.Parallel()

	// Build a wrapped chain that contains a pgdriver.Error zero value.
	// pgdriver.Error{}.Field('C') returns "" by default, so this should
	// fail the "23505" check even though errors.As succeeds.
	zeroPG := pgdriver.Error{}
	wrapped := errors.Join(errors.New("outer"), zeroPG)
	assert.False(t, base.IsUniqueViolationOn(wrapped, phaseNameUniqueConstraint))
}
