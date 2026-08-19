package platform_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/platform"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// --- Cleanup helper for targeting tests ---

func cleanupTargetingTestData(t *testing.T, db *bun.DB, announcementIDs []int64, accountIDs []int64, schoolIDs []int64, orgIDs []int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, aID := range announcementIDs {
		_, _ = db.NewRaw(`DELETE FROM platform.announcement_views WHERE announcement_id = ?`, aID).Exec(ctx)
		_, _ = db.NewRaw(`DELETE FROM platform.announcements WHERE id = ?`, aID).Exec(ctx)
	}
	for _, accID := range accountIDs {
		_, _ = db.NewRaw(`DELETE FROM auth.account_roles WHERE account_id = ?`, accID).Exec(ctx)
		_, _ = db.NewRaw(`DELETE FROM auth.account_tenants WHERE account_id = ?`, accID).Exec(ctx)
		_, _ = db.NewRaw(`DELETE FROM auth.accounts WHERE id = ?`, accID).Exec(ctx)
	}
	for _, sID := range schoolIDs {
		_, _ = db.NewRaw(`DELETE FROM platform.schools WHERE id = ?`, sID).Exec(ctx)
	}
	for _, oID := range orgIDs {
		_, _ = db.NewRaw(`DELETE FROM platform.organizations WHERE id = ?`, oID).Exec(ctx)
	}
}

// --- Tests ---

func TestAnnouncementTargeting_CreateWithTargetingArrays(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	annoRepo := platform.NewAnnouncementRepository(db)

	operator := createTestOperator(t, db, "targeting-create@example.com", "Targeting Create Test")
	defer cleanupTestOperator(t, db, operator.ID)

	announcement := &platformModels.Announcement{
		Title:           "Targeted Announcement",
		Content:         "Only for specific orgs and tenants",
		Type:            platformModels.TypeAnnouncement,
		Severity:        platformModels.SeverityInfo,
		Active:          true,
		CreatedBy:       operator.ID,
		TargetRoles:     []string{},
		TargetOrgIDs:    []int64{100, 200},
		TargetTenantIDs: []int64{300},
	}

	err := annoRepo.Create(ctx, announcement)
	require.NoError(t, err)
	defer cleanupTestAnnouncement(t, db, announcement.ID)

	// Verify targeting arrays persisted correctly
	found, err := annoRepo.FindByID(ctx, announcement.ID)
	require.NoError(t, err)
	require.NotNil(t, found)

	assert.Equal(t, []int64{100, 200}, found.TargetOrgIDs)
	assert.Equal(t, []int64{300}, found.TargetTenantIDs)
}

func TestAnnouncementTargeting_CreateWithEmptyArrays(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	annoRepo := platform.NewAnnouncementRepository(db)

	operator := createTestOperator(t, db, "targeting-empty@example.com", "Targeting Empty Test")
	defer cleanupTestOperator(t, db, operator.ID)

	announcement := &platformModels.Announcement{
		Title:           "Global Announcement",
		Content:         "For everyone",
		Type:            platformModels.TypeAnnouncement,
		Severity:        platformModels.SeverityInfo,
		Active:          true,
		CreatedBy:       operator.ID,
		TargetRoles:     []string{},
		TargetOrgIDs:    []int64{},
		TargetTenantIDs: []int64{},
	}

	err := annoRepo.Create(ctx, announcement)
	require.NoError(t, err)
	defer cleanupTestAnnouncement(t, db, announcement.ID)

	found, err := annoRepo.FindByID(ctx, announcement.ID)
	require.NoError(t, err)
	require.NotNil(t, found)

	assert.Empty(t, found.TargetOrgIDs)
	assert.Empty(t, found.TargetTenantIDs)
}

func TestAnnouncementTargeting_GetUnreadForUser_GlobalVisibleToAll(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	viewRepo := platform.NewAnnouncementViewRepository(db)
	annoRepo := platform.NewAnnouncementRepository(db)

	operator := createTestOperator(t, db, "targeting-global@example.com", "Global Test")
	accountID := createTestAccount(t, db, "user-global@example.com")
	orgID := createTestOrganization(t, db, "Org Global Targeting")
	schoolID := createTestSchool(t, db, "School Global Targeting", orgID)
	createTestAccountTenant(t, db, accountID, schoolID)

	// Create a global announcement (empty targeting)
	announcement := &platformModels.Announcement{
		Title:           "Global News",
		Content:         "Everyone should see this",
		Type:            platformModels.TypeAnnouncement,
		Severity:        platformModels.SeverityInfo,
		Active:          true,
		CreatedBy:       operator.ID,
		TargetRoles:     []string{},
		TargetOrgIDs:    []int64{},
		TargetTenantIDs: []int64{},
	}
	err := annoRepo.Create(ctx, announcement)
	require.NoError(t, err)
	publishTestAnnouncement(t, db, announcement.ID)

	defer cleanupTargetingTestData(t, db,
		[]int64{announcement.ID},
		[]int64{accountID},
		[]int64{schoolID}, []int64{orgID},
	)
	defer cleanupTestOperator(t, db, operator.ID)

	// Global announcements are visible in a valid tenant context.
	unread, err := viewRepo.GetUnreadForUser(ctx, accountID, []string{}, schoolID, orgID)
	require.NoError(t, err)
	assert.Len(t, unread, 1)
	assert.Equal(t, announcement.ID, unread[0].ID)
}

func TestAnnouncementTargeting_GetUnreadForUser_OrgTargetedVisibility(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	viewRepo := platform.NewAnnouncementViewRepository(db)
	annoRepo := platform.NewAnnouncementRepository(db)

	// Setup: org A with school A, org B with school B
	orgAID := createTestOrganization(t, db, "Org A Targeting")
	orgBID := createTestOrganization(t, db, "Org B Targeting")
	schoolAID := createTestSchool(t, db, "School A Tgt", orgAID)
	schoolBID := createTestSchool(t, db, "School B Tgt", orgBID)

	operator := createTestOperator(t, db, "targeting-org@example.com", "Org Targeting Test")
	userA := createTestAccount(t, db, "user-org-a@example.com")
	userB := createTestAccount(t, db, "user-org-b@example.com")

	createTestAccountTenant(t, db, userA, schoolAID)
	createTestAccountTenant(t, db, userB, schoolBID)

	// Announcement targeted at org A only
	announcement := &platformModels.Announcement{
		Title:           "Org A Only",
		Content:         "Only for org A users",
		Type:            platformModels.TypeAnnouncement,
		Severity:        platformModels.SeverityInfo,
		Active:          true,
		CreatedBy:       operator.ID,
		TargetRoles:     []string{},
		TargetOrgIDs:    []int64{orgAID},
		TargetTenantIDs: []int64{},
	}
	err := annoRepo.Create(ctx, announcement)
	require.NoError(t, err)
	publishTestAnnouncement(t, db, announcement.ID)

	defer cleanupTargetingTestData(t, db,
		[]int64{announcement.ID},
		[]int64{userA, userB},
		[]int64{schoolAID, schoolBID},
		[]int64{orgAID, orgBID},
	)
	defer cleanupTestOperator(t, db, operator.ID)

	// User A (in org A's school) should see it
	unreadA, err := viewRepo.GetUnreadForUser(ctx, userA, []string{}, schoolAID, orgAID)
	require.NoError(t, err)
	assert.Len(t, unreadA, 1, "user in org A should see org-A-targeted announcement")

	// User B (in org B's school) should NOT see it
	unreadB, err := viewRepo.GetUnreadForUser(ctx, userB, []string{}, schoolBID, orgBID)
	require.NoError(t, err)
	assert.Len(t, unreadB, 0, "user in org B should NOT see org-A-targeted announcement")
}

func TestAnnouncementTargeting_GetUnreadForUser_TenantTargetedVisibility(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	viewRepo := platform.NewAnnouncementViewRepository(db)
	annoRepo := platform.NewAnnouncementRepository(db)

	orgAID := createTestOrganization(t, db, "Org Tenant Test")
	schoolAID := createTestSchool(t, db, "School Tenant A", orgAID)
	schoolBID := createTestSchool(t, db, "School Tenant B", orgAID)

	operator := createTestOperator(t, db, "targeting-tenant@example.com", "Tenant Targeting Test")
	userA := createTestAccount(t, db, "user-tenant-a@example.com")
	userB := createTestAccount(t, db, "user-tenant-b@example.com")

	createTestAccountTenant(t, db, userA, schoolAID)
	createTestAccountTenant(t, db, userB, schoolBID)

	// Announcement targeted at school A (tenant) only
	announcement := &platformModels.Announcement{
		Title:           "School A Only",
		Content:         "Only for school A",
		Type:            platformModels.TypeAnnouncement,
		Severity:        platformModels.SeverityInfo,
		Active:          true,
		CreatedBy:       operator.ID,
		TargetRoles:     []string{},
		TargetOrgIDs:    []int64{},
		TargetTenantIDs: []int64{schoolAID},
	}
	err := annoRepo.Create(ctx, announcement)
	require.NoError(t, err)
	publishTestAnnouncement(t, db, announcement.ID)

	defer cleanupTargetingTestData(t, db,
		[]int64{announcement.ID},
		[]int64{userA, userB},
		[]int64{schoolAID, schoolBID},
		[]int64{orgAID},
	)
	defer cleanupTestOperator(t, db, operator.ID)

	// User in school A should see it
	unreadA, err := viewRepo.GetUnreadForUser(ctx, userA, []string{}, schoolAID, orgAID)
	require.NoError(t, err)
	assert.Len(t, unreadA, 1, "user in school A should see school-A-targeted announcement")

	// User in school B should NOT see it
	unreadB, err := viewRepo.GetUnreadForUser(ctx, userB, []string{}, schoolBID, orgAID)
	require.NoError(t, err)
	assert.Len(t, unreadB, 0, "user in school B should NOT see school-A-targeted announcement")
}

func TestAnnouncementTargeting_GetUnreadForUser_ORUnion(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	viewRepo := platform.NewAnnouncementViewRepository(db)
	annoRepo := platform.NewAnnouncementRepository(db)

	orgAID := createTestOrganization(t, db, "Org Union A")
	orgBID := createTestOrganization(t, db, "Org Union B")
	schoolAID := createTestSchool(t, db, "School Union A", orgAID)
	schoolBID := createTestSchool(t, db, "School Union B", orgBID)

	operator := createTestOperator(t, db, "targeting-union@example.com", "Union Test")
	userA := createTestAccount(t, db, "user-union-a@example.com")
	userB := createTestAccount(t, db, "user-union-b@example.com")

	createTestAccountTenant(t, db, userA, schoolAID)
	createTestAccountTenant(t, db, userB, schoolBID)

	// OR-union: org A OR tenant school B
	announcement := &platformModels.Announcement{
		Title:           "OR-Union Test",
		Content:         "Org A or School B",
		Type:            platformModels.TypeAnnouncement,
		Severity:        platformModels.SeverityInfo,
		Active:          true,
		CreatedBy:       operator.ID,
		TargetRoles:     []string{},
		TargetOrgIDs:    []int64{orgAID},
		TargetTenantIDs: []int64{schoolBID},
	}
	err := annoRepo.Create(ctx, announcement)
	require.NoError(t, err)
	publishTestAnnouncement(t, db, announcement.ID)

	defer cleanupTargetingTestData(t, db,
		[]int64{announcement.ID},
		[]int64{userA, userB},
		[]int64{schoolAID, schoolBID},
		[]int64{orgAID, orgBID},
	)
	defer cleanupTestOperator(t, db, operator.ID)

	// User A is in org A → should see (org match)
	unreadA, err := viewRepo.GetUnreadForUser(ctx, userA, []string{}, schoolAID, orgAID)
	require.NoError(t, err)
	assert.Len(t, unreadA, 1, "user in org A should see announcement via org match")

	// User B is in school B → should see (tenant match)
	unreadB, err := viewRepo.GetUnreadForUser(ctx, userB, []string{}, schoolBID, orgBID)
	require.NoError(t, err)
	assert.Len(t, unreadB, 1, "user in school B should see announcement via tenant match")
}

func TestAnnouncementTargeting_CountUnread_RespectsTargeting(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	viewRepo := platform.NewAnnouncementViewRepository(db)
	annoRepo := platform.NewAnnouncementRepository(db)

	orgAID := createTestOrganization(t, db, "Org Count")
	schoolAID := createTestSchool(t, db, "School Count A", orgAID)
	schoolBID := createTestSchool(t, db, "School Count B", orgAID)

	operator := createTestOperator(t, db, "targeting-count@example.com", "Count Test")
	userA := createTestAccount(t, db, "user-count-a@example.com")

	createTestAccountTenant(t, db, userA, schoolAID)

	// Announcement for school A only
	ann1 := &platformModels.Announcement{
		Title: "For School A", Content: "Content", Type: platformModels.TypeAnnouncement,
		Severity: platformModels.SeverityInfo, Active: true, CreatedBy: operator.ID,
		TargetRoles: []string{}, TargetOrgIDs: []int64{}, TargetTenantIDs: []int64{schoolAID},
	}
	// Announcement for school B only
	ann2 := &platformModels.Announcement{
		Title: "For School B", Content: "Content", Type: platformModels.TypeAnnouncement,
		Severity: platformModels.SeverityInfo, Active: true, CreatedBy: operator.ID,
		TargetRoles: []string{}, TargetOrgIDs: []int64{}, TargetTenantIDs: []int64{schoolBID},
	}

	require.NoError(t, annoRepo.Create(ctx, ann1))
	require.NoError(t, annoRepo.Create(ctx, ann2))
	publishTestAnnouncement(t, db, ann1.ID)
	publishTestAnnouncement(t, db, ann2.ID)

	defer cleanupTargetingTestData(t, db,
		[]int64{ann1.ID, ann2.ID},
		[]int64{userA},
		[]int64{schoolAID, schoolBID},
		[]int64{orgAID},
	)
	defer cleanupTestOperator(t, db, operator.ID)

	count, err := viewRepo.CountUnread(ctx, userA, []string{}, schoolAID, orgAID)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "should only count announcement targeting user's school")
}

func TestAnnouncementTargeting_GetUnreadForUser_RolesWithOrgTargeting(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	viewRepo := platform.NewAnnouncementViewRepository(db)
	annoRepo := platform.NewAnnouncementRepository(db)

	orgAID := createTestOrganization(t, db, "Org Roles A")
	orgBID := createTestOrganization(t, db, "Org Roles B")
	schoolAID := createTestSchool(t, db, "School Roles A", orgAID)
	schoolBID := createTestSchool(t, db, "School Roles B", orgBID)

	operator := createTestOperator(t, db, "targeting-roles@example.com", "Roles Targeting Test")

	roleName := "test-targeting-teacher"
	roleID := createTestRole(t, db, roleName)

	// User with matching role in org A
	userA := createTestAccount(t, db, "user-roles-a@example.com")
	createTestAccountTenant(t, db, userA, schoolAID)
	assignTestRole(t, db, userA, roleID, schoolAID)

	// User with matching role in org B
	userB := createTestAccount(t, db, "user-roles-b@example.com")
	createTestAccountTenant(t, db, userB, schoolBID)
	assignTestRole(t, db, userB, roleID, schoolBID)

	// User without role in org A
	userNoRole := createTestAccount(t, db, "user-roles-norole@example.com")
	createTestAccountTenant(t, db, userNoRole, schoolAID)

	// Announcement: role-filtered + org-targeted at org A
	announcement := &platformModels.Announcement{
		Title:           "Teachers in Org A",
		Content:         "Role + org targeted",
		Type:            platformModels.TypeAnnouncement,
		Severity:        platformModels.SeverityInfo,
		Active:          true,
		CreatedBy:       operator.ID,
		TargetRoles:     []string{roleName},
		TargetOrgIDs:    []int64{orgAID},
		TargetTenantIDs: []int64{},
	}
	err := annoRepo.Create(ctx, announcement)
	require.NoError(t, err)
	publishTestAnnouncement(t, db, announcement.ID)

	defer cleanupTargetingTestData(t, db,
		[]int64{announcement.ID},
		[]int64{userA, userB, userNoRole},
		[]int64{schoolAID, schoolBID},
		[]int64{orgAID, orgBID},
	)
	defer cleanupTestOperator(t, db, operator.ID)
	defer cleanupTestRole(t, db, roleID)
	defer cleanupTestAccountRole(t, db, userA, roleID)
	defer cleanupTestAccountRole(t, db, userB, roleID)

	// User A has role + is in org A → should see it
	unreadA, err := viewRepo.GetUnreadForUser(ctx, userA, []string{roleName}, schoolAID, orgAID)
	require.NoError(t, err)
	assert.Len(t, unreadA, 1, "user with matching role in org A should see announcement")

	// User B has role but is in org B → should NOT see it (wrong org)
	unreadB, err := viewRepo.GetUnreadForUser(ctx, userB, []string{roleName}, schoolBID, orgBID)
	require.NoError(t, err)
	assert.Len(t, unreadB, 0, "user with matching role in org B should NOT see org-A-targeted announcement")

	// User with no role in org A → should NOT see it (role filter excludes)
	unreadNoRole, err := viewRepo.GetUnreadForUser(ctx, userNoRole, []string{}, schoolAID, orgAID)
	require.NoError(t, err)
	assert.Len(t, unreadNoRole, 0, "user without matching role should NOT see role-filtered announcement")

	// Also verify CountUnread is consistent
	countA, err := viewRepo.CountUnread(ctx, userA, []string{roleName}, schoolAID, orgAID)
	require.NoError(t, err)
	assert.Equal(t, 1, countA)

	countB, err := viewRepo.CountUnread(ctx, userB, []string{roleName}, schoolBID, orgBID)
	require.NoError(t, err)
	assert.Equal(t, 0, countB)
}

func TestAnnouncementTargeting_GetUnreadForUser_DualRoleNotDuplicated(t *testing.T) {
	// Regression test: an account with BOTH a direct role match AND a custom role
	// with matching base_role must see the announcement exactly once, not duplicated.
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	viewRepo := platform.NewAnnouncementViewRepository(db)
	annoRepo := platform.NewAnnouncementRepository(db)

	orgID := createTestOrganization(t, db, "Org Dual Role")
	schoolID := createTestSchool(t, db, "School Dual Role", orgID)

	operator := createTestOperator(t, db, "targeting-dual-role@example.com", "Dual Role Test")

	// Role with name "user" (direct name match on target_roles)
	sysRoleID := createTestRoleWithBaseRole(t, db, "user", "user", schoolID)

	// Custom role with base_role = "user" (base_role match on target_roles)
	customRoleID := createTestRoleWithBaseRole(t, db, "gruppenleitung-dual-delivery", "user", schoolID)

	// Account has BOTH roles
	userID := createTestAccount(t, db, "user-dual-delivery@example.com")
	createTestAccountTenant(t, db, userID, schoolID)
	assignTestRole(t, db, userID, sysRoleID, schoolID)
	assignTestRole(t, db, userID, customRoleID, schoolID)

	// Announcement targets "user" — both roles match (one by name, one by base_role)
	announcement := &platformModels.Announcement{
		Title:           "Dual Role Delivery",
		Content:         "Should appear exactly once",
		Type:            platformModels.TypeAnnouncement,
		Severity:        platformModels.SeverityInfo,
		Active:          true,
		CreatedBy:       operator.ID,
		TargetRoles:     []string{"user"},
		TargetOrgIDs:    []int64{},
		TargetTenantIDs: []int64{},
	}
	err := annoRepo.Create(ctx, announcement)
	require.NoError(t, err)
	publishTestAnnouncement(t, db, announcement.ID)

	defer cleanupTargetingTestData(t, db,
		[]int64{announcement.ID},
		[]int64{userID},
		[]int64{schoolID},
		[]int64{orgID},
	)
	defer cleanupTestOperator(t, db, operator.ID)
	defer cleanupTestRole(t, db, sysRoleID)
	defer cleanupTestRole(t, db, customRoleID)
	defer cleanupTestAccountRole(t, db, userID, sysRoleID)
	defer cleanupTestAccountRole(t, db, userID, customRoleID)

	// GetUnreadForUser should return the announcement exactly once
	unread, err := viewRepo.GetUnreadForUser(ctx, userID, []string{"user", "gruppenleitung-dual-delivery"}, schoolID, orgID)
	require.NoError(t, err)

	matchCount := 0
	for _, a := range unread {
		if a.ID == announcement.ID {
			matchCount++
		}
	}
	assert.Equal(t, 1, matchCount, "announcement must appear exactly once even with both direct and base_role match")

	// CountUnread should also be consistent
	count, err := viewRepo.CountUnread(ctx, userID, []string{"user", "gruppenleitung-dual-delivery"}, schoolID, orgID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 1, "count should include the dual-matched announcement")
}

func TestAnnouncementTargeting_GetUnreadForUser_UpdateBaseRoleChangesDelivery(t *testing.T) {
	// Test: changing a custom role's base_role should change which announcements
	// the user receives via the base_role expansion path.
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	viewRepo := platform.NewAnnouncementViewRepository(db)
	annoRepo := platform.NewAnnouncementRepository(db)

	orgID := createTestOrganization(t, db, "Org BaseRole Update")
	schoolID := createTestSchool(t, db, "School BaseRole Update", orgID)

	operator := createTestOperator(t, db, "targeting-update-baserole@example.com", "Update BaseRole Test")

	// Custom role initially with base_role = "user"
	customRoleID := createTestRoleWithBaseRole(t, db, "dynamic-role-test", "user", schoolID)

	userID := createTestAccount(t, db, "user-update-baserole@example.com")
	createTestAccountTenant(t, db, userID, schoolID)
	assignTestRole(t, db, userID, customRoleID, schoolID)

	// Announcement targeting "admin"
	annoAdmin := &platformModels.Announcement{
		Title:           "Admin Only",
		Content:         "For admins",
		Type:            platformModels.TypeAnnouncement,
		Severity:        platformModels.SeverityInfo,
		Active:          true,
		CreatedBy:       operator.ID,
		TargetRoles:     []string{"admin"},
		TargetOrgIDs:    []int64{},
		TargetTenantIDs: []int64{},
	}
	err := annoRepo.Create(ctx, annoAdmin)
	require.NoError(t, err)
	publishTestAnnouncement(t, db, annoAdmin.ID)

	// Announcement targeting "user"
	annoUser := &platformModels.Announcement{
		Title:           "User Only",
		Content:         "For users",
		Type:            platformModels.TypeAnnouncement,
		Severity:        platformModels.SeverityInfo,
		Active:          true,
		CreatedBy:       operator.ID,
		TargetRoles:     []string{"user"},
		TargetOrgIDs:    []int64{},
		TargetTenantIDs: []int64{},
	}
	err = annoRepo.Create(ctx, annoUser)
	require.NoError(t, err)
	publishTestAnnouncement(t, db, annoUser.ID)

	defer cleanupTargetingTestData(t, db,
		[]int64{annoAdmin.ID, annoUser.ID},
		[]int64{userID},
		[]int64{schoolID},
		[]int64{orgID},
	)
	defer cleanupTestOperator(t, db, operator.ID)
	defer cleanupTestRole(t, db, customRoleID)
	defer cleanupTestAccountRole(t, db, userID, customRoleID)

	// With base_role = "user", the user should see "User Only" but NOT "Admin Only"
	unread, err := viewRepo.GetUnreadForUser(ctx, userID, []string{"dynamic-role-test"}, schoolID, orgID)
	require.NoError(t, err)

	hasUser, hasAdmin := false, false
	for _, a := range unread {
		if a.ID == annoUser.ID {
			hasUser = true
		}
		if a.ID == annoAdmin.ID {
			hasAdmin = true
		}
	}
	assert.True(t, hasUser, "user with base_role='user' should see user-targeted announcement")
	assert.False(t, hasAdmin, "user with base_role='user' should NOT see admin-targeted announcement")

	// Now update the role's base_role to "admin"
	updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = db.NewRaw(`UPDATE auth.roles SET base_role = 'admin' WHERE id = ?`, customRoleID).Exec(updateCtx)
	require.NoError(t, err)

	// After update: should see "Admin Only" but NOT "User Only"
	unread2, err := viewRepo.GetUnreadForUser(ctx, userID, []string{"dynamic-role-test"}, schoolID, orgID)
	require.NoError(t, err)

	hasUser2, hasAdmin2 := false, false
	for _, a := range unread2 {
		if a.ID == annoUser.ID {
			hasUser2 = true
		}
		if a.ID == annoAdmin.ID {
			hasAdmin2 = true
		}
	}
	assert.False(t, hasUser2, "after base_role change to 'admin', should NOT see user-targeted announcement")
	assert.True(t, hasAdmin2, "after base_role change to 'admin', should see admin-targeted announcement")
}

func TestOrganizationRepository_CountByIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	orgRepo := platform.NewOrganizationRepository(db)

	orgAID := createTestOrganization(t, db, "CountByIDs Org A")
	orgBID := createTestOrganization(t, db, "CountByIDs Org B")
	defer cleanupTargetingTestData(t, db, nil, nil, nil, []int64{orgAID, orgBID})

	ctx := context.Background()

	t.Run("counts existing IDs", func(t *testing.T) {
		count, err := orgRepo.CountByIDs(ctx, []int64{orgAID, orgBID})
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("returns 0 for empty slice", func(t *testing.T) {
		count, err := orgRepo.CountByIDs(ctx, []int64{})
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("counts only existing IDs when some are invalid", func(t *testing.T) {
		count, err := orgRepo.CountByIDs(ctx, []int64{orgAID, 999999})
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}

func TestSchoolRepository_CountByIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	schoolRepo := platform.NewSchoolRepository(db)

	orgID := createTestOrganization(t, db, "CountByIDs School Org")
	schoolAID := createTestSchool(t, db, "CountByIDs School A", orgID)
	schoolBID := createTestSchool(t, db, "CountByIDs School B", orgID)
	defer cleanupTargetingTestData(t, db, nil, nil, []int64{schoolAID, schoolBID}, []int64{orgID})

	ctx := context.Background()

	t.Run("counts existing IDs", func(t *testing.T) {
		count, err := schoolRepo.CountByIDs(ctx, []int64{schoolAID, schoolBID})
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("returns 0 for empty slice", func(t *testing.T) {
		count, err := schoolRepo.CountByIDs(ctx, []int64{})
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}
