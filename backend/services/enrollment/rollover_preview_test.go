package enrollment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// seedChildWithStatus submits one child and forces it into the given
// status. Mirrors seedApprovedChild but for the non-approved cohort the
// preview must report as excluded.
func seedChildWithStatus(t *testing.T, env *rolloverTestEnv, phaseID int64, email, first, last string, grade int16, status string) *enrollmentModels.RequestChild {
	t.Helper()
	child := seedApprovedChild(t, env, phaseID, "Eltern", last, email, first, last, grade)
	if status == enrollmentModels.ChildStatusApproved {
		return child
	}
	ctx := testpkg.Ctx(t)
	require.NoError(t, env.repos.RequestChild.UpdateStatus(ctx, child.ID, status, nil, env.creatorID))
	updated, err := env.repos.RequestChild.FindByID(ctx, child.ID)
	require.NoError(t, err)
	return updated
}

func TestRolloverService_PreviewPhaseFromSource_CountsCarriedExcludedReview(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	// Two approved children under separate requests: grade 3 rolls, grade 4
	// with bump lands above the max (4) and needs review.
	seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Ahrens", "anna.preview@example.com", "Lena", "Ahrens", 3)
	seedApprovedChild(t, env, env.sourcePhase.ID, "Bernd", "Berg", "bernd.preview@example.com", "Tim", "Berg", 4)
	// Not approved: never silently carried, must show up as excluded.
	seedChildWithStatus(t, env, env.sourcePhase.ID, "carla.preview@example.com", "Mia", "Clasen", 2, enrollmentModels.ChildStatusSubmitted)
	seedChildWithStatus(t, env, env.sourcePhase.ID, "dora.preview@example.com", "Ole", "Dahl", 2, enrollmentModels.ChildStatusWithdrawn)

	preview, err := env.rolloverSvc.PreviewPhaseFromSource(ctx, env.sourcePhase.ID, true)
	require.NoError(t, err)

	assert.Equal(t, 2, preview.CarryCandidateCount)
	assert.Equal(t, 1, preview.CarriedCount)
	assert.Equal(t, 1, preview.ReviewCount)
	assert.Equal(t, map[string]int{enrollmentModels.ReviewReasonGradeAboveMax: 1}, preview.ReviewByReason)
	assert.Equal(t, 2, preview.ExcludedCount)
	assert.Equal(t, map[string]int{
		enrollmentModels.ChildStatusSubmitted: 1,
		enrollmentModels.ChildStatusWithdrawn: 1,
	}, preview.ExcludedByStatus)
	assert.Equal(t, 2, preview.RequestCount)
}

func TestRolloverService_PreviewPhaseFromSource_NoGradeBumpKeepsAllCarried(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Ahrens", "anna.keep@example.com", "Lena", "Ahrens", 3)
	seedApprovedChild(t, env, env.sourcePhase.ID, "Bernd", "Berg", "bernd.keep@example.com", "Tim", "Berg", 4)

	preview, err := env.rolloverSvc.PreviewPhaseFromSource(ctx, env.sourcePhase.ID, false)
	require.NoError(t, err)

	assert.Equal(t, 2, preview.CarriedCount)
	assert.Equal(t, 0, preview.ReviewCount)
	assert.Equal(t, 0, preview.ExcludedCount)
}

func TestRolloverService_PreviewPhaseFromSource_IsReadOnly(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Ahrens", "anna.readonly@example.com", "Lena", "Ahrens", 3)

	_, err := env.rolloverSvc.PreviewPhaseFromSource(ctx, env.sourcePhase.ID, false)
	require.NoError(t, err)

	// The preview must not mark the source as rolled or create any rows —
	// a real rollover right after it must still succeed.
	req := validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptIn, false)
	req.ServiceStartDate = timezone.NewDate(2027, 2, 1)
	req.ServiceEndDate = timezone.NewDate(2027, 7, 31)
	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, 1, result.RolledCount)
}

func TestRolloverService_PreviewPhaseFromSource_RejectsAlreadyRolledSource(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Ahrens", "anna.rolled@example.com", "Lena", "Ahrens", 3)
	_, err := env.rolloverSvc.CreatePhaseFromSource(ctx, validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true))
	require.NoError(t, err)

	_, err = env.rolloverSvc.PreviewPhaseFromSource(ctx, env.sourcePhase.ID, true)
	require.ErrorIs(t, err, enrollmentService.ErrRolloverSourceAlreadyRolled)
}
