package enrollment_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// --- ListPublicOpen: audience filtering (#1663) ------------------------

func TestPhaseRepository_ListPublicOpen_HidesLinkedParentsPhases(t *testing.T) {
	db, repo, tenantID := setupPhaseRepoTest(t)
	openName := uniquePhaseName("audienceOpen")
	newStudentsName := uniquePhaseName("audienceNew")
	linkedName := uniquePhaseName("audienceLinked")
	defer wipePhases(db, tenantID, openName, newStudentsName, linkedName)

	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

	pOpen := makeValidPhase(openName)
	pOpen.Audience = enrollmentModels.PhaseAudienceOpen

	pNew := makeValidPhase(newStudentsName)
	pNew.Audience = enrollmentModels.PhaseAudienceNewStudents

	pLinked := makeValidPhase(linkedName)
	pLinked.Audience = enrollmentModels.PhaseAudienceLinkedParents

	for _, p := range []*enrollmentModels.Phase{pOpen, pNew, pLinked} {
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.Create(ctx, p)
		}))
	}

	var list []*enrollmentModels.Phase
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListPublicOpen(ctx, now)
		return lErr
	}))

	ours := make(map[string]string)
	for _, p := range list {
		ours[p.Name] = p.Audience
	}
	assert.Contains(t, ours, openName, "open phase MUST appear in the public listing")
	assert.Contains(t, ours, newStudentsName, "new_students phase MUST appear in the public listing")
	assert.NotContains(t, ours, linkedName,
		"linked_parents phase MUST be hidden from the anonymous public listing")
}

// --- Create/Update roundtrip of the eligibility columns ----------------

func TestPhaseRepository_EligibilityColumnsRoundtrip(t *testing.T) {
	db, repo, tenantID := setupPhaseRepoTest(t)
	name := uniquePhaseName("eligibilityRoundtrip")
	defer wipePhases(db, tenantID, name)

	p := makeValidPhase(name)
	p.Audience = enrollmentModels.PhaseAudienceNewStudents
	// Eligible classes must also be offered by the phase (#1663), so seed the
	// pick list with every class the eligibility list (and the later update)
	// references.
	p.AvailableSchoolClasses = []string{"2a", "3b", "4c"}
	p.EligibleSchoolClasses = []string{"2a", "3b"}

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, p)
	}))

	var loaded *enrollmentModels.Phase
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fErr error
		loaded, fErr = repo.FindByID(ctx, p.ID)
		return fErr
	}))
	assert.Equal(t, enrollmentModels.PhaseAudienceNewStudents, loaded.Audience)
	assert.Equal(t, []string{"2a", "3b"}, loaded.EligibleSchoolClasses)

	// Update must persist both columns (they are explicit Set() entries
	// in the update query — a missing Set silently keeps stale values).
	loaded.Audience = enrollmentModels.PhaseAudienceLinkedParents
	loaded.EligibleSchoolClasses = []string{"4c"}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Update(ctx, loaded)
	}))

	var reloaded *enrollmentModels.Phase
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fErr error
		reloaded, fErr = repo.FindByID(ctx, loaded.ID)
		return fErr
	}))
	assert.Equal(t, enrollmentModels.PhaseAudienceLinkedParents, reloaded.Audience)
	assert.Equal(t, []string{"4c"}, reloaded.EligibleSchoolClasses)
}

// The grade restriction is a jsonb int array — same Set()-must-exist trap as
// the columns above, plus the driver has to bind []int as JSON rather than a
// Postgres array (#1663).
func TestPhaseRepository_EligibleGradeLevelsRoundtrip(t *testing.T) {
	db, repo, tenantID := setupPhaseRepoTest(t)
	name := uniquePhaseName("gradeEligibilityRoundtrip")
	defer wipePhases(db, tenantID, name)

	p := makeValidPhase(name)
	p.EligibleGradeLevels = []int{3, 4}

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, p)
	}))

	var loaded *enrollmentModels.Phase
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fErr error
		loaded, fErr = repo.FindByID(ctx, p.ID)
		return fErr
	}))
	assert.Equal(t, []int{3, 4}, loaded.EligibleGradeLevels)

	loaded.EligibleGradeLevels = []int{1}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Update(ctx, loaded)
	}))

	var reloaded *enrollmentModels.Phase
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fErr error
		reloaded, fErr = repo.FindByID(ctx, loaded.ID)
		return fErr
	}))
	assert.Equal(t, []int{1}, reloaded.EligibleGradeLevels,
		"update must persist the grade restriction (a missing Set() keeps stale values)")
}

// Backs the settings guard that refuses to disable grade-level collection
// while a phase would then reject every submission (#1663).
func TestPhaseRepository_ExistsActiveWithEligibleGradeLevels(t *testing.T) {
	db, repo, tenantID := setupPhaseRepoTest(t)
	unrestrictedName := uniquePhaseName("gradeGuardUnrestricted")
	inactiveName := uniquePhaseName("gradeGuardInactive")
	activeName := uniquePhaseName("gradeGuardActive")
	defer wipePhases(db, tenantID, unrestrictedName, inactiveName, activeName)

	unrestricted := makeValidPhase(unrestrictedName)
	unrestricted.IsActive = true

	inactive := makeValidPhase(inactiveName)
	inactive.IsActive = false
	inactive.EligibleGradeLevels = []int{2}

	for _, p := range []*enrollmentModels.Phase{unrestricted, inactive} {
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.Create(ctx, p)
		}))
	}

	var exists bool
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var eErr error
		exists, eErr = repo.ExistsActiveWithEligibleGradeLevels(ctx)
		return eErr
	}))
	assert.False(t, exists, "an unrestricted or inactive phase must not block the toggle")

	active := makeValidPhase(activeName)
	active.IsActive = true
	active.EligibleGradeLevels = []int{3}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, active)
	}))

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var eErr error
		exists, eErr = repo.ExistsActiveWithEligibleGradeLevels(ctx)
		return eErr
	}))
	assert.True(t, exists, "an active grade-restricted phase must block the toggle")
}
