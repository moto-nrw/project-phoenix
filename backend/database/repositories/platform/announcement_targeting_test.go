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
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
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
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
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
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	viewRepo := platform.NewAnnouncementViewRepository(db)
	annoRepo := platform.NewAnnouncementRepository(db)

	operator := createTestOperator(t, db, "targeting-global@example.com", "Global Test")
	accountID := createTestAccount(t, db, "user-global@example.com")

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
		nil, nil,
	)
	defer cleanupTestOperator(t, db, operator.ID)

	// Any user should see this regardless of tenant/org
	unread, err := viewRepo.GetUnreadForUser(ctx, accountID, []string{}, 999, 999)
	require.NoError(t, err)
	assert.Len(t, unread, 1)
	assert.Equal(t, announcement.ID, unread[0].ID)
}

func TestAnnouncementTargeting_GetUnreadForUser_OrgTargetedVisibility(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
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
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
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
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
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
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
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

func TestOrganizationRepository_CountByIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

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
	defer func() { _ = db.Close() }()

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
