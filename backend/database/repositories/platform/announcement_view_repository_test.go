package platform_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/platform"
	"github.com/moto-nrw/project-phoenix/models/auth"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnnouncementViewRepository_MarkSeen(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := platform.NewAnnouncementViewRepository(db)
	ctx := context.Background()

	operator := createTestOperator(t, db, "test@example.com", "Test Operator")
	defer cleanupTestOperator(t, db, operator.ID)

	announcement := createTestAnnouncement(t, db, "Test Announcement", operator.ID)
	defer cleanupTestAnnouncement(t, db, announcement.ID)

	account := testpkg.CreateTestAccount(t, db, "user")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("Success", func(t *testing.T) {
		err := repo.MarkSeen(ctx, account.ID, announcement.ID)
		require.NoError(t, err)

		// Verify it was marked seen
		hasSeen, err := repo.HasSeen(ctx, account.ID, announcement.ID)
		require.NoError(t, err)
		assert.True(t, hasSeen)
	})

	t.Run("Idempotent", func(t *testing.T) {
		// Marking seen again should not error
		err := repo.MarkSeen(ctx, account.ID, announcement.ID)
		require.NoError(t, err)
	})
}

func TestAnnouncementViewRepository_MarkDismissed(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := platform.NewAnnouncementViewRepository(db)
	ctx := context.Background()

	operator := createTestOperator(t, db, "test@example.com", "Test Operator")
	defer cleanupTestOperator(t, db, operator.ID)

	announcement := createTestAnnouncement(t, db, "Test Announcement", operator.ID)
	defer cleanupTestAnnouncement(t, db, announcement.ID)

	account := testpkg.CreateTestAccount(t, db, "user")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	err := repo.MarkDismissed(ctx, account.ID, announcement.ID)
	require.NoError(t, err)

	// Verify the view record was created with dismissed flag
	var view platformModels.AnnouncementView
	err = db.NewSelect().
		Model(&view).
		ModelTableExpr(`platform.announcement_views`).
		Where("user_id = ? AND announcement_id = ?", account.ID, announcement.ID).
		Scan(ctx)
	require.NoError(t, err)
	assert.True(t, view.Dismissed)
}

func TestAnnouncementViewRepository_GetUnreadForUser(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := platform.NewAnnouncementViewRepository(db)
	ctx := context.Background()

	operator := createTestOperator(t, db, "test@example.com", "Test Operator")
	defer cleanupTestOperator(t, db, operator.ID)

	account := testpkg.CreateTestAccount(t, db, "user")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	now := time.Now()
	past := now.Add(-1 * time.Hour)

	// Create published announcement (should appear)
	published := createTestAnnouncement(t, db, "Published", operator.ID)
	published.PublishedAt = &past
	published.TargetRoles = []string{} // Empty array = all roles
	_, err := db.NewUpdate().
		Model(published).
		ModelTableExpr(`platform.announcements`).
		Column("published_at", "target_roles").
		Where("id = ?", published.ID).
		Exec(ctx)
	require.NoError(t, err)
	defer cleanupTestAnnouncement(t, db, published.ID)

	// Create role-specific announcement
	roleSpecific := createTestAnnouncement(t, db, "Admin Only", operator.ID)
	roleSpecific.PublishedAt = &past
	roleSpecific.TargetRoles = []string{"admin"}
	_, err = db.NewUpdate().
		Model(roleSpecific).
		ModelTableExpr(`platform.announcements`).
		Column("published_at", "target_roles").
		Where("id = ?", roleSpecific.ID).
		Exec(ctx)
	require.NoError(t, err)
	defer cleanupTestAnnouncement(t, db, roleSpecific.ID)

	t.Run("AllRoles", func(t *testing.T) {
		announcements, err := repo.GetUnreadForUser(ctx, account.ID, "user")
		require.NoError(t, err)

		// Should contain the published announcement
		found := false
		for _, a := range announcements {
			if a.ID == published.ID {
				found = true
			}
			// Should not contain role-specific announcement
			assert.NotEqual(t, roleSpecific.ID, a.ID)
		}
		assert.True(t, found, "published announcement should be in list")
	})

	t.Run("AdminRole", func(t *testing.T) {
		announcements, err := repo.GetUnreadForUser(ctx, account.ID, "admin")
		require.NoError(t, err)

		foundPublished := false
		foundRoleSpecific := false
		for _, a := range announcements {
			if a.ID == published.ID {
				foundPublished = true
			}
			if a.ID == roleSpecific.ID {
				foundRoleSpecific = true
			}
		}
		assert.True(t, foundPublished, "published announcement should be in list")
		assert.True(t, foundRoleSpecific, "role-specific announcement should be in list")
	})

	t.Run("ExcludesSeen", func(t *testing.T) {
		// Mark as seen
		err := repo.MarkSeen(ctx, account.ID, published.ID)
		require.NoError(t, err)

		announcements, err := repo.GetUnreadForUser(ctx, account.ID, "user")
		require.NoError(t, err)

		// Should not contain the seen announcement
		for _, a := range announcements {
			assert.NotEqual(t, published.ID, a.ID, "seen announcement should not be in list")
		}
	})
}

func TestAnnouncementViewRepository_CountUnread(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := platform.NewAnnouncementViewRepository(db)
	ctx := context.Background()

	operator := createTestOperator(t, db, "test@example.com", "Test Operator")
	defer cleanupTestOperator(t, db, operator.ID)

	account := testpkg.CreateTestAccount(t, db, "user")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	now := time.Now()
	past := now.Add(-1 * time.Hour)

	// Create two published announcements
	ann1 := createTestAnnouncement(t, db, "Announcement 1", operator.ID)
	ann1.PublishedAt = &past
	ann1.TargetRoles = []string{} // All roles
	_, err := db.NewUpdate().
		Model(ann1).
		ModelTableExpr(`platform.announcements`).
		Column("published_at", "target_roles").
		Where("id = ?", ann1.ID).
		Exec(ctx)
	require.NoError(t, err)
	defer cleanupTestAnnouncement(t, db, ann1.ID)

	ann2 := createTestAnnouncement(t, db, "Announcement 2", operator.ID)
	ann2.PublishedAt = &past
	ann2.TargetRoles = []string{} // All roles
	_, err = db.NewUpdate().
		Model(ann2).
		ModelTableExpr(`platform.announcements`).
		Column("published_at", "target_roles").
		Where("id = ?", ann2.ID).
		Exec(ctx)
	require.NoError(t, err)
	defer cleanupTestAnnouncement(t, db, ann2.ID)

	t.Run("CountAll", func(t *testing.T) {
		count, err := repo.CountUnread(ctx, account.ID, "user")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 2, "should count at least 2 unread announcements")
	})

	t.Run("CountAfterSeen", func(t *testing.T) {
		// Mark one as seen
		err := repo.MarkSeen(ctx, account.ID, ann1.ID)
		require.NoError(t, err)

		count, err := repo.CountUnread(ctx, account.ID, "user")
		require.NoError(t, err)
		// Should have one less unread announcement
		assert.GreaterOrEqual(t, count, 1)
	})
}

func TestAnnouncementViewRepository_HasSeen(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := platform.NewAnnouncementViewRepository(db)
	ctx := context.Background()

	operator := createTestOperator(t, db, "test@example.com", "Test Operator")
	defer cleanupTestOperator(t, db, operator.ID)

	announcement := createTestAnnouncement(t, db, "Test", operator.ID)
	defer cleanupTestAnnouncement(t, db, announcement.ID)

	account := testpkg.CreateTestAccount(t, db, "user")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("NotSeen", func(t *testing.T) {
		hasSeen, err := repo.HasSeen(ctx, account.ID, announcement.ID)
		require.NoError(t, err)
		assert.False(t, hasSeen)
	})

	t.Run("Seen", func(t *testing.T) {
		err := repo.MarkSeen(ctx, account.ID, announcement.ID)
		require.NoError(t, err)

		hasSeen, err := repo.HasSeen(ctx, account.ID, announcement.ID)
		require.NoError(t, err)
		assert.True(t, hasSeen)
	})
}

func TestAnnouncementViewRepository_GetStats(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := platform.NewAnnouncementViewRepository(db)
	ctx := context.Background()

	operator := createTestOperator(t, db, "test@example.com", "Test Operator")
	defer cleanupTestOperator(t, db, operator.ID)

	now := time.Now()
	past := now.Add(-1 * time.Hour)

	announcement := createTestAnnouncement(t, db, "Test", operator.ID)
	announcement.PublishedAt = &past
	announcement.TargetRoles = []string{} // All roles
	_, err := db.NewUpdate().
		Model(announcement).
		ModelTableExpr(`platform.announcements`).
		Column("published_at", "target_roles").
		Where("id = ?", announcement.ID).
		Exec(ctx)
	require.NoError(t, err)
	defer cleanupTestAnnouncement(t, db, announcement.ID)

	// Create accounts and mark views
	account1 := testpkg.CreateTestAccount(t, db, "user1")
	defer testpkg.CleanupAuthFixtures(t, db, account1.ID)

	account2 := testpkg.CreateTestAccount(t, db, "user2")
	defer testpkg.CleanupAuthFixtures(t, db, account2.ID)

	// Mark account1 as seen
	err = repo.MarkSeen(ctx, account1.ID, announcement.ID)
	require.NoError(t, err)

	// Mark account2 as dismissed
	err = repo.MarkDismissed(ctx, account2.ID, announcement.ID)
	require.NoError(t, err)

	stats, err := repo.GetStats(ctx, announcement.ID)
	require.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, announcement.ID, stats.AnnouncementID)
	assert.GreaterOrEqual(t, stats.TargetCount, 2, "should count target users")
	assert.GreaterOrEqual(t, stats.SeenCount, 2, "should count seen users")
	assert.GreaterOrEqual(t, stats.DismissedCount, 1, "should count dismissed users")
}

func TestAnnouncementViewRepository_GetViewDetails(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := platform.NewAnnouncementViewRepository(db)
	ctx := context.Background()

	operator := createTestOperator(t, db, "test@example.com", "Test Operator")
	defer cleanupTestOperator(t, db, operator.ID)

	announcement := createTestAnnouncement(t, db, "Test", operator.ID)
	defer cleanupTestAnnouncement(t, db, announcement.ID)

	// Create person with account (to test name resolution)
	person, account := testpkg.CreateTestPersonWithAccount(t, db, "John", "Doe")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)
	defer testpkg.CleanupActivityFixtures(t, db, person.ID)

	// Mark as seen
	err := repo.MarkSeen(ctx, account.ID, announcement.ID)
	require.NoError(t, err)

	details, err := repo.GetViewDetails(ctx, announcement.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, details)

	// Find our user in the details
	found := false
	for _, d := range details {
		if d.UserID == account.ID {
			found = true
			assert.Contains(t, d.UserName, "John", "should contain first name")
			assert.NotZero(t, d.SeenAt)
		}
	}
	assert.True(t, found, "should find user in view details")
}

func TestAnnouncementViewRepository_GetStats_RoleFiltering(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := platform.NewAnnouncementViewRepository(db)
	ctx := context.Background()

	operator := createTestOperator(t, db, "test@example.com", "Test Operator")
	defer cleanupTestOperator(t, db, operator.ID)

	now := time.Now()
	past := now.Add(-1 * time.Hour)

	// Create announcement targeting specific role
	announcement := createTestAnnouncement(t, db, "Admin Only", operator.ID)
	announcement.PublishedAt = &past
	announcement.TargetRoles = []string{"admin"}
	_, err := db.NewUpdate().
		Model(announcement).
		ModelTableExpr(`platform.announcements`).
		Column("published_at", "target_roles").
		Where("id = ?", announcement.ID).
		Exec(ctx)
	require.NoError(t, err)
	defer cleanupTestAnnouncement(t, db, announcement.ID)

	// Create account with admin role
	account := testpkg.CreateTestAccount(t, db, "admin")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	adminRole := testpkg.GetOrCreateTestRole(t, db, "admin")

	// Assign role to account
	_, err = db.NewInsert().
		Model(&auth.AccountRole{
			AccountID: account.ID,
			RoleID:    adminRole.ID,
		}).
		ModelTableExpr(`auth.account_roles`).
		Exec(ctx)
	require.NoError(t, err)

	stats, err := repo.GetStats(ctx, announcement.ID)
	require.NoError(t, err)
	assert.NotNil(t, stats)
	// With role filtering, target count should only include admin users
	assert.GreaterOrEqual(t, stats.TargetCount, 1, "should count at least one admin user")
}
