package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// Pure-helper tests for the rollover service. computeNewGrade and
// renewalInitialStatus drive the bulk of the rollover dispatch, so
// they're worth covering without firing up the test DB.

func grade(n int16) *int16 { return &n }

func TestComputeNewGrade_NilSourceNeedsReview(t *testing.T) {
	t.Parallel()

	got, reason := computeNewGrade(nil, true, 4)
	assert.Nil(t, got, "no source grade → no new grade")
	assert.Equal(t, enrollmentModels.ReviewReasonNoGradeLevel, reason)
}

func TestComputeNewGrade_BumpAdvancesByOne(t *testing.T) {
	t.Parallel()

	got, reason := computeNewGrade(grade(2), true, 4)
	require.NotNil(t, got)
	assert.Equal(t, int16(3), *got)
	assert.Empty(t, reason, "in-range bump must not request review")
}

func TestComputeNewGrade_HalfYearKeepsGrade(t *testing.T) {
	t.Parallel()

	got, reason := computeNewGrade(grade(2), false, 4)
	require.NotNil(t, got)
	assert.Equal(t, int16(2), *got, "bumpsGrade=false preserves the grade")
	assert.Empty(t, reason)
}

func TestComputeNewGrade_AboveMaxSurfacesReview(t *testing.T) {
	t.Parallel()

	got, reason := computeNewGrade(grade(4), true, 4)
	require.NotNil(t, got)
	assert.Equal(t, int16(5), *got, "row still carries the would-be grade")
	assert.Equal(t, enrollmentModels.ReviewReasonGradeAboveMax, reason,
		"out-of-range bump must request admin review")
}

func TestComputeNewGrade_AtMaxOK(t *testing.T) {
	t.Parallel()

	got, reason := computeNewGrade(grade(3), true, 4)
	require.NotNil(t, got)
	assert.Equal(t, int16(4), *got)
	assert.Empty(t, reason)
}

func TestRenewalInitialStatus_OptInIsPendingRenewal(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		enrollmentModels.ChildStatusPendingRenewal,
		renewalInitialStatus(enrollmentModels.PhaseRolloverModeOptIn),
	)
}

func TestRenewalInitialStatus_OptOutIsAutoRenewed(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		enrollmentModels.ChildStatusAutoRenewed,
		renewalInitialStatus(enrollmentModels.PhaseRolloverModeOptOut),
	)
}

func TestRenewalInitialStatus_UnknownDefaultsToAutoRenewed(t *testing.T) {
	t.Parallel()

	// Defensive: unknown mode falls into the auto_renewed branch so
	// the rollover dispatch doesn't strand a child in an invalid state.
	assert.Equal(t,
		enrollmentModels.ChildStatusAutoRenewed,
		renewalInitialStatus("weird-mode"),
	)
}
