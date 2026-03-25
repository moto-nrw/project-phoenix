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

func TestAnnouncementViewRepository_GetViewDetails(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	viewRepo := platform.NewAnnouncementViewRepository(db)

	// Create operator for announcement
	operator := createTestOperator(t, db, "viewtest@example.com", "View Test Operator")
	defer cleanupTestOperator(t, db, operator.ID)

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
	err := annoRepo.Create(ctx, announcement)
	require.NoError(t, err)
	defer cleanupTestAnnouncement(t, db, announcement.ID)

	// Create an auth account
	accountID := createTestAccount(t, db, "viewuser@example.com")
	defer cleanupTestAccount(t, db, accountID)

	// Create a person linked to that account
	personID := createTestPerson(t, db, accountID, "Max", "Mustermann")
	defer cleanupTestPerson(t, db, personID)

	// Mark announcement as seen + dismissed
	err = viewRepo.MarkSeen(ctx, accountID, announcement.ID)
	require.NoError(t, err)
	err = viewRepo.MarkDismissed(ctx, accountID, announcement.ID)
	require.NoError(t, err)
	defer cleanupTestAnnouncementView(t, db, accountID, announcement.ID)

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
		defer cleanupTestAccount(t, db, orphanAccountID)

		err := viewRepo.MarkSeen(ctx, orphanAccountID, announcement.ID)
		require.NoError(t, err)
		err = viewRepo.MarkDismissed(ctx, orphanAccountID, announcement.ID)
		require.NoError(t, err)
		defer cleanupTestAnnouncementView(t, db, orphanAccountID, announcement.ID)

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
	return id
}

func cleanupTestAccount(t *testing.T, db *bun.DB, accountID int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewRaw(`DELETE FROM auth.accounts WHERE id = ?`, accountID).Exec(ctx)
	if err != nil {
		t.Logf("cleanup account %d: %v", accountID, err)
	}
}

func createTestPerson(t *testing.T, db *bun.DB, accountID int64, firstName, lastName string) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id int64
	_, err := db.NewRaw(`
		INSERT INTO users.persons (account_id, first_name, last_name, tenant_id, created_at, updated_at)
		VALUES (?, ?, ?, 1, NOW(), NOW())
		RETURNING id
	`, accountID, firstName, lastName).Exec(ctx, &id)
	require.NoError(t, err)
	return id
}

func cleanupTestPerson(t *testing.T, db *bun.DB, personID int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewRaw(`DELETE FROM users.persons WHERE id = ?`, personID).Exec(ctx)
	if err != nil {
		t.Logf("cleanup person %d: %v", personID, err)
	}
}

func cleanupTestAnnouncementView(t *testing.T, db *bun.DB, userID, announcementID int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewRaw(`DELETE FROM platform.announcement_views WHERE user_id = ? AND announcement_id = ?`, userID, announcementID).Exec(ctx)
	if err != nil {
		t.Logf("cleanup announcement view user=%d announcement=%d: %v", userID, announcementID, err)
	}
}
