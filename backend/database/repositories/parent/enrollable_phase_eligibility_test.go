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

// deactivateAccountTenantMapping flips the account's account_tenants row to
// inactive, simulating a guardian whose school membership was revoked while
// the historical guardian-profile / students_guardians rows linger.
func deactivateAccountTenantMapping(t *testing.T, db *bun.DB, accountID int64) {
	t.Helper()
	_, err := db.NewRaw(`
		UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ?
	`, accountID).Exec(context.Background())
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

func TestEnrollablePhaseRepository_ListEnrollable_RevokedSubmitPermissionScopedToExistingStudents(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() { testpkg.CleanupParentGuardianChain(t, db, chain) })
	enableEnrollmentForTenant(t, db, chain.TenantID)
	revokeGuardianSubmitPermission(t, db, chain.StudentID)

	// The account has a guardian link at the school but no relationship grants
	// parent_portal.enrollment.submit. The revoked/lapsed permission must block
	// re-enrolling an EXISTING child (existing_students audience), but it must
	// NOT hide an OPEN phase the same account can bootstrap a genuinely new
	// child into and submit via a direct URL — the authenticated submit path
	// applies no account-wide denial, so the picker must not either (#1663).
	openName := fmt.Sprintf("eligibility-revoked-open-%d", time.Now().UnixNano())
	existingName := fmt.Sprintf("eligibility-revoked-existing-%d", time.Now().UnixNano())
	defer wipePhasesForTenant(db, chain.TenantID, "eligibility-revoked")
	insertEnrollablePhaseWithAudience(t, db, chain.TenantID, openName, enrollmentModels.PhaseAudienceOpen)
	insertEnrollablePhaseWithAudience(t, db, chain.TenantID, existingName, enrollmentModels.PhaseAudienceExistingStudents)

	names := phaseNamesOf(listEnrollableAsAdmin(t, db, chain.AccountID))
	assert.Contains(t, names, openName,
		"a restricted guardian must still see an open phase they can bootstrap a new child into")
	assert.NotContains(t, names, existingName,
		"a guardian whose links all lack enrollment.submit must not see existing_students re-enrollment phases")
}

func TestEnrollablePhaseRepository_ListEnrollable_ExistingStudentsHiddenWithoutGuardianLink(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	var tenantID int64 = 1
	testpkg.EnsureTestTenant(t, db, tenantID)
	enableEnrollmentForTenant(t, db, tenantID)

	// Fresh account: no tenant mapping, no guardian rows anywhere. Such an
	// account can never complete an existing_students phase — a child it does
	// not have fails the enrolled gate, and a child it does have fails the
	// per-student submit permission — so the picker must not advertise it
	// (#1663).
	account := testpkg.CreateTestAccount(t, db, "eligibility-existing-unlinked")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.accounts").Where("id = ?", account.ID).Exec(context.Background())
	})

	openName := fmt.Sprintf("eligibility-exunlinked-open-%d", time.Now().UnixNano())
	existingName := fmt.Sprintf("eligibility-exunlinked-existing-%d", time.Now().UnixNano())
	defer wipePhasesForTenant(db, tenantID, "eligibility-exunlinked")
	insertEnrollablePhaseWithAudience(t, db, tenantID, openName, enrollmentModels.PhaseAudienceOpen)
	insertEnrollablePhaseWithAudience(t, db, tenantID, existingName, enrollmentModels.PhaseAudienceExistingStudents)

	names := phaseNamesOf(listEnrollableAsAdmin(t, db, account.ID))
	assert.Contains(t, names, openName,
		"a genuinely new applicant must still see open phases")
	assert.NotContains(t, names, existingName,
		"existing_students must stay hidden for an account with no submit permission at the school")
}

func TestEnrollablePhaseRepository_ListEnrollable_ExistingStudentsVisibleWithSubmitPermission(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() { testpkg.CleanupParentGuardianChain(t, db, chain) })
	enableEnrollmentForTenant(t, db, chain.TenantID)

	existingName := fmt.Sprintf("eligibility-exvisible-%d", time.Now().UnixNano())
	defer wipePhasesForTenant(db, chain.TenantID, "eligibility-exvisible")
	insertEnrollablePhaseWithAudience(t, db, chain.TenantID, existingName, enrollmentModels.PhaseAudienceExistingStudents)

	names := phaseNamesOf(listEnrollableAsAdmin(t, db, chain.AccountID))
	assert.Contains(t, names, existingName,
		"a guardian holding enrollment.submit must see the re-enrollment phase")
}

func TestEnrollablePhaseRepository_ListEnrollable_DeactivatedMappingHidesLinkedPhase(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() { testpkg.CleanupParentGuardianChain(t, db, chain) })
	enableEnrollmentForTenant(t, db, chain.TenantID)
	// Full permission set stays on the guardian link; only the membership
	// mapping is deactivated. The linked_parents phase must still disappear.
	deactivateAccountTenantMapping(t, db, chain.AccountID)

	linkedName := fmt.Sprintf("eligibility-deactivated-%d", time.Now().UnixNano())
	defer wipePhasesForTenant(db, chain.TenantID, "eligibility-deactivated")
	insertEnrollablePhaseWithAudience(t, db, chain.TenantID, linkedName, enrollmentModels.PhaseAudienceLinkedParents)

	names := phaseNamesOf(listEnrollableAsAdmin(t, db, chain.AccountID))
	assert.NotContains(t, names, linkedName,
		"a former guardian whose account_tenants mapping is deactivated must not see linked_parents phases")
}

// setSchoolHidden flips platform.schools.hidden for a tenant, simulating an
// operator hiding a school from public discovery.
func setSchoolHidden(t *testing.T, db *bun.DB, tenantID int64, hidden bool) {
	t.Helper()
	_, err := db.NewRaw(`UPDATE platform.schools SET hidden = ? WHERE id = ?`, hidden, tenantID).Exec(context.Background())
	require.NoError(t, err)
}

// --- ListEnrollable: hidden-school discovery (#1663) -------------------

func TestEnrollablePhaseRepository_ListEnrollable_HiddenSchoolExcludedForUnlinkedParent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() { testpkg.CleanupParentGuardianChain(t, db, chain) })
	enableEnrollmentForTenant(t, db, chain.TenantID)
	// The chain reuses the shared tenant 1; always restore hidden so this
	// test can never leak the flag onto tenant-1 tests that run after it.
	// A defer (not t.Cleanup) so it runs before the deferred db.Close.
	defer setSchoolHidden(t, db, chain.TenantID, false)
	setSchoolHidden(t, db, chain.TenantID, true)

	openName := fmt.Sprintf("eligibility-hidden-%d", time.Now().UnixNano())
	defer wipePhasesForTenant(db, chain.TenantID, "eligibility-hidden")
	insertEnrollablePhaseWithAudience(t, db, chain.TenantID, openName, enrollmentModels.PhaseAudienceOpen)

	// Fresh account: not a member of the hidden school.
	fresh := testpkg.CreateTestAccount(t, db, "hidden-unlinked")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.accounts").Where("id = ?", fresh.ID).Exec(context.Background())
	})

	names := phaseNamesOf(listEnrollableAsAdmin(t, db, fresh.ID))
	assert.NotContains(t, names, openName,
		"a hidden school's phases must not surface to an unlinked parent (matches the public tenant listing)")

	// The very same phase is discoverable once the school is un-hidden.
	setSchoolHidden(t, db, chain.TenantID, false)
	visible := phaseNamesOf(listEnrollableAsAdmin(t, db, fresh.ID))
	assert.Contains(t, visible, openName,
		"a non-hidden school's open phase is discoverable by any parent")
}

func TestEnrollablePhaseRepository_ListEnrollable_HiddenSchoolVisibleToActiveMember(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() { testpkg.CleanupParentGuardianChain(t, db, chain) })
	enableEnrollmentForTenant(t, db, chain.TenantID)
	// The chain reuses the shared tenant 1; always restore hidden so this
	// test can never leak the flag onto tenant-1 tests that run after it.
	// A defer (not t.Cleanup) so it runs before the deferred db.Close.
	defer setSchoolHidden(t, db, chain.TenantID, false)
	setSchoolHidden(t, db, chain.TenantID, true)

	openName := fmt.Sprintf("eligibility-hiddenmember-%d", time.Now().UnixNano())
	defer wipePhasesForTenant(db, chain.TenantID, "eligibility-hiddenmember")
	insertEnrollablePhaseWithAudience(t, db, chain.TenantID, openName, enrollmentModels.PhaseAudienceOpen)

	names := phaseNamesOf(listEnrollableAsAdmin(t, db, chain.AccountID))
	assert.Contains(t, names, openName,
		"an active member must still see their own (hidden) school's phases")
}

// --- GuardianSubmitStatus ----------------------------------------------

func TestEnrollablePhaseRepository_GuardianSubmitStatus_DeactivatedMappingDropsSubmitPermission(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() { testpkg.CleanupParentGuardianChain(t, db, chain) })
	deactivateAccountTenantMapping(t, db, chain.AccountID)

	repo := parentRepo.NewEnrollablePhaseRepository(db)
	var status *parentModels.GuardianSubmitStatus
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var sErr error
		status, sErr = repo.GuardianSubmitStatus(ctx, chain.AccountID, chain.TenantID)
		return sErr
	}))
	assert.False(t, status.Linked, "a deactivated mapping is not active")
	assert.True(t, status.HasGuardianLink, "historical guardian rows still exist")
	assert.False(t, status.HasSubmitPermission,
		"submit permission must require an active account_tenants mapping, not just a permission-granting guardian link")
}

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
