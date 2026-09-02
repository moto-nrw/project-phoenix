package platform_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/database/repositories/platform"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// schoolCounter ensures unique school slugs/subdomains across concurrent test runs.
var schoolCounter int64

func TestAnnouncementViewRepository_GetViewDetails(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	// Viewer names come from the People Directory composition (#2661), so
	// the test drives the composed repository the service graph uses.
	factory, err := repositories.NewFactoryWithPeopleDirectory(db)
	require.NoError(t, err)
	viewRepo := factory.AnnouncementView

	// Create operator for announcement
	operator := createTestOperator(t, db, "viewtest@example.com", "View Test Operator")

	// Create announcement
	announcement := &platformModels.Announcement{
		Title:       "View Detail Test",
		Content:     "Testing view details with names",
		Type:        platformModels.TypeAnnouncement,
		Severity:    platformModels.SeverityInfo,
		Active:      true,
		CreatedBy:   operator.ID,
		TargetRoles: []string{},
	}
	annoRepo := platform.NewAnnouncementRepository(db)
	err = annoRepo.Create(ctx, announcement)
	require.NoError(t, err)

	// Create an auth account
	accountID := createTestAccount(t, db, "viewuser@example.com")

	// Create a person linked to that account
	createTestPerson(t, db, accountID, "Max", "Mustermann")

	// Mark announcement as seen + dismissed
	err = viewRepo.MarkSeen(ctx, accountID, announcement.ID)
	require.NoError(t, err)
	err = viewRepo.MarkDismissed(ctx, accountID, announcement.ID)
	require.NoError(t, err)

	t.Run("returns user name from persons table", func(t *testing.T) {
		details, err := viewRepo.GetViewDetails(ctx, announcement.ID)
		require.NoError(t, err)
		require.Len(t, details, 1)

		detail := details[0]
		assert.Equal(t, accountID, detail.UserID)
		assert.Equal(t, "Max Mustermann", detail.UserName)
		assert.True(t, detail.Dismissed)
		assert.False(t, detail.SeenAt.IsZero())
	})

	t.Run("falls back to email when no person exists", func(t *testing.T) {
		// Create account without a person
		orphanAccountID := createTestAccount(t, db, "orphan@example.com")

		err := viewRepo.MarkSeen(ctx, orphanAccountID, announcement.ID)
		require.NoError(t, err)
		err = viewRepo.MarkDismissed(ctx, orphanAccountID, announcement.ID)
		require.NoError(t, err)

		details, err := viewRepo.GetViewDetails(ctx, announcement.ID)
		require.NoError(t, err)

		// Find the orphan entry
		var orphanDetail *platformModels.AnnouncementViewDetail
		for _, d := range details {
			if d.UserID == orphanAccountID {
				orphanDetail = d
				break
			}
		}
		require.NotNil(t, orphanDetail, "should find view detail for orphan account")
		assert.Equal(t, "orphan@example.com", orphanDetail.UserName)
	})

	t.Run("orders viewers by most recent view", func(t *testing.T) {
		olderAccountID := createTestAccount(t, db, "older@example.com")
		newerAccountID := createTestAccount(t, db, "newer@example.com")
		require.NoError(t, viewRepo.MarkSeen(ctx, olderAccountID, announcement.ID))
		require.NoError(t, viewRepo.MarkSeen(ctx, newerAccountID, announcement.ID))

		newer := time.Now().Add(time.Hour)
		_, err := db.NewRaw(
			`UPDATE platform.announcement_views SET seen_at = ? WHERE announcement_id = ? AND user_id = ?`,
			newer.Add(-time.Hour), announcement.ID, olderAccountID,
		).Exec(ctx)
		require.NoError(t, err)
		_, err = db.NewRaw(
			`UPDATE platform.announcement_views SET seen_at = ? WHERE announcement_id = ? AND user_id = ?`,
			newer, announcement.ID, newerAccountID,
		).Exec(ctx)
		require.NoError(t, err)

		details, err := viewRepo.GetViewDetails(ctx, announcement.ID)
		require.NoError(t, err)
		require.NotEmpty(t, details)
		assert.Equal(t, newerAccountID, details[0].UserID)
	})

}

// Test helpers

func createTestAccount(t *testing.T, db *bun.DB, email string) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id int64
	_, err := db.NewRaw(`
		INSERT INTO auth.accounts (email, password_hash, active, created_at, updated_at)
		VALUES (?, 'dummy-hash', true, NOW(), NOW())
		RETURNING id
	`, email).Exec(ctx, &id)
	require.NoError(t, err)
	// auth.accounts carries no tenant: without taking the row back it stays
	// visible to every other test in the binary (#2419).
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = db.NewRaw(`DELETE FROM auth.account_roles WHERE account_id = ?`, id).Exec(bg)
		_, _ = db.NewRaw(`DELETE FROM auth.account_tenants WHERE account_id = ?`, id).Exec(bg)
		_, _ = db.NewRaw(`DELETE FROM users.persons WHERE account_id = ?`, id).Exec(bg)
		_, _ = db.NewRaw(`DELETE FROM auth.accounts WHERE id = ?`, id).Exec(bg)
	})
	return id
}

func createTestPerson(t *testing.T, db *bun.DB, accountID int64, firstName, lastName string) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id int64
	_, err := db.NewRaw(`
		INSERT INTO users.persons (account_id, first_name, last_name, tenant_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, NOW(), NOW())
		RETURNING id
	`, accountID, firstName, lastName, testpkg.Tenant(t)).Exec(ctx, &id)
	require.NoError(t, err)
	return id
}

// --- Additional test helpers for org/tenant targeting tests ---

func createTestOrganization(t *testing.T, db *bun.DB, name string) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	suffix := fmt.Sprintf("%d-%d", time.Now().UnixNano(), atomic.AddInt64(&schoolCounter, 1))
	_, err := db.NewRaw(`
		SELECT setval(
			pg_get_serial_sequence('platform.organizations', 'id'),
			GREATEST(
				COALESCE((SELECT MAX(id) FROM platform.organizations), 1),
				(SELECT last_value FROM platform.organizations_id_seq)
			)
		)
	`).Exec(ctx)
	require.NoError(t, err)

	var id int64
	_, err = db.NewRaw(`
		INSERT INTO platform.organizations (name, slug, active, created_at, updated_at)
		VALUES (?, ?, true, NOW(), NOW())
		RETURNING id
	`, name, name+"-"+suffix).Exec(ctx, &id)
	require.NoError(t, err)
	return id
}

func createTestSchool(t *testing.T, db *bun.DB, name string, orgID int64) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	suffix := fmt.Sprintf("%d-%d", time.Now().UnixNano(), atomic.AddInt64(&schoolCounter, 1))
	_, err := db.NewRaw(`
		SELECT setval(
			pg_get_serial_sequence('platform.schools', 'id'),
			GREATEST(
				COALESCE((SELECT MAX(id) FROM platform.schools), 1),
				(SELECT last_value FROM platform.schools_id_seq)
			)
		)
	`).Exec(ctx)
	require.NoError(t, err)

	var id int64
	_, err = db.NewRaw(`
		INSERT INTO platform.schools (name, slug, subdomain, organization_id, active, created_at, updated_at)
		VALUES (?, ?, ?, ?, true, NOW(), NOW())
		RETURNING id
	`, name, name+"-"+suffix, name+"-"+suffix, orgID).Exec(ctx, &id)
	require.NoError(t, err)
	return id
}

func createTestRole(t *testing.T, db *bun.DB, roleName string) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var id int64
	_, err := db.NewRaw(`
		INSERT INTO auth.roles (name, created_at, updated_at)
		VALUES (?, NOW(), NOW())
		RETURNING id
	`, roleName).Exec(ctx, &id)
	require.NoError(t, err)
	// A role without a tenant is part of the clone-wide RBAC catalog.
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = db.NewRaw(`DELETE FROM auth.account_roles WHERE role_id = ?`, id).Exec(bg)
		_, _ = db.NewRaw(`DELETE FROM auth.roles WHERE id = ?`, id).Exec(bg)
	})
	return id
}

func createTestRoleWithBaseRole(t *testing.T, db *bun.DB, roleName, baseRole string, tenantID int64) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var id int64
	_, err := db.NewRaw(`
		INSERT INTO auth.roles (name, tenant_id, base_role, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
		RETURNING id
	`, roleName, tenantID, baseRole).Exec(ctx, &id)
	require.NoError(t, err)
	return id
}

func assignTestRole(t *testing.T, db *bun.DB, accountID, roleID, tenantID int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewRaw(`
		INSERT INTO auth.account_roles (account_id, role_id, tenant_id, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
	`, accountID, roleID, tenantID).Exec(ctx)
	require.NoError(t, err)
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

// createTestAnnouncementWithTargeting creates an announcement with specific targeting arrays via raw SQL.
func createTestAnnouncementWithTargeting(
	t *testing.T, db *bun.DB, title string, createdBy int64,
	targetRoles []string, targetOrgIDs []int64, targetTenantIDs []int64,
) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Build Postgres array literals
	rolesLit := pgTextArray(targetRoles)
	orgLit := pgInt64Array(targetOrgIDs)
	tenantLit := pgInt64Array(targetTenantIDs)

	var id int64
	_, err := db.NewRaw(`
		INSERT INTO platform.announcements
			(title, content, type, severity, active, created_by, target_roles, target_org_ids, target_tenant_ids, created_at, updated_at)
		VALUES (?, 'test content', 'announcement', 'info', true, ?, ?::text[], ?::bigint[], ?::bigint[], NOW(), NOW())
		RETURNING id
	`, title, createdBy, rolesLit, orgLit, tenantLit).Exec(ctx, &id)
	require.NoError(t, err)
	// platform.announcements has no tenant, so the row is shared state for
	// every other test in this binary and for the leftover gate: this fixture
	// takes it back itself (#2419).
	t.Cleanup(func() {
		_, _ = db.NewRaw(`DELETE FROM platform.announcement_views WHERE announcement_id = ?`, id).
			Exec(context.Background())
		_, _ = db.NewRaw(`DELETE FROM platform.announcements WHERE id = ?`, id).
			Exec(context.Background())
	})
	return id
}

// publishTestAnnouncement sets published_at = NOW() for the given announcement.
func publishTestAnnouncement(t *testing.T, db *bun.DB, announcementID int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewRaw(`UPDATE platform.announcements SET published_at = NOW() WHERE id = ?`, announcementID).Exec(ctx)
	require.NoError(t, err)
}

func publishTestAnnouncementAt(t *testing.T, db *bun.DB, announcementID int64, publishedAt time.Time) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewRaw(`UPDATE platform.announcements SET published_at = ? WHERE id = ?`, publishedAt, announcementID).Exec(ctx)
	require.NoError(t, err)
}

func setTestAccountCreatedAt(t *testing.T, db *bun.DB, accountID int64, createdAt time.Time) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewRaw(`UPDATE auth.accounts SET created_at = ? WHERE id = ?`, createdAt, accountID).Exec(ctx)
	require.NoError(t, err)
}

func setTestSchoolCreatedAt(t *testing.T, db *bun.DB, schoolID int64, createdAt time.Time) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewRaw(`UPDATE platform.schools SET created_at = ? WHERE id = ?`, createdAt, schoolID).Exec(ctx)
	require.NoError(t, err)
}

func setTestAccountTenantInvitedAt(t *testing.T, db *bun.DB, accountID, tenantID int64, invitedAt time.Time) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewRaw(`
		UPDATE auth.account_tenants
		SET invited_at = ?, created_at = ?
		WHERE account_id = ? AND tenant_id = ?
	`, invitedAt, invitedAt, accountID, tenantID).Exec(ctx)
	require.NoError(t, err)
}

// expireTestAnnouncement sets expires_at to the past for the given announcement.
func expireTestAnnouncement(t *testing.T, db *bun.DB, announcementID int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewRaw(`UPDATE platform.announcements SET expires_at = NOW() - INTERVAL '1 hour' WHERE id = ?`, announcementID).Exec(ctx)
	require.NoError(t, err)
}

// softDeleteTestSchool sets deleted_at on a school to simulate soft-deletion.
func softDeleteTestSchool(t *testing.T, db *bun.DB, schoolID int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewRaw(`UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, schoolID).Exec(ctx)
	require.NoError(t, err)
}

// pgTextArray returns a Postgres text array literal like '{foo,bar}' or '{}' for empty.
func pgTextArray(vals []string) string {
	if len(vals) == 0 {
		return "{}"
	}
	s := "{"
	for i, v := range vals {
		if i > 0 {
			s += ","
		}
		s += v
	}
	s += "}"
	return s
}

// pgInt64Array returns a Postgres bigint array literal like '{1,2}' or '{}' for empty.
func pgInt64Array(vals []int64) string {
	if len(vals) == 0 {
		return "{}"
	}
	s := "{"
	for i, v := range vals {
		if i > 0 {
			s += ","
		}
		s += fmt.Sprintf("%d", v)
	}
	s += "}"
	return s
}

// --- Test: MarkSeen ---

func TestAnnouncementViewRepository_MarkSeen(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	viewRepo := platform.NewAnnouncementViewRepository(db)

	operator := createTestOperator(t, db, "markseen-op@test.com", "MarkSeen Op")

	announcement := createTestAnnouncement(t, db, "MarkSeen Test", operator.ID)

	accountID := createTestAccount(t, db, "markseen-user@test.com")

	err := viewRepo.MarkSeen(ctx, accountID, announcement.ID)
	require.NoError(t, err)

	// Verify via raw SQL (HasSeen has a known BUN alias bug)
	var count int
	err = db.NewRaw(`SELECT COUNT(*) FROM platform.announcement_views WHERE user_id = ? AND announcement_id = ?`,
		accountID, announcement.ID).Scan(ctx, &count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "announcement should be marked as seen")

	// Calling MarkSeen again should not error (upsert)
	err = viewRepo.MarkSeen(ctx, accountID, announcement.ID)
	require.NoError(t, err)
}

// --- Test: MarkDismissed ---

func TestAnnouncementViewRepository_MarkDismissed(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	viewRepo := platform.NewAnnouncementViewRepository(db)

	operator := createTestOperator(t, db, "markdismiss-op@test.com", "Dismiss Op")

	announcement := createTestAnnouncement(t, db, "MarkDismissed Test", operator.ID)

	accountID := createTestAccount(t, db, "markdismiss-user@test.com")

	err := viewRepo.MarkDismissed(ctx, accountID, announcement.ID)
	require.NoError(t, err)

	// Verify dismissed flag via GetViewDetails
	details, err := viewRepo.GetViewDetails(ctx, announcement.ID)
	require.NoError(t, err)
	require.Len(t, details, 1)
	assert.True(t, details[0].Dismissed, "announcement should be dismissed")
	assert.Equal(t, accountID, details[0].UserID)
}

// --- Test: HasSeen ---
// NOTE: HasSeen has a known BUN ORM alias conflict. The repository uses
// ModelTableExpr('platform.announcement_views AS "view"') but Model(view) causes BUN
// to auto-generate SELECT columns referencing "announcement_view" (derived from the struct name).
// The WHERE clause uses "view".user_id which is correct for the FROM alias, but the SELECT
// columns reference the wrong alias. This needs a fix in the repository (e.g., adding a
// BeforeAppendModel hook for SelectQuery, or using ColumnExpr to override column generation).
// The test below documents the expected behavior and will pass once the bug is fixed.

// --- Test: GetUnreadForUser ---

func TestAnnouncementViewRepository_GetUnreadForUser(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	viewRepo := platform.NewAnnouncementViewRepository(db)

	operator := createTestOperator(t, db, "unread-op@test.com", "Unread Op")

	accountID := createTestAccount(t, db, "unread-user@test.com")

	// Create org/school infrastructure for targeting tests
	orgID := createTestOrganization(t, db, "unread-test-org")

	schoolID := createTestSchool(t, db, "unread-test-school", orgID)

	otherOrgID := createTestOrganization(t, db, "unread-other-org")

	otherSchoolID := createTestSchool(t, db, "unread-other-school", otherOrgID)

	// Create a role for the user
	roleID := createTestRole(t, db, "unread-test-teacher")

	assignTestRole(t, db, accountID, roleID, schoolID)

	// Create account_tenants membership so the subquery finds this user's org/tenant
	createTestAccountTenant(t, db, accountID, schoolID)
	defer func() {
		cleanupCtx := testpkg.Ctx(t)
		_, _ = db.NewRaw(`DELETE FROM auth.account_tenants WHERE account_id = ? AND tenant_id = ?`, accountID, schoolID).Exec(cleanupCtx)
	}()

	t.Run("global announcement visible to all", func(t *testing.T) {
		annoID := createTestAnnouncementWithTargeting(t, db, "global-all", operator.ID, []string{}, []int64{}, []int64{})
		publishTestAnnouncement(t, db, annoID)

		results, err := viewRepo.GetUnreadForUser(ctx, accountID, []string{"unread-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)

		found := false
		for _, a := range results {
			if a.ID == annoID {
				found = true
			}
		}
		assert.True(t, found, "global announcement should be visible")
	})

	t.Run("role-targeted announcement matches user role", func(t *testing.T) {
		annoID := createTestAnnouncementWithTargeting(t, db, "role-match", operator.ID, []string{"unread-test-teacher"}, []int64{}, []int64{})
		publishTestAnnouncement(t, db, annoID)

		results, err := viewRepo.GetUnreadForUser(ctx, accountID, []string{"unread-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)

		found := false
		for _, a := range results {
			if a.ID == annoID {
				found = true
			}
		}
		assert.True(t, found, "role-targeted announcement should match user with that role")
	})

	t.Run("role-targeted announcement skips non-matching", func(t *testing.T) {
		annoID := createTestAnnouncementWithTargeting(t, db, "role-nomatch", operator.ID, []string{"unread-test-admin"}, []int64{}, []int64{})
		publishTestAnnouncement(t, db, annoID)

		results, err := viewRepo.GetUnreadForUser(ctx, accountID, []string{"unread-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)

		for _, a := range results {
			assert.NotEqual(t, annoID, a.ID, "role-targeted announcement should NOT match user with different role")
		}
	})

	t.Run("base_role maps custom role to system role for targeting", func(t *testing.T) {
		// Create a custom role with base_role = "user" in the test tenant
		customRoleID := createTestRoleWithBaseRole(t, db, "gruppenleitung-base-test", "user", schoolID)

		// Assign the custom role to the user
		assignTestRole(t, db, accountID, customRoleID, schoolID)

		// Create announcement targeting system role "user"
		annoID := createTestAnnouncementWithTargeting(t, db, "base-role-match", operator.ID, []string{"user"}, []int64{}, []int64{})
		publishTestAnnouncement(t, db, annoID)

		// User's JWT roles don't include "user" — only the custom name
		results, err := viewRepo.GetUnreadForUser(ctx, accountID, []string{"gruppenleitung-base-test"}, schoolID, orgID)
		require.NoError(t, err)

		found := false
		for _, a := range results {
			if a.ID == annoID {
				found = true
			}
		}
		assert.True(t, found, "announcement targeting 'user' should be visible to user with custom role base_role='user'")
	})

	t.Run("base_role does not leak across tenants", func(t *testing.T) {
		// Create a custom role with base_role = "admin" in a DIFFERENT tenant
		otherRoleID := createTestRoleWithBaseRole(t, db, "other-tenant-admin-test", "admin", otherSchoolID)

		// Assign it in the other tenant
		assignTestRole(t, db, accountID, otherRoleID, otherSchoolID)

		// Create announcement targeting "admin" — should NOT match because the
		// base_role="admin" assignment is in otherSchoolID, not schoolID
		annoID := createTestAnnouncementWithTargeting(t, db, "base-role-cross-tenant", operator.ID, []string{"admin"}, []int64{}, []int64{})
		publishTestAnnouncement(t, db, annoID)

		// Query with the user's actual tenant (schoolID) — the other-tenant role should not match
		results, err := viewRepo.GetUnreadForUser(ctx, accountID, []string{"gruppenleitung-base-test"}, schoolID, orgID)
		require.NoError(t, err)

		for _, a := range results {
			assert.NotEqual(t, annoID, a.ID, "base_role from a different tenant should NOT cause announcement match")
		}
	})

	t.Run("org-targeted announcement matches user org", func(t *testing.T) {
		annoID := createTestAnnouncementWithTargeting(t, db, "org-match", operator.ID, []string{}, []int64{orgID}, []int64{})
		publishTestAnnouncement(t, db, annoID)

		results, err := viewRepo.GetUnreadForUser(ctx, accountID, []string{"unread-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)

		found := false
		for _, a := range results {
			if a.ID == annoID {
				found = true
			}
		}
		assert.True(t, found, "org-targeted announcement should match user's org")
	})

	t.Run("org-targeted announcement skips different org", func(t *testing.T) {
		annoID := createTestAnnouncementWithTargeting(t, db, "org-nomatch", operator.ID, []string{}, []int64{otherOrgID}, []int64{})
		publishTestAnnouncement(t, db, annoID)

		results, err := viewRepo.GetUnreadForUser(ctx, accountID, []string{"unread-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)

		for _, a := range results {
			assert.NotEqual(t, annoID, a.ID, "org-targeted announcement should NOT match different org")
		}
	})

	t.Run("tenant-targeted announcement matches", func(t *testing.T) {
		annoID := createTestAnnouncementWithTargeting(t, db, "tenant-match", operator.ID, []string{}, []int64{}, []int64{schoolID})
		publishTestAnnouncement(t, db, annoID)

		results, err := viewRepo.GetUnreadForUser(ctx, accountID, []string{"unread-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)

		found := false
		for _, a := range results {
			if a.ID == annoID {
				found = true
			}
		}
		assert.True(t, found, "tenant-targeted announcement should match user's tenant")
	})

	t.Run("tenant-targeted announcement skips different tenant", func(t *testing.T) {
		annoID := createTestAnnouncementWithTargeting(t, db, "tenant-nomatch", operator.ID, []string{}, []int64{}, []int64{otherSchoolID})
		publishTestAnnouncement(t, db, annoID)

		results, err := viewRepo.GetUnreadForUser(ctx, accountID, []string{"unread-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)

		for _, a := range results {
			assert.NotEqual(t, annoID, a.ID, "tenant-targeted announcement should NOT match different tenant")
		}
	})

	t.Run("OR-union: org match wins even if tenant doesn't match", func(t *testing.T) {
		// Both org and tenant targeting set, but only org matches
		annoID := createTestAnnouncementWithTargeting(t, db, "or-union", operator.ID, []string{}, []int64{orgID}, []int64{otherSchoolID})
		publishTestAnnouncement(t, db, annoID)

		results, err := viewRepo.GetUnreadForUser(ctx, accountID, []string{"unread-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)

		found := false
		for _, a := range results {
			if a.ID == annoID {
				found = true
			}
		}
		assert.True(t, found, "OR-union: org match should make announcement visible even if tenant doesn't match")
	})

	t.Run("multi-membership user does not see other tenant announcements", func(t *testing.T) {
		// Regression test for P1 cross-tenant leak: a user with active memberships
		// in schools A and B, querying as school A, must NOT see an announcement
		// targeted only at school B (or B's org).
		secondSchoolID := createTestSchool(t, db, "unread-second-school", otherOrgID)

		// Give the user an active membership in the second school too
		createTestAccountTenant(t, db, accountID, secondSchoolID)

		// Announcement targets only the second school's org
		annoOrgB := createTestAnnouncementWithTargeting(t, db, "cross-tenant-org", operator.ID, []string{}, []int64{otherOrgID}, []int64{})
		publishTestAnnouncement(t, db, annoOrgB)

		// Announcement targets only the second school directly
		annoTenantB := createTestAnnouncementWithTargeting(t, db, "cross-tenant-school", operator.ID, []string{}, []int64{}, []int64{secondSchoolID})
		publishTestAnnouncement(t, db, annoTenantB)

		// Query as school A (schoolID / orgID) — neither announcement should be visible
		results, err := viewRepo.GetUnreadForUser(ctx, accountID, []string{"unread-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)

		for _, a := range results {
			assert.NotEqual(t, annoOrgB, a.ID, "announcement targeting other org must not leak to current session tenant")
			assert.NotEqual(t, annoTenantB, a.ID, "announcement targeting other school must not leak to current session tenant")
		}

		// But when querying as school B, both should be visible
		resultsB, err := viewRepo.GetUnreadForUser(ctx, accountID, []string{"unread-test-teacher"}, secondSchoolID, otherOrgID)
		require.NoError(t, err)

		foundOrg, foundTenant := false, false
		for _, a := range resultsB {
			if a.ID == annoOrgB {
				foundOrg = true
			}
			if a.ID == annoTenantB {
				foundTenant = true
			}
		}
		assert.True(t, foundOrg, "org-targeted announcement should be visible when querying as the target org")
		assert.True(t, foundTenant, "tenant-targeted announcement should be visible when querying as the target tenant")
	})

	t.Run("seen announcements are excluded", func(t *testing.T) {
		annoID := createTestAnnouncementWithTargeting(t, db, "seen-excluded", operator.ID, []string{}, []int64{}, []int64{})
		publishTestAnnouncement(t, db, annoID)

		// Mark it seen
		err := viewRepo.MarkSeen(ctx, accountID, annoID)
		require.NoError(t, err)

		results, err := viewRepo.GetUnreadForUser(ctx, accountID, []string{"unread-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)

		for _, a := range results {
			assert.NotEqual(t, annoID, a.ID, "seen announcement should be excluded from unread")
		}
	})

	t.Run("unpublished announcements are excluded", func(t *testing.T) {
		// Do NOT call publishTestAnnouncement
		annoID := createTestAnnouncementWithTargeting(t, db, "unpublished", operator.ID, []string{}, []int64{}, []int64{})

		results, err := viewRepo.GetUnreadForUser(ctx, accountID, []string{"unread-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)

		for _, a := range results {
			assert.NotEqual(t, annoID, a.ID, "unpublished announcement should be excluded")
		}
	})

	t.Run("expired announcements are excluded", func(t *testing.T) {
		annoID := createTestAnnouncementWithTargeting(t, db, "expired-excl", operator.ID, []string{}, []int64{}, []int64{})
		publishTestAnnouncement(t, db, annoID)
		expireTestAnnouncement(t, db, annoID)

		results, err := viewRepo.GetUnreadForUser(ctx, accountID, []string{"unread-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)

		for _, a := range results {
			assert.NotEqual(t, annoID, a.ID, "expired announcement should be excluded")
		}
	})
}

func TestAnnouncementViewRepository_RecipientBaseline(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	viewRepo := platform.NewAnnouncementViewRepository(db)

	operator := createTestOperator(t, db, "baseline-op@test.com", "Baseline Op")

	orgID := createTestOrganization(t, db, "baseline-org")

	schoolID := createTestSchool(t, db, "baseline-school", orgID)

	accountID := createTestAccount(t, db, "baseline-user@test.com")
	createTestAccountTenant(t, db, accountID, schoolID)

	now := time.Now().UTC()
	setTestAccountCreatedAt(t, db, accountID, now.Add(-6*time.Hour))
	setTestAccountTenantInvitedAt(t, db, accountID, schoolID, now.Add(-5*time.Hour))
	setTestSchoolCreatedAt(t, db, schoolID, now.Add(-4*time.Hour))

	findAnnouncement := func(announcements []*platformModels.Announcement, id int64) bool {
		for _, announcement := range announcements {
			if announcement.ID == id {
				return true
			}
		}
		return false
	}

	t.Run("global announcement before tenant creation is excluded", func(t *testing.T) {
		annoID := createTestAnnouncementWithTargeting(t, db, "baseline-old-global", operator.ID, []string{}, []int64{}, []int64{})
		publishTestAnnouncementAt(t, db, annoID, now.Add(-5*time.Hour))

		results, err := viewRepo.GetUnreadForUser(ctx, accountID, []string{}, schoolID, orgID)
		require.NoError(t, err)
		assert.False(t, findAnnouncement(results, annoID), "announcement published before school creation should be excluded")
	})

	t.Run("global announcement after recipient baseline is visible", func(t *testing.T) {
		annoID := createTestAnnouncementWithTargeting(t, db, "baseline-new-global", operator.ID, []string{}, []int64{}, []int64{})
		publishTestAnnouncementAt(t, db, annoID, now.Add(-3*time.Hour))

		results, err := viewRepo.GetUnreadForUser(ctx, accountID, []string{}, schoolID, orgID)
		require.NoError(t, err)
		assert.True(t, findAnnouncement(results, annoID), "announcement published after recipient baseline should be visible")
	})

	t.Run("tenant-targeted announcement before baseline is excluded", func(t *testing.T) {
		annoID := createTestAnnouncementWithTargeting(t, db, "baseline-old-tenant", operator.ID, []string{}, []int64{}, []int64{schoolID})
		publishTestAnnouncementAt(t, db, annoID, now.Add(-5*time.Hour))

		results, err := viewRepo.GetUnreadForUser(ctx, accountID, []string{}, schoolID, orgID)
		require.NoError(t, err)
		assert.False(t, findAnnouncement(results, annoID), "explicit tenant targeting should still respect the recipient baseline")
	})

	t.Run("new employee does not see announcements from before tenant invitation", func(t *testing.T) {
		newAccountID := createTestAccount(t, db, "baseline-new-employee@test.com")
		createTestAccountTenant(t, db, newAccountID, schoolID)
		setTestAccountCreatedAt(t, db, newAccountID, now.Add(-6*time.Hour))
		setTestAccountTenantInvitedAt(t, db, newAccountID, schoolID, now.Add(-50*time.Minute))

		oldAnnoID := createTestAnnouncementWithTargeting(t, db, "baseline-before-invite", operator.ID, []string{}, []int64{}, []int64{})
		publishTestAnnouncementAt(t, db, oldAnnoID, now.Add(-2*time.Hour))

		newAnnoID := createTestAnnouncementWithTargeting(t, db, "baseline-after-invite", operator.ID, []string{}, []int64{}, []int64{})
		publishTestAnnouncementAt(t, db, newAnnoID, now.Add(-30*time.Minute))

		results, err := viewRepo.GetUnreadForUser(ctx, newAccountID, []string{}, schoolID, orgID)
		require.NoError(t, err)
		assert.False(t, findAnnouncement(results, oldAnnoID), "announcement before membership invitation should be excluded")
		assert.True(t, findAnnouncement(results, newAnnoID), "announcement after membership invitation should be visible")
	})

	t.Run("account creation participates in the baseline", func(t *testing.T) {
		newAccountID := createTestAccount(t, db, "baseline-new-account@test.com")
		createTestAccountTenant(t, db, newAccountID, schoolID)
		setTestAccountCreatedAt(t, db, newAccountID, now.Add(-30*time.Minute))
		setTestAccountTenantInvitedAt(t, db, newAccountID, schoolID, now.Add(-3*time.Hour))

		oldAnnoID := createTestAnnouncementWithTargeting(t, db, "baseline-before-account", operator.ID, []string{}, []int64{}, []int64{})
		publishTestAnnouncementAt(t, db, oldAnnoID, now.Add(-1*time.Hour))

		newAnnoID := createTestAnnouncementWithTargeting(t, db, "baseline-after-account", operator.ID, []string{}, []int64{}, []int64{})
		publishTestAnnouncementAt(t, db, newAnnoID, now.Add(-20*time.Minute))

		results, err := viewRepo.GetUnreadForUser(ctx, newAccountID, []string{}, schoolID, orgID)
		require.NoError(t, err)
		assert.False(t, findAnnouncement(results, oldAnnoID), "announcement before account creation should be excluded")
		assert.True(t, findAnnouncement(results, newAnnoID), "announcement after account creation should be visible")
	})

	t.Run("count unread uses the same baseline", func(t *testing.T) {
		countAccountID := createTestAccount(t, db, "baseline-count@test.com")
		createTestAccountTenant(t, db, countAccountID, schoolID)
		setTestAccountCreatedAt(t, db, countAccountID, now.Add(-6*time.Hour))
		setTestAccountTenantInvitedAt(t, db, countAccountID, schoolID, now.Add(-50*time.Minute))

		baselineCount, err := viewRepo.CountUnread(ctx, countAccountID, []string{}, schoolID, orgID)
		require.NoError(t, err)

		oldAnnoID := createTestAnnouncementWithTargeting(t, db, "baseline-count-old", operator.ID, []string{}, []int64{}, []int64{})
		publishTestAnnouncementAt(t, db, oldAnnoID, now.Add(-2*time.Hour))

		newAnnoID := createTestAnnouncementWithTargeting(t, db, "baseline-count-new", operator.ID, []string{}, []int64{}, []int64{})
		publishTestAnnouncementAt(t, db, newAnnoID, now.Add(-30*time.Minute))

		count, err := viewRepo.CountUnread(ctx, countAccountID, []string{}, schoolID, orgID)
		require.NoError(t, err)
		assert.Equal(t, baselineCount+1, count, "count should include only the announcement after recipient baseline")
	})
}

// --- Test: CountUnread ---

func TestAnnouncementViewRepository_CountUnread(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	viewRepo := platform.NewAnnouncementViewRepository(db)

	operator := createTestOperator(t, db, "count-op@test.com", "Count Op")

	accountID := createTestAccount(t, db, "count-user@test.com")

	orgID := createTestOrganization(t, db, "count-test-org")

	schoolID := createTestSchool(t, db, "count-test-school", orgID)

	otherOrgID := createTestOrganization(t, db, "count-other-org")

	otherSchoolID := createTestSchool(t, db, "count-other-school", otherOrgID)

	roleID := createTestRole(t, db, "count-test-teacher")

	assignTestRole(t, db, accountID, roleID, schoolID)

	// Create account_tenants membership so the subquery finds this user's org/tenant
	createTestAccountTenant(t, db, accountID, schoolID)
	defer func() {
		cleanupCtx := testpkg.Ctx(t)
		_, _ = db.NewRaw(`DELETE FROM auth.account_tenants WHERE account_id = ? AND tenant_id = ?`, accountID, schoolID).Exec(cleanupCtx)
	}()

	// Baseline per subtest, not once for the test: announcements are
	// tenant-less, so every subtest's announcement is visible to the next one
	// until its fixture cleanup runs at the end of the test (#2419).
	baseline := func(t *testing.T) int {
		t.Helper()
		count, err := viewRepo.CountUnread(ctx, accountID, []string{"count-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)
		return count
	}

	t.Run("global announcement counted", func(t *testing.T) {
		baselineCount := baseline(t)
		annoID := createTestAnnouncementWithTargeting(t, db, "count-global", operator.ID, []string{}, []int64{}, []int64{})
		publishTestAnnouncement(t, db, annoID)

		count, err := viewRepo.CountUnread(ctx, accountID, []string{"count-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)
		assert.Equal(t, baselineCount+1, count, "count should increase by 1 for a global announcement")
	})

	t.Run("role-targeted announcement counted when matching", func(t *testing.T) {
		baselineCount := baseline(t)
		annoID := createTestAnnouncementWithTargeting(t, db, "count-role-match", operator.ID, []string{"count-test-teacher"}, []int64{}, []int64{})
		publishTestAnnouncement(t, db, annoID)

		count, err := viewRepo.CountUnread(ctx, accountID, []string{"count-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)
		assert.Equal(t, baselineCount+1, count)
	})

	t.Run("role-targeted announcement not counted when not matching", func(t *testing.T) {
		baselineCount := baseline(t)
		annoID := createTestAnnouncementWithTargeting(t, db, "count-role-nomatch", operator.ID, []string{"count-test-admin"}, []int64{}, []int64{})
		publishTestAnnouncement(t, db, annoID)

		count, err := viewRepo.CountUnread(ctx, accountID, []string{"count-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)
		assert.Equal(t, baselineCount, count)
	})

	t.Run("org-targeted announcement counted when matching", func(t *testing.T) {
		baselineCount := baseline(t)
		annoID := createTestAnnouncementWithTargeting(t, db, "count-org-match", operator.ID, []string{}, []int64{orgID}, []int64{})
		publishTestAnnouncement(t, db, annoID)

		count, err := viewRepo.CountUnread(ctx, accountID, []string{"count-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)
		assert.Equal(t, baselineCount+1, count)
	})

	t.Run("org-targeted announcement not counted for different org", func(t *testing.T) {
		baselineCount := baseline(t)
		annoID := createTestAnnouncementWithTargeting(t, db, "count-org-nomatch", operator.ID, []string{}, []int64{otherOrgID}, []int64{})
		publishTestAnnouncement(t, db, annoID)

		count, err := viewRepo.CountUnread(ctx, accountID, []string{"count-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)
		assert.Equal(t, baselineCount, count)
	})

	t.Run("tenant-targeted announcement counted when matching", func(t *testing.T) {
		baselineCount := baseline(t)
		annoID := createTestAnnouncementWithTargeting(t, db, "count-tenant-match", operator.ID, []string{}, []int64{}, []int64{schoolID})
		publishTestAnnouncement(t, db, annoID)

		count, err := viewRepo.CountUnread(ctx, accountID, []string{"count-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)
		assert.Equal(t, baselineCount+1, count)
	})

	t.Run("tenant-targeted announcement not counted for different tenant", func(t *testing.T) {
		baselineCount := baseline(t)
		annoID := createTestAnnouncementWithTargeting(t, db, "count-tenant-nomatch", operator.ID, []string{}, []int64{}, []int64{otherSchoolID})
		publishTestAnnouncement(t, db, annoID)

		count, err := viewRepo.CountUnread(ctx, accountID, []string{"count-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)
		assert.Equal(t, baselineCount, count)
	})

	t.Run("OR-union: org match counted even if tenant doesn't match", func(t *testing.T) {
		baselineCount := baseline(t)
		// Both org and tenant targeting set, but only org matches the session
		annoID := createTestAnnouncementWithTargeting(t, db, "count-or-union", operator.ID, []string{}, []int64{orgID}, []int64{otherSchoolID})
		publishTestAnnouncement(t, db, annoID)

		count, err := viewRepo.CountUnread(ctx, accountID, []string{"count-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)
		assert.Equal(t, baselineCount+1, count, "OR-union: org match should make announcement counted")
	})

	t.Run("multi-membership user does not count other tenant announcements", func(t *testing.T) {
		baselineCount := baseline(t)
		// Regression: user in schools A+B, announcement targets B only, count as A should be 0
		secondSchoolID := createTestSchool(t, db, "count-second-school", otherOrgID)

		createTestAccountTenant(t, db, accountID, secondSchoolID)

		annoID := createTestAnnouncementWithTargeting(t, db, "count-cross-tenant", operator.ID, []string{}, []int64{}, []int64{secondSchoolID})
		publishTestAnnouncement(t, db, annoID)

		// Count as school A — should not include the school-B-only announcement
		count, err := viewRepo.CountUnread(ctx, accountID, []string{"count-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)
		assert.Equal(t, baselineCount, count, "announcement targeting other tenant must not be counted in current session")
	})

	t.Run("seen announcement not counted", func(t *testing.T) {
		baselineCount := baseline(t)
		annoID := createTestAnnouncementWithTargeting(t, db, "count-seen", operator.ID, []string{}, []int64{}, []int64{})
		publishTestAnnouncement(t, db, annoID)

		err := viewRepo.MarkSeen(ctx, accountID, annoID)
		require.NoError(t, err)

		count, err := viewRepo.CountUnread(ctx, accountID, []string{"count-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)
		assert.Equal(t, baselineCount, count)
	})

	t.Run("expired announcement not counted", func(t *testing.T) {
		baselineCount := baseline(t)
		annoID := createTestAnnouncementWithTargeting(t, db, "count-expired", operator.ID, []string{}, []int64{}, []int64{})
		publishTestAnnouncement(t, db, annoID)
		expireTestAnnouncement(t, db, annoID)

		count, err := viewRepo.CountUnread(ctx, accountID, []string{"count-test-teacher"}, schoolID, orgID)
		require.NoError(t, err)
		assert.Equal(t, baselineCount, count)
	})
}

// --- Test: GetStats ---

func TestAnnouncementViewRepository_GetStats(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	viewRepo := platform.NewAnnouncementViewRepository(db)

	operator := createTestOperator(t, db, "stats-op@test.com", "Stats Op")

	t.Run("global announcement counts all accounts", func(t *testing.T) {
		orgID := createTestOrganization(t, db, "stats-global-org")

		schoolID := createTestSchool(t, db, "stats-global-school", orgID)

		acc := createTestAccount(t, db, "stats-global-acc@test.com")
		createTestAccountTenant(t, db, acc, schoolID)

		annoID := createTestAnnouncementWithTargeting(t, db, "stats-global", operator.ID, []string{}, []int64{}, []int64{})

		stats, err := viewRepo.GetStats(ctx, annoID)
		require.NoError(t, err)
		assert.Equal(t, annoID, stats.AnnouncementID)
		assert.GreaterOrEqual(t, stats.TargetCount, 1, "global announcement should target at least the test account")
		assert.Equal(t, 0, stats.SeenCount)
		assert.Equal(t, 0, stats.DismissedCount)
	})

	t.Run("role-filtered announcement counts role-matching accounts", func(t *testing.T) {
		// Need a school for account_roles.tenant_id FK
		sOrgID := createTestOrganization(t, db, "stats-role-org")
		sSchoolID := createTestSchool(t, db, "stats-role-school", sOrgID)

		roleID := createTestRole(t, db, "stats-role-filter")

		acc1 := createTestAccount(t, db, "stats-role-acc1@test.com")
		assignTestRole(t, db, acc1, roleID, sSchoolID)
		// GetStats role-only query requires active account_tenants membership
		createTestAccountTenant(t, db, acc1, sSchoolID)

		acc2 := createTestAccount(t, db, "stats-role-acc2@test.com")
		assignTestRole(t, db, acc2, roleID, sSchoolID)
		// GetStats role-only query requires active account_tenants membership
		createTestAccountTenant(t, db, acc2, sSchoolID)

		annoID := createTestAnnouncementWithTargeting(t, db, "stats-role", operator.ID, []string{"stats-role-filter"}, []int64{}, []int64{})

		stats, err := viewRepo.GetStats(ctx, annoID)
		require.NoError(t, err)
		assert.Equal(t, 2, stats.TargetCount, "should count exactly the 2 accounts with this role")
	})

	t.Run("base_role-mapped custom role counted in stats", func(t *testing.T) {
		bOrgID := createTestOrganization(t, db, "stats-base-role-org")
		bSchoolID := createTestSchool(t, db, "stats-base-role-school", bOrgID)

		// Create a custom role with base_role = "user"
		customRoleID := createTestRoleWithBaseRole(t, db, "stats-gruppenleitung", "user", bSchoolID)

		acc := createTestAccount(t, db, "stats-base-role-acc@test.com")
		assignTestRole(t, db, acc, customRoleID, bSchoolID)
		createTestAccountTenant(t, db, acc, bSchoolID)

		// Announcement targets system role "user"
		annoID := createTestAnnouncementWithTargeting(t, db, "stats-base-role", operator.ID, []string{"user"}, []int64{}, []int64{})

		stats, err := viewRepo.GetStats(ctx, annoID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, stats.TargetCount, 1, "stats should count user with custom role base_role='user'")
	})

	t.Run("org-filtered announcement counts org-matching accounts", func(t *testing.T) {
		orgID := createTestOrganization(t, db, "stats-org")

		schoolID := createTestSchool(t, db, "stats-org-school", orgID)

		acc1 := createTestAccount(t, db, "stats-org-acc1@test.com")
		createTestAccountTenant(t, db, acc1, schoolID)

		annoID := createTestAnnouncementWithTargeting(t, db, "stats-org-target", operator.ID, []string{}, []int64{orgID}, []int64{})

		stats, err := viewRepo.GetStats(ctx, annoID)
		require.NoError(t, err)
		assert.Equal(t, 1, stats.TargetCount, "should count exactly 1 account in the org")
	})

	t.Run("tenant-filtered announcement counts tenant-matching accounts", func(t *testing.T) {
		orgID := createTestOrganization(t, db, "stats-tenant-org")

		schoolID := createTestSchool(t, db, "stats-tenant-school", orgID)

		acc1 := createTestAccount(t, db, "stats-tenant-acc1@test.com")
		createTestAccountTenant(t, db, acc1, schoolID)

		annoID := createTestAnnouncementWithTargeting(t, db, "stats-tenant-target", operator.ID, []string{}, []int64{}, []int64{schoolID})

		stats, err := viewRepo.GetStats(ctx, annoID)
		require.NoError(t, err)
		assert.Equal(t, 1, stats.TargetCount, "should count exactly 1 account in the tenant")
	})

	t.Run("published announcement target count respects recipient baseline", func(t *testing.T) {
		now := time.Now().UTC()

		orgID := createTestOrganization(t, db, "stats-baseline-org")

		schoolID := createTestSchool(t, db, "stats-baseline-school", orgID)
		setTestSchoolCreatedAt(t, db, schoolID, now.Add(-4*time.Hour))

		beforePublish := createTestAccount(t, db, "stats-baseline-before@test.com")
		createTestAccountTenant(t, db, beforePublish, schoolID)
		setTestAccountCreatedAt(t, db, beforePublish, now.Add(-3*time.Hour))
		setTestAccountTenantInvitedAt(t, db, beforePublish, schoolID, now.Add(-3*time.Hour))

		afterPublish := createTestAccount(t, db, "stats-baseline-after@test.com")
		createTestAccountTenant(t, db, afterPublish, schoolID)
		setTestAccountCreatedAt(t, db, afterPublish, now.Add(-3*time.Hour))
		setTestAccountTenantInvitedAt(t, db, afterPublish, schoolID, now.Add(-1*time.Hour))

		annoID := createTestAnnouncementWithTargeting(t, db, "stats-baseline-target", operator.ID, []string{}, []int64{}, []int64{schoolID})
		publishTestAnnouncementAt(t, db, annoID, now.Add(-2*time.Hour))

		stats, err := viewRepo.GetStats(ctx, annoID)
		require.NoError(t, err)
		assert.Equal(t, 1, stats.TargetCount, "target count should exclude accounts invited after the announcement was published")
	})

	t.Run("combined role+org filter counts intersection", func(t *testing.T) {
		orgID := createTestOrganization(t, db, "stats-combo-org")

		schoolID := createTestSchool(t, db, "stats-combo-school", orgID)

		// Create a different org/school for "role-only" account's tenant_id FK
		otherOrgID := createTestOrganization(t, db, "stats-combo-other-org")
		otherSchoolID := createTestSchool(t, db, "stats-combo-other-school", otherOrgID)

		roleID := createTestRole(t, db, "stats-combo-role")

		// Account with both role AND tenant in the target org
		accBoth := createTestAccount(t, db, "stats-combo-both@test.com")
		assignTestRole(t, db, accBoth, roleID, schoolID)
		createTestAccountTenant(t, db, accBoth, schoolID)

		// Account with role only (tenant in a different org — not in target org)
		accRoleOnly := createTestAccount(t, db, "stats-combo-roleonly@test.com")
		assignTestRole(t, db, accRoleOnly, roleID, otherSchoolID)
		createTestAccountTenant(t, db, accRoleOnly, otherSchoolID)

		// Account with tenant only (no role)
		accTenantOnly := createTestAccount(t, db, "stats-combo-tenantonly@test.com")
		createTestAccountTenant(t, db, accTenantOnly, schoolID)

		annoID := createTestAnnouncementWithTargeting(t, db, "stats-combo", operator.ID, []string{"stats-combo-role"}, []int64{orgID}, []int64{})

		stats, err := viewRepo.GetStats(ctx, annoID)
		require.NoError(t, err)
		// Only accBoth should be counted (intersection of role AND org).
		// accRoleOnly has the role but in a different org — must NOT be counted.
		// accTenantOnly is in the org but without the role — must NOT be counted.
		assert.Equal(t, 1, stats.TargetCount, "only the account with both matching role AND org should be counted")
	})

	t.Run("org+tenant filter counts OR-union without role", func(t *testing.T) {
		orgID := createTestOrganization(t, db, "stats-orgtenant-org")

		schoolInOrg := createTestSchool(t, db, "stats-orgtenant-school1", orgID)

		otherOrgID := createTestOrganization(t, db, "stats-orgtenant-other-org")

		schoolInOtherOrg := createTestSchool(t, db, "stats-orgtenant-school2", otherOrgID)

		// Account in org (should match org filter)
		accInOrg := createTestAccount(t, db, "stats-orgtenant-inorg@test.com")
		createTestAccountTenant(t, db, accInOrg, schoolInOrg)

		// Account in specific tenant in other org (should match tenant filter)
		accInTenant := createTestAccount(t, db, "stats-orgtenant-intenant@test.com")
		createTestAccountTenant(t, db, accInTenant, schoolInOtherOrg)

		// Announcement targets org AND a specific tenant in other org (OR-union)
		annoID := createTestAnnouncementWithTargeting(t, db, "stats-orgtenant", operator.ID, []string{}, []int64{orgID}, []int64{schoolInOtherOrg})

		stats, err := viewRepo.GetStats(ctx, annoID)
		require.NoError(t, err)
		assert.Equal(t, 2, stats.TargetCount, "OR-union should count exactly the 2 accounts: one in org, one in target tenant")
	})

	t.Run("role+org+tenant filter counts intersection of role with OR-union", func(t *testing.T) {
		orgID := createTestOrganization(t, db, "stats-roleorgtenant-org")

		schoolInOrg := createTestSchool(t, db, "stats-roleorgtenant-school1", orgID)

		otherOrgID := createTestOrganization(t, db, "stats-roleorgtenant-other-org")

		schoolInOtherOrg := createTestSchool(t, db, "stats-roleorgtenant-school2", otherOrgID)

		roleID := createTestRole(t, db, "stats-roleorgtenant-role")

		// Account with role + in target org (should match: role AND org)
		accRoleInOrg := createTestAccount(t, db, "stats-rot-roleinorg@test.com")
		assignTestRole(t, db, accRoleInOrg, roleID, schoolInOrg)
		createTestAccountTenant(t, db, accRoleInOrg, schoolInOrg)

		// Account with role + in target tenant (should match: role AND tenant)
		accRoleInTenant := createTestAccount(t, db, "stats-rot-roleintenant@test.com")
		assignTestRole(t, db, accRoleInTenant, roleID, schoolInOtherOrg)
		createTestAccountTenant(t, db, accRoleInTenant, schoolInOtherOrg)

		// Account without role but in target org (should NOT match: missing role)
		accNoRole := createTestAccount(t, db, "stats-rot-norole@test.com")
		createTestAccountTenant(t, db, accNoRole, schoolInOrg)

		annoID := createTestAnnouncementWithTargeting(t, db, "stats-roleorgtenant", operator.ID, []string{"stats-roleorgtenant-role"}, []int64{orgID}, []int64{schoolInOtherOrg})

		stats, err := viewRepo.GetStats(ctx, annoID)
		require.NoError(t, err)
		// accRoleInOrg matches (role AND org), accRoleInTenant matches (role AND tenant).
		// accNoRole has no role — must NOT be counted despite being in the target org.
		assert.Equal(t, 2, stats.TargetCount, "should count exactly role+org account AND role+tenant account, not the no-role account")
	})

	t.Run("role+tenant filter counts intersection without org", func(t *testing.T) {
		orgID := createTestOrganization(t, db, "stats-roletenant-org")

		schoolID := createTestSchool(t, db, "stats-roletenant-school", orgID)

		roleID := createTestRole(t, db, "stats-roletenant-role")

		// Account with role + in target tenant (should match)
		accMatch := createTestAccount(t, db, "stats-roletenant-match@test.com")
		assignTestRole(t, db, accMatch, roleID, schoolID)
		createTestAccountTenant(t, db, accMatch, schoolID)

		// Account with role only, wrong tenant (should NOT match)
		otherSchoolID := createTestSchool(t, db, "stats-roletenant-other", orgID)
		accWrongTenant := createTestAccount(t, db, "stats-roletenant-wrong@test.com")
		assignTestRole(t, db, accWrongTenant, roleID, otherSchoolID)
		createTestAccountTenant(t, db, accWrongTenant, otherSchoolID)

		// Target: role + specific tenant, no org filter
		annoID := createTestAnnouncementWithTargeting(t, db, "stats-roletenant", operator.ID, []string{"stats-roletenant-role"}, []int64{}, []int64{schoolID})

		stats, err := viewRepo.GetStats(ctx, annoID)
		require.NoError(t, err)
		// accMatch has role + correct tenant. accWrongTenant has role but different tenant — must NOT be counted.
		assert.Equal(t, 1, stats.TargetCount, "should count only the account with matching role AND tenant, not the wrong-tenant account")
	})

	t.Run("soft-deleted school excludes accounts from target count", func(t *testing.T) {
		orgID := createTestOrganization(t, db, "stats-softdel-org")

		activeSchoolID := createTestSchool(t, db, "stats-softdel-active", orgID)

		deletedSchoolID := createTestSchool(t, db, "stats-softdel-deleted", orgID)

		// Account in active school
		accActive := createTestAccount(t, db, "stats-softdel-active@test.com")
		createTestAccountTenant(t, db, accActive, activeSchoolID)

		// Account in soft-deleted school
		accDeleted := createTestAccount(t, db, "stats-softdel-deleted@test.com")
		createTestAccountTenant(t, db, accDeleted, deletedSchoolID)

		// Soft-delete the school
		softDeleteTestSchool(t, db, deletedSchoolID)

		// Global announcement — should exclude accounts linked to soft-deleted school
		annoID := createTestAnnouncementWithTargeting(t, db, "stats-softdel", operator.ID, []string{}, []int64{}, []int64{})

		stats, err := viewRepo.GetStats(ctx, annoID)
		require.NoError(t, err)

		// Verify: accDeleted should NOT be counted because their school is soft-deleted.
		// We can't assert an exact total (other accounts may exist in test DB),
		// so we create a tenant-scoped announcement to get an exact count.
		annoTenantID := createTestAnnouncementWithTargeting(t, db, "stats-softdel-tenant", operator.ID, []string{}, []int64{}, []int64{activeSchoolID, deletedSchoolID})

		statsTenant, err := viewRepo.GetStats(ctx, annoTenantID)
		require.NoError(t, err)
		assert.Equal(t, 1, statsTenant.TargetCount, "should count only the account in the active school, not the soft-deleted one")

		// Also verify org-scoped: both schools are in the same org
		annoOrgID := createTestAnnouncementWithTargeting(t, db, "stats-softdel-org-target", operator.ID, []string{}, []int64{orgID}, []int64{})

		statsOrg, err := viewRepo.GetStats(ctx, annoOrgID)
		require.NoError(t, err)
		assert.Equal(t, 1, statsOrg.TargetCount, "org-scoped should also exclude accounts in soft-deleted school")

		assert.GreaterOrEqual(t, stats.TargetCount, 1,
			"global stats should include at least the active-school account")
	})

	t.Run("account with both direct role and base_role-mapped role counted once", func(t *testing.T) {
		// Regression test: if an account has BOTH a direct role name match
		// AND a custom role with matching base_role, the COUNT(DISTINCT at.account_id)
		// must still return 1, not 2. Protects the DISTINCT keyword in GetStats.
		dOrgID := createTestOrganization(t, db, "stats-dual-role-org")
		dSchoolID := createTestSchool(t, db, "stats-dual-role-school", dOrgID)

		// Role with name "user" (direct name match on target_roles)
		directRoleID := createTestRoleWithBaseRole(t, db, "user", "user", dSchoolID)

		// Custom role with base_role = "user" (base_role match on target_roles)
		customRoleID := createTestRoleWithBaseRole(t, db, "gruppenleitung-dual-test", "user", dSchoolID)

		// Single account has BOTH roles assigned
		acc := createTestAccount(t, db, "stats-dual-role@test.com")
		assignTestRole(t, db, acc, directRoleID, dSchoolID)
		assignTestRole(t, db, acc, customRoleID, dSchoolID)
		createTestAccountTenant(t, db, acc, dSchoolID)

		// Scope to this tenant so we only count accounts in dSchoolID
		annoID := createTestAnnouncementWithTargeting(t, db, "stats-dual-role", operator.ID, []string{"user"}, []int64{}, []int64{dSchoolID})

		stats, err := viewRepo.GetStats(ctx, annoID)
		require.NoError(t, err)
		assert.Equal(t, 1, stats.TargetCount, "account with both direct and base_role match must be counted exactly once")
	})

	t.Run("seen and dismissed counts", func(t *testing.T) {
		annoID := createTestAnnouncementWithTargeting(t, db, "stats-views", operator.ID, []string{}, []int64{}, []int64{})

		acc1 := createTestAccount(t, db, "stats-view-acc1@test.com")

		acc2 := createTestAccount(t, db, "stats-view-acc2@test.com")

		// acc1 seen only
		err := viewRepo.MarkSeen(ctx, acc1, annoID)
		require.NoError(t, err)

		// acc2 seen + dismissed
		err = viewRepo.MarkDismissed(ctx, acc2, annoID)
		require.NoError(t, err)

		stats, err := viewRepo.GetStats(ctx, annoID)
		require.NoError(t, err)
		assert.Equal(t, 2, stats.SeenCount, "2 users have seen the announcement")
		assert.Equal(t, 1, stats.DismissedCount, "1 user dismissed the announcement")
	})
}
