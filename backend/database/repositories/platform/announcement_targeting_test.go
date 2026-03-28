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

// --- helpers for targeting tests ---

func createTestOrg(t *testing.T, db *bun.DB, name, slug string) *platformModels.Organization {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	org := &platformModels.Organization{Name: name, Slug: slug, Active: true}
	_, err := db.NewInsert().
		Model(org).
		ModelTableExpr("platform.organizations").
		Returning("*").
		Exec(ctx)
	require.NoError(t, err)
	return org
}

func createTestSchool(t *testing.T, db *bun.DB, orgID int64, name, slug, subdomain string) *platformModels.School {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	school := &platformModels.School{
		OrganizationID: orgID,
		Name:           name,
		Slug:           slug,
		Subdomain:      subdomain,
		Active:         true,
	}
	_, err := db.NewInsert().
		Model(school).
		ModelTableExpr("platform.schools").
		Returning("*").
		Exec(ctx)
	require.NoError(t, err)
	return school
}

func createTestAccountTenant(t *testing.T, db *bun.DB, accountID, tenantID int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.NewRaw(`
		INSERT INTO auth.account_tenants (account_id, tenant_id, status, created_at, updated_at)
		VALUES (?, ?, 'active', NOW(), NOW())
	`, accountID, tenantID).Exec(ctx)
	require.NoError(t, err)
}

func publishTestAnnouncement(t *testing.T, db *bun.DB, announcementID int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.NewRaw(`
		UPDATE platform.announcements SET published_at = NOW() - INTERVAL '1 minute' WHERE id = ?
	`, announcementID).Exec(ctx)
	require.NoError(t, err)
}

// --- Cleanup helpers ---

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
	orgA := createTestOrg(t, db, "Org A Targeting", "org-a-targeting")
	orgB := createTestOrg(t, db, "Org B Targeting", "org-b-targeting")
	schoolA := createTestSchool(t, db, orgA.ID, "School A", "school-a-tgt", "school-a-tgt")
	schoolB := createTestSchool(t, db, orgB.ID, "School B", "school-b-tgt", "school-b-tgt")

	operator := createTestOperator(t, db, "targeting-org@example.com", "Org Targeting Test")
	userA := createTestAccount(t, db, "user-org-a@example.com")
	userB := createTestAccount(t, db, "user-org-b@example.com")

	createTestAccountTenant(t, db, userA, schoolA.ID)
	createTestAccountTenant(t, db, userB, schoolB.ID)

	// Announcement targeted at org A only
	announcement := &platformModels.Announcement{
		Title:           "Org A Only",
		Content:         "Only for org A users",
		Type:            platformModels.TypeAnnouncement,
		Severity:        platformModels.SeverityInfo,
		Active:          true,
		CreatedBy:       operator.ID,
		TargetRoles:     []string{},
		TargetOrgIDs:    []int64{orgA.ID},
		TargetTenantIDs: []int64{},
	}
	err := annoRepo.Create(ctx, announcement)
	require.NoError(t, err)
	publishTestAnnouncement(t, db, announcement.ID)

	defer cleanupTargetingTestData(t, db,
		[]int64{announcement.ID},
		[]int64{userA, userB},
		[]int64{schoolA.ID, schoolB.ID},
		[]int64{orgA.ID, orgB.ID},
	)
	defer cleanupTestOperator(t, db, operator.ID)

	// User A (in org A's school) should see it
	unreadA, err := viewRepo.GetUnreadForUser(ctx, userA, []string{}, schoolA.ID, orgA.ID)
	require.NoError(t, err)
	assert.Len(t, unreadA, 1, "user in org A should see org-A-targeted announcement")

	// User B (in org B's school) should NOT see it
	unreadB, err := viewRepo.GetUnreadForUser(ctx, userB, []string{}, schoolB.ID, orgB.ID)
	require.NoError(t, err)
	assert.Len(t, unreadB, 0, "user in org B should NOT see org-A-targeted announcement")
}

func TestAnnouncementTargeting_GetUnreadForUser_TenantTargetedVisibility(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	viewRepo := platform.NewAnnouncementViewRepository(db)
	annoRepo := platform.NewAnnouncementRepository(db)

	orgA := createTestOrg(t, db, "Org Tenant Test", "org-tenant-test")
	schoolA := createTestSchool(t, db, orgA.ID, "School Tenant A", "school-tenant-a", "school-tenant-a")
	schoolB := createTestSchool(t, db, orgA.ID, "School Tenant B", "school-tenant-b", "school-tenant-b")

	operator := createTestOperator(t, db, "targeting-tenant@example.com", "Tenant Targeting Test")
	userA := createTestAccount(t, db, "user-tenant-a@example.com")
	userB := createTestAccount(t, db, "user-tenant-b@example.com")

	createTestAccountTenant(t, db, userA, schoolA.ID)
	createTestAccountTenant(t, db, userB, schoolB.ID)

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
		TargetTenantIDs: []int64{schoolA.ID},
	}
	err := annoRepo.Create(ctx, announcement)
	require.NoError(t, err)
	publishTestAnnouncement(t, db, announcement.ID)

	defer cleanupTargetingTestData(t, db,
		[]int64{announcement.ID},
		[]int64{userA, userB},
		[]int64{schoolA.ID, schoolB.ID},
		[]int64{orgA.ID},
	)
	defer cleanupTestOperator(t, db, operator.ID)

	// User in school A should see it
	unreadA, err := viewRepo.GetUnreadForUser(ctx, userA, []string{}, schoolA.ID, orgA.ID)
	require.NoError(t, err)
	assert.Len(t, unreadA, 1, "user in school A should see school-A-targeted announcement")

	// User in school B should NOT see it
	unreadB, err := viewRepo.GetUnreadForUser(ctx, userB, []string{}, schoolB.ID, orgA.ID)
	require.NoError(t, err)
	assert.Len(t, unreadB, 0, "user in school B should NOT see school-A-targeted announcement")
}

func TestAnnouncementTargeting_GetUnreadForUser_ORUnion(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	viewRepo := platform.NewAnnouncementViewRepository(db)
	annoRepo := platform.NewAnnouncementRepository(db)

	orgA := createTestOrg(t, db, "Org Union A", "org-union-a")
	orgB := createTestOrg(t, db, "Org Union B", "org-union-b")
	schoolA := createTestSchool(t, db, orgA.ID, "School Union A", "school-union-a", "school-union-a")
	schoolB := createTestSchool(t, db, orgB.ID, "School Union B", "school-union-b", "school-union-b")

	operator := createTestOperator(t, db, "targeting-union@example.com", "Union Test")
	userA := createTestAccount(t, db, "user-union-a@example.com")
	userB := createTestAccount(t, db, "user-union-b@example.com")

	createTestAccountTenant(t, db, userA, schoolA.ID)
	createTestAccountTenant(t, db, userB, schoolB.ID)

	// OR-union: org A OR tenant school B
	announcement := &platformModels.Announcement{
		Title:           "OR-Union Test",
		Content:         "Org A or School B",
		Type:            platformModels.TypeAnnouncement,
		Severity:        platformModels.SeverityInfo,
		Active:          true,
		CreatedBy:       operator.ID,
		TargetRoles:     []string{},
		TargetOrgIDs:    []int64{orgA.ID},
		TargetTenantIDs: []int64{schoolB.ID},
	}
	err := annoRepo.Create(ctx, announcement)
	require.NoError(t, err)
	publishTestAnnouncement(t, db, announcement.ID)

	defer cleanupTargetingTestData(t, db,
		[]int64{announcement.ID},
		[]int64{userA, userB},
		[]int64{schoolA.ID, schoolB.ID},
		[]int64{orgA.ID, orgB.ID},
	)
	defer cleanupTestOperator(t, db, operator.ID)

	// User A is in org A → should see (org match)
	unreadA, err := viewRepo.GetUnreadForUser(ctx, userA, []string{}, schoolA.ID, orgA.ID)
	require.NoError(t, err)
	assert.Len(t, unreadA, 1, "user in org A should see announcement via org match")

	// User B is in school B → should see (tenant match)
	unreadB, err := viewRepo.GetUnreadForUser(ctx, userB, []string{}, schoolB.ID, orgB.ID)
	require.NoError(t, err)
	assert.Len(t, unreadB, 1, "user in school B should see announcement via tenant match")
}

func TestAnnouncementTargeting_CountUnread_RespectsTargeting(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	viewRepo := platform.NewAnnouncementViewRepository(db)
	annoRepo := platform.NewAnnouncementRepository(db)

	orgA := createTestOrg(t, db, "Org Count", "org-count-test")
	schoolA := createTestSchool(t, db, orgA.ID, "School Count A", "school-count-a", "school-count-a")
	schoolB := createTestSchool(t, db, orgA.ID, "School Count B", "school-count-b", "school-count-b")

	operator := createTestOperator(t, db, "targeting-count@example.com", "Count Test")
	userA := createTestAccount(t, db, "user-count-a@example.com")

	createTestAccountTenant(t, db, userA, schoolA.ID)

	// Announcement for school A only
	ann1 := &platformModels.Announcement{
		Title: "For School A", Content: "Content", Type: platformModels.TypeAnnouncement,
		Severity: platformModels.SeverityInfo, Active: true, CreatedBy: operator.ID,
		TargetRoles: []string{}, TargetOrgIDs: []int64{}, TargetTenantIDs: []int64{schoolA.ID},
	}
	// Announcement for school B only
	ann2 := &platformModels.Announcement{
		Title: "For School B", Content: "Content", Type: platformModels.TypeAnnouncement,
		Severity: platformModels.SeverityInfo, Active: true, CreatedBy: operator.ID,
		TargetRoles: []string{}, TargetOrgIDs: []int64{}, TargetTenantIDs: []int64{schoolB.ID},
	}

	require.NoError(t, annoRepo.Create(ctx, ann1))
	require.NoError(t, annoRepo.Create(ctx, ann2))
	publishTestAnnouncement(t, db, ann1.ID)
	publishTestAnnouncement(t, db, ann2.ID)

	defer cleanupTargetingTestData(t, db,
		[]int64{ann1.ID, ann2.ID},
		[]int64{userA},
		[]int64{schoolA.ID, schoolB.ID},
		[]int64{orgA.ID},
	)
	defer cleanupTestOperator(t, db, operator.ID)

	count, err := viewRepo.CountUnread(ctx, userA, []string{}, schoolA.ID, orgA.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "should only count announcement targeting user's school")
}

func TestOrganizationRepository_CountByIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	orgRepo := platform.NewOrganizationRepository(db)

	orgA := createTestOrg(t, db, "CountByIDs Org A", "countbyids-org-a")
	orgB := createTestOrg(t, db, "CountByIDs Org B", "countbyids-org-b")
	defer cleanupTargetingTestData(t, db, nil, nil, nil, []int64{orgA.ID, orgB.ID})

	ctx := context.Background()

	t.Run("counts existing IDs", func(t *testing.T) {
		count, err := orgRepo.CountByIDs(ctx, []int64{orgA.ID, orgB.ID})
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("returns 0 for empty slice", func(t *testing.T) {
		count, err := orgRepo.CountByIDs(ctx, []int64{})
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("counts only existing IDs when some are invalid", func(t *testing.T) {
		count, err := orgRepo.CountByIDs(ctx, []int64{orgA.ID, 999999})
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}

func TestSchoolRepository_CountByIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	schoolRepo := platform.NewSchoolRepository(db)

	org := createTestOrg(t, db, "CountByIDs School Org", "countbyids-school-org")
	schoolA := createTestSchool(t, db, org.ID, "CountByIDs School A", "countbyids-sa", "countbyids-sa")
	schoolB := createTestSchool(t, db, org.ID, "CountByIDs School B", "countbyids-sb", "countbyids-sb")
	defer cleanupTargetingTestData(t, db, nil, nil, []int64{schoolA.ID, schoolB.ID}, []int64{org.ID})

	ctx := context.Background()

	t.Run("counts existing IDs", func(t *testing.T) {
		count, err := schoolRepo.CountByIDs(ctx, []int64{schoolA.ID, schoolB.ID})
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("returns 0 for empty slice", func(t *testing.T) {
		count, err := schoolRepo.CountByIDs(ctx, []int64{})
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}
