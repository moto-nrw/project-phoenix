package parent_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// insertEnrollablePhaseWithAudience mirrors insertEnrollablePhase but
// sets the audience column (#1663).
func insertEnrollablePhaseWithAudience(t *testing.T, db *bun.DB, tenantID int64, name, audience string) int64 {
	t.Helper()
	bg := context.Background()
	var id int64
	err := db.NewRaw(`
		INSERT INTO enrollment.phases
		  (tenant_id, name, kind, service_start_date, service_end_date,
		   is_active, care_overflow_mode, show_status_reason_to_parent, audience)
		VALUES (?, ?, 'school_year', '2026-09-01', '2027-07-31',
		        TRUE, 'waitlist', FALSE, ?)
		RETURNING id
	`, tenantID, name, audience).Scan(bg, &id)
	require.NoError(t, err)
	return id
}

// revokeGuardianSubmitPermission empties the permissions map on every
// guardian link of the chain's student, simulating an admin-side
// revocation of parent_portal.enrollment.submit.
func revokeGuardianSubmitPermission(t *testing.T, db *bun.DB, studentID int64) {
	t.Helper()
	_, err := db.NewRaw(`
		UPDATE users.students_guardians SET permissions = '{}'::jsonb WHERE student_id = ?
	`, studentID).Exec(context.Background())
	require.NoError(t, err)
}

func listEnrollableAsAdmin(t *testing.T, db *bun.DB, accountID int64) []*parentModels.EnrollablePhase {
	t.Helper()
	repo := parentRepo.NewEnrollablePhaseRepository(db)
	var list []*parentModels.EnrollablePhase
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListEnrollable(ctx, accountID)
		return lErr
	}))
	return list
}

func phaseNamesOf(list []*parentModels.EnrollablePhase) map[string]string {
	out := make(map[string]string, len(list))
	for _, p := range list {
		out[p.PhaseName] = p.Audience
	}
	return out
}

// --- ListEnrollable: audience + permission filtering (#1663) -----------

func TestEnrollablePhaseRepository_ListEnrollable_LinkedParentsPhaseHiddenWithoutGuardianLink(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	var tenantID int64 = 1
	testpkg.EnsureTestTenant(t, db, tenantID)
	enableEnrollmentForTenant(t, db, tenantID)

	// Fresh account: no tenant mapping, no guardian rows anywhere.
	account := testpkg.CreateTestAccount(t, db, "eligibility-unlinked")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.accounts").Where("id = ?", account.ID).Exec(context.Background())
	})

	openName := fmt.Sprintf("eligibility-open-%d", time.Now().UnixNano())
	linkedName := fmt.Sprintf("eligibility-linked-%d", time.Now().UnixNano())
	defer wipePhasesForTenant(db, tenantID, "eligibility-")
	insertEnrollablePhaseWithAudience(t, db, tenantID, openName, enrollmentModels.PhaseAudienceOpen)
	insertEnrollablePhaseWithAudience(t, db, tenantID, linkedName, enrollmentModels.PhaseAudienceLinkedParents)

	names := phaseNamesOf(listEnrollableAsAdmin(t, db, account.ID))
	assert.Contains(t, names, openName, "open phase must be listed for an unlinked parent")
	assert.Equal(t, enrollmentModels.PhaseAudienceOpen, names[openName], "audience must be mapped onto the row")
	assert.NotContains(t, names, linkedName,
		"linked_parents phase must be hidden for an account without a guardian link")
}

func TestEnrollablePhaseRepository_ListEnrollable_LinkedParentsPhaseVisibleWithSubmitPermission(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() { testpkg.CleanupParentGuardianChain(t, db, chain) })
	enableEnrollmentForTenant(t, db, chain.TenantID)

	linkedName := fmt.Sprintf("eligibility-visible-%d", time.Now().UnixNano())
	defer wipePhasesForTenant(db, chain.TenantID, "eligibility-visible")
	insertEnrollablePhaseWithAudience(t, db, chain.TenantID, linkedName, enrollmentModels.PhaseAudienceLinkedParents)

	names := phaseNamesOf(listEnrollableAsAdmin(t, db, chain.AccountID))
	assert.Contains(t, names, linkedName,
		"primary guardian (full permission set) must see the linked_parents phase")
}

func TestEnrollablePhaseRepository_ListEnrollable_RevokedSubmitPermissionHidesSchool(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() { testpkg.CleanupParentGuardianChain(t, db, chain) })
	enableEnrollmentForTenant(t, db, chain.TenantID)
	revokeGuardianSubmitPermission(t, db, chain.StudentID)

	openName := fmt.Sprintf("eligibility-revoked-%d", time.Now().UnixNano())
	defer wipePhasesForTenant(db, chain.TenantID, "eligibility-revoked")
	insertEnrollablePhaseWithAudience(t, db, chain.TenantID, openName, enrollmentModels.PhaseAudienceOpen)

	names := phaseNamesOf(listEnrollableAsAdmin(t, db, chain.AccountID))
	assert.NotContains(t, names, openName,
		"a guardian whose links all lack enrollment.submit must not see the school's phases at all")
}

// --- GuardianSubmitStatus ----------------------------------------------

func TestEnrollablePhaseRepository_GuardianSubmitStatus(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() { testpkg.CleanupParentGuardianChain(t, db, chain) })

	repo := parentRepo.NewEnrollablePhaseRepository(db)

	// Full chain: linked + guardian link + submit permission.
	var status *parentModels.GuardianSubmitStatus
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var sErr error
		status, sErr = repo.GuardianSubmitStatus(ctx, chain.AccountID, chain.TenantID)
		return sErr
	}))
	assert.True(t, status.Linked)
	assert.True(t, status.HasGuardianLink)
	assert.True(t, status.HasSubmitPermission)

	// Fresh account: nothing anywhere.
	fresh := testpkg.CreateTestAccount(t, db, "submitstatus-fresh")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.accounts").Where("id = ?", fresh.ID).Exec(context.Background())
	})
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var sErr error
		status, sErr = repo.GuardianSubmitStatus(ctx, fresh.ID, chain.TenantID)
		return sErr
	}))
	assert.False(t, status.Linked)
	assert.False(t, status.HasGuardianLink)
	assert.False(t, status.HasSubmitPermission)

	// Revoked permission: link stays, permission drops.
	revokeGuardianSubmitPermission(t, db, chain.StudentID)
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var sErr error
		status, sErr = repo.GuardianSubmitStatus(ctx, chain.AccountID, chain.TenantID)
		return sErr
	}))
	assert.True(t, status.Linked)
	assert.True(t, status.HasGuardianLink)
	assert.False(t, status.HasSubmitPermission)

	// Guard clauses.
	_, err := repo.GuardianSubmitStatus(context.Background(), 0, chain.TenantID)
	require.Error(t, err)
}
