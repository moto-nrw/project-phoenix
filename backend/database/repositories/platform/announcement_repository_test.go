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

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestAnnouncementRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := platform.NewAnnouncementRepository(db)
	ctx := testpkg.Ctx(t)

	// Create test operator for announcement creator
	operator := createTestOperator(t, db, "test@example.com", "Test Operator")
	defer cleanupTestOperator(t, db, operator.ID)

	t.Run("Success", func(t *testing.T) {
		announcement := &platformModels.Announcement{
			Title:       "Test Announcement",
			Content:     "This is a test announcement",
			Type:        platformModels.TypeAnnouncement,
			Severity:    platformModels.SeverityInfo,
			Active:      true,
			CreatedBy:   operator.ID,
			TargetRoles: []string{"all"},
		}

		err := repo.Create(ctx, announcement)
		require.NoError(t, err)
		assert.NotZero(t, announcement.ID)
		assert.NotZero(t, announcement.CreatedAt)

		// Cleanup
		defer cleanupTestAnnouncement(t, db, announcement.ID)
	})

	t.Run("NilAnnouncement", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("ValidationError_EmptyTitle", func(t *testing.T) {
		announcement := &platformModels.Announcement{
			Title:       "",
			Content:     "Content",
			Type:        platformModels.TypeAnnouncement,
			Severity:    platformModels.SeverityInfo,
			CreatedBy:   operator.ID,
			TargetRoles: []string{"all"},
		}

		err := repo.Create(ctx, announcement)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "title is required")
	})

	t.Run("ValidationError_InvalidType", func(t *testing.T) {
		announcement := &platformModels.Announcement{
			Title:       "Test",
			Content:     "Content",
			Type:        "invalid",
			Severity:    platformModels.SeverityInfo,
			CreatedBy:   operator.ID,
			TargetRoles: []string{"all"},
		}

		err := repo.Create(ctx, announcement)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid announcement type")
	})
}

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestAnnouncementRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := platform.NewAnnouncementRepository(db)
	ctx := testpkg.Ctx(t)

	operator := createTestOperator(t, db, "test@example.com", "Test Operator")
	defer cleanupTestOperator(t, db, operator.ID)

	t.Run("Success", func(t *testing.T) {
		announcement := createTestAnnouncement(t, db, "Test Announcement", operator.ID)
		defer cleanupTestAnnouncement(t, db, announcement.ID)

		found, err := repo.FindByID(ctx, announcement.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, announcement.ID, found.ID)
		assert.Equal(t, announcement.Title, found.Title)
	})

	t.Run("NotFound", func(t *testing.T) {
		found, err := repo.FindByID(ctx, 999999)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestAnnouncementRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := platform.NewAnnouncementRepository(db)
	ctx := testpkg.Ctx(t)

	operator := createTestOperator(t, db, "test@example.com", "Test Operator")
	defer cleanupTestOperator(t, db, operator.ID)

	t.Run("Success", func(t *testing.T) {
		announcement := createTestAnnouncement(t, db, "Original Title", operator.ID)
		defer cleanupTestAnnouncement(t, db, announcement.ID)

		announcement.Title = "Updated Title"
		announcement.Content = "Updated content"
		err := repo.Update(ctx, announcement)
		require.NoError(t, err)

		// Verify update
		found, err := repo.FindByID(ctx, announcement.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Title", found.Title)
		assert.Equal(t, "Updated content", found.Content)
	})

	t.Run("NilAnnouncement", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("ValidationError", func(t *testing.T) {
		announcement := createTestAnnouncement(t, db, "Test", operator.ID)
		defer cleanupTestAnnouncement(t, db, announcement.ID)

		announcement.Type = "invalid"
		err := repo.Update(ctx, announcement)
		require.Error(t, err)
	})
}

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestAnnouncementRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := platform.NewAnnouncementRepository(db)
	ctx := testpkg.Ctx(t)

	operator := createTestOperator(t, db, "test@example.com", "Test Operator")
	defer cleanupTestOperator(t, db, operator.ID)

	announcement := createTestAnnouncement(t, db, "To Delete", operator.ID)

	err := repo.Delete(ctx, announcement.ID)
	require.NoError(t, err)

	// Verify deletion
	found, err := repo.FindByID(ctx, announcement.ID)
	require.NoError(t, err)
	assert.Nil(t, found)
}

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestAnnouncementRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := platform.NewAnnouncementRepository(db)
	ctx := testpkg.Ctx(t)

	operator := createTestOperator(t, db, "test@example.com", "Test Operator")
	defer cleanupTestOperator(t, db, operator.ID)

	// Create test announcements
	active := createTestAnnouncement(t, db, "Active", operator.ID)
	defer cleanupTestAnnouncement(t, db, active.ID)

	inactive := createTestAnnouncement(t, db, "Inactive", operator.ID)
	inactive.Active = false
	err := repo.Update(ctx, inactive)
	require.NoError(t, err)
	defer cleanupTestAnnouncement(t, db, inactive.ID)

	t.Run("ActiveOnly", func(t *testing.T) {
		announcements, err := repo.List(ctx, false)
		require.NoError(t, err)

		// Should contain at least our active announcement
		found := false
		for _, a := range announcements {
			if a.ID == active.ID {
				found = true
				assert.True(t, a.Active)
			}
			// All should be active
			assert.True(t, a.Active)
		}
		assert.True(t, found, "active announcement should be in list")
	})

	t.Run("IncludeInactive", func(t *testing.T) {
		announcements, err := repo.List(ctx, true)
		require.NoError(t, err)

		foundActive := false
		foundInactive := false
		for _, a := range announcements {
			if a.ID == active.ID {
				foundActive = true
			}
			if a.ID == inactive.ID {
				foundInactive = true
			}
		}
		assert.True(t, foundActive, "active announcement should be in list")
		assert.True(t, foundInactive, "inactive announcement should be in list")
	})
}

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestAnnouncementRepository_Publish(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := platform.NewAnnouncementRepository(db)
	ctx := testpkg.Ctx(t)

	operator := createTestOperator(t, db, "test@example.com", "Test Operator")
	defer cleanupTestOperator(t, db, operator.ID)

	announcement := createTestAnnouncement(t, db, "To Publish", operator.ID)
	defer cleanupTestAnnouncement(t, db, announcement.ID)

	// Initially not published
	assert.Nil(t, announcement.PublishedAt)

	err := repo.Publish(ctx, announcement.ID)
	require.NoError(t, err)

	// Verify published
	found, err := repo.FindByID(ctx, announcement.ID)
	require.NoError(t, err)
	require.NotNil(t, found.PublishedAt)
	assert.True(t, found.PublishedAt.Before(time.Now().Add(1*time.Second)))
}

// Deliberately NOT parallel: platform announcements and the e-mail outbox are
// tenant-less. Their fixtures reuse fixed operator e-mails (delete-then-insert)
// and the assertions count rows the whole clone shares, so two of these tests
// running side by side delete each other's operator and each other's rows.
func TestAnnouncementRepository_Unpublish(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := platform.NewAnnouncementRepository(db)
	ctx := testpkg.Ctx(t)

	operator := createTestOperator(t, db, "test@example.com", "Test Operator")
	defer cleanupTestOperator(t, db, operator.ID)

	announcement := createTestAnnouncement(t, db, "To Unpublish", operator.ID)
	defer cleanupTestAnnouncement(t, db, announcement.ID)

	// First publish it
	err := repo.Publish(ctx, announcement.ID)
	require.NoError(t, err)

	// Then unpublish
	err = repo.Unpublish(ctx, announcement.ID)
	require.NoError(t, err)

	// Verify unpublished
	found, err := repo.FindByID(ctx, announcement.ID)
	require.NoError(t, err)
	assert.Nil(t, found.PublishedAt)
}

// Helper functions for creating test data

func createTestOperator(t *testing.T, db *bun.DB, email, displayName string) *platformModels.Operator {
	t.Helper()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 5*time.Second)
	defer cancel()

	// These tests reuse fixed emails. Remove stale rows from previous runs so the
	// shared integration DB does not fail on platform.operators(email) uniqueness.
	_, err := db.ExecContext(ctx, `
		DELETE FROM platform.announcements
		WHERE created_by IN (
			SELECT id FROM platform.operators WHERE email = ?
		)`,
		email,
	)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `DELETE FROM platform.operators WHERE email = ?`, email)
	require.NoError(t, err)

	operator := &platformModels.Operator{
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: "dummy-hash",
		Active:       true,
	}

	_, err = db.NewInsert().
		Model(operator).
		ModelTableExpr(`platform.operators`).
		Exec(ctx)
	require.NoError(t, err)

	// Fetch the created operator to get the ID
	err = db.NewSelect().
		Model(operator).
		ModelTableExpr(`platform.operators AS "operator"`).
		Where(`"operator".email = ?`, email).
		Scan(ctx)
	require.NoError(t, err)

	return operator
}

func createTestAnnouncement(t *testing.T, db *bun.DB, title string, createdBy int64) *platformModels.Announcement {
	t.Helper()

	announcement := &platformModels.Announcement{
		Title:       title,
		Content:     "Test content for " + title,
		Type:        platformModels.TypeAnnouncement,
		Severity:    platformModels.SeverityInfo,
		Active:      true,
		CreatedBy:   createdBy,
		TargetRoles: []string{"all"},
	}

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 5*time.Second)
	defer cancel()

	err := db.NewInsert().
		Model(announcement).
		ModelTableExpr(`platform.announcements`).
		Scan(ctx)
	require.NoError(t, err)

	return announcement
}

func cleanupTestOperator(t *testing.T, db *bun.DB, operatorID int64) {
	t.Helper()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 5*time.Second)
	defer cancel()

	_, err := db.NewDelete().
		Model((*platformModels.Operator)(nil)).
		ModelTableExpr(`platform.operators`).
		Where("id = ?", operatorID).
		Exec(ctx)
	if err != nil {
		t.Logf("cleanup operator: %v", err)
	}
}

func cleanupTestAnnouncement(t *testing.T, db *bun.DB, announcementID int64) {
	t.Helper()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 5*time.Second)
	defer cancel()

	_, err := db.NewDelete().
		Model((*platformModels.Announcement)(nil)).
		ModelTableExpr(`platform.announcements`).
		Where("id = ?", announcementID).
		Exec(ctx)
	if err != nil {
		t.Logf("cleanup announcement: %v", err)
	}
}
