package platform_test

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"testing"
	"time"

	reposPlatform "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	"github.com/moto-nrw/project-phoenix/models/platform"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// --- Integration test helpers ---

// setupAnnouncementService wires up real repositories against the test database.
func setupAnnouncementService(t *testing.T) (platformSvc.AnnouncementService, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	announcementRepo := reposPlatform.NewAnnouncementRepository(db)
	viewRepo := reposPlatform.NewAnnouncementViewRepository(db)
	auditLogRepo := reposPlatform.NewOperatorAuditLogRepository(db)
	schoolRepo := reposPlatform.NewSchoolRepository(db)

	svc := platformSvc.NewAnnouncementService(platformSvc.AnnouncementServiceConfig{
		AnnouncementRepo:     announcementRepo,
		AnnouncementViewRepo: viewRepo,
		AuditLogRepo:         auditLogRepo,
		Organizations: &mockOrgRepoShared{countByIDsFn: func(ctx context.Context, ids []int64) (int, error) {
			return db.NewSelect().Model((*platform.Organization)(nil)).
				ModelTableExpr(`platform.organizations AS "organization"`).
				Where(`"organization".id IN (?)`, bun.List(ids)).
				Where(`"organization".deleted_at IS NULL`).
				Count(ctx)
		}},
		SchoolRepo: schoolRepo,
		DB:         db,
		Logger:     slog.Default(),
	})

	return svc, db
}

// createIntegrationOperator inserts a minimal operator for test announcements.
// Appends a nanosecond suffix to the email to avoid unique constraint collisions.
func createIntegrationOperator(t *testing.T, db *bun.DB, emailPrefix string) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	email := fmt.Sprintf("%s-%d@test.com", emailPrefix, time.Now().UnixNano())
	var id int64
	_, err := db.NewRaw(`
		INSERT INTO platform.operators (email, display_name, password_hash, active, created_at, updated_at)
		VALUES (?, ?, 'dummy-hash', true, NOW(), NOW())
		RETURNING id
	`, email, "Test Operator "+emailPrefix).Exec(ctx, &id)
	require.NoError(t, err)
	return id
}

func cleanupIntegrationOperator(t *testing.T, db *bun.DB, id int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewRaw(`DELETE FROM platform.operators WHERE id = ?`, id).Exec(ctx)
	if err != nil {
		t.Logf("cleanup operator %d: %v", id, err)
	}
}

func createIntegrationOrg(t *testing.T, db *bun.DB, name string) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var id int64
	_, err := db.NewRaw(`
		INSERT INTO platform.organizations (name, slug, active, created_at, updated_at)
		VALUES (?, ?, true, NOW(), NOW())
		RETURNING id
	`, name, name+"-"+suffix).Exec(ctx, &id)
	require.NoError(t, err)
	return id
}

func cleanupIntegrationOrg(t *testing.T, db *bun.DB, id int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = db.NewRaw(`DELETE FROM platform.organizations WHERE id = ?`, id).Exec(ctx)
}

func createIntegrationSchool(t *testing.T, db *bun.DB, name string, orgID int64) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var id int64
	_, err := db.NewRaw(`
		INSERT INTO platform.schools (name, slug, subdomain, organization_id, active, created_at, updated_at)
		VALUES (?, ?, ?, ?, true, NOW(), NOW())
		RETURNING id
	`, name, name+"-"+suffix, name+"-"+suffix, orgID).Exec(ctx, &id)
	require.NoError(t, err)
	return id
}

func cleanupIntegrationSchool(t *testing.T, db *bun.DB, id int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = db.NewRaw(`DELETE FROM platform.schools WHERE id = ?`, id).Exec(ctx)
}

func cleanupIntegrationAnnouncement(t *testing.T, db *bun.DB, id int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = db.NewRaw(`DELETE FROM platform.announcements WHERE id = ?`, id).Exec(ctx)
}

func cleanupIntegrationAuditLogs(t *testing.T, db *bun.DB, operatorID int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = db.NewRaw(`DELETE FROM platform.operator_audit_log WHERE operator_id = ?`, operatorID).Exec(ctx)
}

func createIntegrationAccount(t *testing.T, db *bun.DB, emailPrefix string) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	email := fmt.Sprintf("%s-%d@test.com", emailPrefix, time.Now().UnixNano())
	var id int64
	_, err := db.NewRaw(`
		INSERT INTO auth.accounts (email, password_hash, active, created_at, updated_at)
		VALUES (?, 'dummy-hash', true, NOW(), NOW())
		RETURNING id
	`, email).Exec(ctx, &id)
	require.NoError(t, err)
	return id
}

func cleanupIntegrationAccount(t *testing.T, db *bun.DB, id int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = db.NewRaw(`DELETE FROM auth.accounts WHERE id = ?`, id).Exec(ctx)
}

func createIntegrationAccountTenant(t *testing.T, db *bun.DB, accountID, tenantID int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewRaw(`
		INSERT INTO auth.account_tenants (account_id, tenant_id, status, created_at, updated_at)
		VALUES (?, ?, 'active', NOW(), NOW())
	`, accountID, tenantID).Exec(ctx)
	require.NoError(t, err)
}

func cleanupIntegrationAccountTenant(t *testing.T, db *bun.DB, accountID, tenantID int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = db.NewRaw(`DELETE FROM auth.account_tenants WHERE account_id = ? AND tenant_id = ?`, accountID, tenantID).Exec(ctx)
}

// --- Integration Tests: CreateAnnouncement ---

func TestAnnouncementServiceIntegration_CreateAnnouncement(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	svc, db := setupAnnouncementService(t)

	operatorID := createIntegrationOperator(t, db, "svc-create@test.com")
	defer cleanupIntegrationOperator(t, db, operatorID)
	defer cleanupIntegrationAuditLogs(t, db, operatorID)

	t.Run("creates global announcement", func(t *testing.T) {
		a := &platform.Announcement{
			Title:       "Integration Global",
			Content:     "Global announcement content",
			Type:        platform.TypeAnnouncement,
			Severity:    platform.SeverityInfo,
			TargetRoles: []string{},
		}

		err := svc.CreateAnnouncement(context.Background(), a, operatorID, net.ParseIP("127.0.0.1"))
		require.NoError(t, err)
		defer cleanupIntegrationAnnouncement(t, db, a.ID)

		assert.NotZero(t, a.ID)
		assert.Equal(t, operatorID, a.CreatedBy)
	})

	t.Run("creates org-targeted announcement", func(t *testing.T) {
		orgID := createIntegrationOrg(t, db, "svc-create-org")
		defer cleanupIntegrationOrg(t, db, orgID)

		a := &platform.Announcement{
			Title:        "Integration Org Targeted",
			Content:      "Org targeted content",
			Type:         platform.TypeAnnouncement,
			Severity:     platform.SeverityWarning,
			TargetOrgIDs: []int64{orgID},
		}

		err := svc.CreateAnnouncement(context.Background(), a, operatorID, net.ParseIP("127.0.0.1"))
		require.NoError(t, err)
		defer cleanupIntegrationAnnouncement(t, db, a.ID)

		assert.NotZero(t, a.ID)
	})

	t.Run("creates tenant-targeted announcement", func(t *testing.T) {
		orgID := createIntegrationOrg(t, db, "svc-create-tnt-org")
		defer cleanupIntegrationOrg(t, db, orgID)
		schoolID := createIntegrationSchool(t, db, "svc-create-tnt-school", orgID)
		defer cleanupIntegrationSchool(t, db, schoolID)

		a := &platform.Announcement{
			Title:           "Integration Tenant Targeted",
			Content:         "Tenant targeted content",
			Type:            platform.TypeMaintenance,
			Severity:        platform.SeverityCritical,
			TargetTenantIDs: []int64{schoolID},
		}

		err := svc.CreateAnnouncement(context.Background(), a, operatorID, net.ParseIP("127.0.0.1"))
		require.NoError(t, err)
		defer cleanupIntegrationAnnouncement(t, db, a.ID)

		assert.NotZero(t, a.ID)
	})

	t.Run("creates OR-union targeted announcement", func(t *testing.T) {
		orgID := createIntegrationOrg(t, db, "svc-create-union-org")
		defer cleanupIntegrationOrg(t, db, orgID)
		schoolID := createIntegrationSchool(t, db, "svc-create-union-school", orgID)
		defer cleanupIntegrationSchool(t, db, schoolID)

		a := &platform.Announcement{
			Title:           "Integration OR-Union",
			Content:         "OR-union content",
			Type:            platform.TypeRelease,
			Severity:        platform.SeverityInfo,
			TargetOrgIDs:    []int64{orgID},
			TargetTenantIDs: []int64{schoolID},
		}

		err := svc.CreateAnnouncement(context.Background(), a, operatorID, net.ParseIP("127.0.0.1"))
		require.NoError(t, err)
		defer cleanupIntegrationAnnouncement(t, db, a.ID)

		assert.NotZero(t, a.ID)
	})

	t.Run("rejects nil announcement", func(t *testing.T) {
		err := svc.CreateAnnouncement(context.Background(), nil, operatorID, net.ParseIP("127.0.0.1"))
		require.Error(t, err)
		assert.IsType(t, &platformSvc.InvalidDataError{}, err)
	})

	t.Run("rejects invalid announcement", func(t *testing.T) {
		a := &platform.Announcement{
			Title:   "",
			Content: "Content",
		}
		err := svc.CreateAnnouncement(context.Background(), a, operatorID, nil)
		require.Error(t, err)
		assert.IsType(t, &platformSvc.InvalidDataError{}, err)
	})

	t.Run("rejects nonexistent org IDs", func(t *testing.T) {
		a := &platform.Announcement{
			Title:        "Bad Org",
			Content:      "Content",
			Type:         platform.TypeAnnouncement,
			Severity:     platform.SeverityInfo,
			TargetOrgIDs: []int64{999999},
		}
		err := svc.CreateAnnouncement(context.Background(), a, operatorID, nil)
		require.Error(t, err)
		assert.IsType(t, &platformSvc.InvalidDataError{}, err)
		assert.Contains(t, err.Error(), "organization IDs do not exist")
	})

	t.Run("rejects nonexistent tenant IDs", func(t *testing.T) {
		a := &platform.Announcement{
			Title:           "Bad Tenant",
			Content:         "Content",
			Type:            platform.TypeAnnouncement,
			Severity:        platform.SeverityInfo,
			TargetTenantIDs: []int64{999999},
		}
		err := svc.CreateAnnouncement(context.Background(), a, operatorID, nil)
		require.Error(t, err)
		assert.IsType(t, &platformSvc.InvalidDataError{}, err)
		assert.Contains(t, err.Error(), "school (tenant) IDs do not exist")
	})

	t.Run("normalizes nil targeting arrays", func(t *testing.T) {
		a := &platform.Announcement{
			Title:           "Nil Arrays",
			Content:         "Content",
			Type:            platform.TypeAnnouncement,
			Severity:        platform.SeverityInfo,
			TargetOrgIDs:    nil,
			TargetTenantIDs: nil,
			TargetRoles:     nil,
		}
		err := svc.CreateAnnouncement(context.Background(), a, operatorID, nil)
		require.NoError(t, err)
		defer cleanupIntegrationAnnouncement(t, db, a.ID)

		// Fetch from DB to verify arrays are not NULL
		got, err := svc.GetAnnouncement(context.Background(), a.ID)
		require.NoError(t, err)
		assert.NotNil(t, got.TargetOrgIDs)
		assert.NotNil(t, got.TargetTenantIDs)
		assert.NotNil(t, got.TargetRoles)
	})
}

// --- Integration Tests: GetAnnouncement ---

func TestAnnouncementServiceIntegration_GetAnnouncement(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	svc, db := setupAnnouncementService(t)

	operatorID := createIntegrationOperator(t, db, "svc-get@test.com")
	defer cleanupIntegrationOperator(t, db, operatorID)
	defer cleanupIntegrationAuditLogs(t, db, operatorID)

	t.Run("returns existing announcement", func(t *testing.T) {
		a := &platform.Announcement{
			Title:    "Get Test",
			Content:  "Content",
			Type:     platform.TypeAnnouncement,
			Severity: platform.SeverityInfo,
		}
		err := svc.CreateAnnouncement(context.Background(), a, operatorID, nil)
		require.NoError(t, err)
		defer cleanupIntegrationAnnouncement(t, db, a.ID)

		got, err := svc.GetAnnouncement(context.Background(), a.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, a.ID, got.ID)
		assert.Equal(t, "Get Test", got.Title)
	})

	t.Run("returns not found for nonexistent ID", func(t *testing.T) {
		_, err := svc.GetAnnouncement(context.Background(), 999999)
		require.Error(t, err)
		assert.IsType(t, &platformSvc.AnnouncementNotFoundError{}, err)
	})
}

// --- Integration Tests: UpdateAnnouncement ---

func TestAnnouncementServiceIntegration_UpdateAnnouncement(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	svc, db := setupAnnouncementService(t)

	operatorID := createIntegrationOperator(t, db, "svc-update@test.com")
	defer cleanupIntegrationOperator(t, db, operatorID)
	defer cleanupIntegrationAuditLogs(t, db, operatorID)

	t.Run("updates title and content", func(t *testing.T) {
		a := &platform.Announcement{
			Title:    "Original Title",
			Content:  "Original Content",
			Type:     platform.TypeAnnouncement,
			Severity: platform.SeverityInfo,
		}
		err := svc.CreateAnnouncement(context.Background(), a, operatorID, nil)
		require.NoError(t, err)
		defer cleanupIntegrationAnnouncement(t, db, a.ID)

		a.Title = "Updated Title"
		a.Content = "Updated Content"
		err = svc.UpdateAnnouncement(context.Background(), a, operatorID, net.ParseIP("127.0.0.1"))
		require.NoError(t, err)

		got, err := svc.GetAnnouncement(context.Background(), a.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Title", got.Title)
		assert.Equal(t, "Updated Content", got.Content)
	})

	t.Run("updates targeting arrays", func(t *testing.T) {
		orgID := createIntegrationOrg(t, db, "svc-update-targeting-org")
		defer cleanupIntegrationOrg(t, db, orgID)
		schoolID := createIntegrationSchool(t, db, "svc-update-targeting-school", orgID)
		defer cleanupIntegrationSchool(t, db, schoolID)

		a := &platform.Announcement{
			Title:    "Update Targeting",
			Content:  "Content",
			Type:     platform.TypeAnnouncement,
			Severity: platform.SeverityInfo,
		}
		err := svc.CreateAnnouncement(context.Background(), a, operatorID, nil)
		require.NoError(t, err)
		defer cleanupIntegrationAnnouncement(t, db, a.ID)

		a.TargetOrgIDs = []int64{orgID}
		a.TargetTenantIDs = []int64{schoolID}
		err = svc.UpdateAnnouncement(context.Background(), a, operatorID, net.ParseIP("127.0.0.1"))
		require.NoError(t, err)

		got, err := svc.GetAnnouncement(context.Background(), a.ID)
		require.NoError(t, err)
		assert.Equal(t, []int64{orgID}, got.TargetOrgIDs)
		assert.Equal(t, []int64{schoolID}, got.TargetTenantIDs)
	})

	t.Run("rejects update with nonexistent org", func(t *testing.T) {
		a := &platform.Announcement{
			Title:    "Update Bad Org",
			Content:  "Content",
			Type:     platform.TypeAnnouncement,
			Severity: platform.SeverityInfo,
		}
		err := svc.CreateAnnouncement(context.Background(), a, operatorID, nil)
		require.NoError(t, err)
		defer cleanupIntegrationAnnouncement(t, db, a.ID)

		a.TargetOrgIDs = []int64{999999}
		err = svc.UpdateAnnouncement(context.Background(), a, operatorID, nil)
		require.Error(t, err)
		assert.IsType(t, &platformSvc.InvalidDataError{}, err)
	})

	t.Run("rejects update with nonexistent tenant", func(t *testing.T) {
		a := &platform.Announcement{
			Title:    "Update Bad Tenant",
			Content:  "Content",
			Type:     platform.TypeAnnouncement,
			Severity: platform.SeverityInfo,
		}
		err := svc.CreateAnnouncement(context.Background(), a, operatorID, nil)
		require.NoError(t, err)
		defer cleanupIntegrationAnnouncement(t, db, a.ID)

		a.TargetTenantIDs = []int64{999999}
		err = svc.UpdateAnnouncement(context.Background(), a, operatorID, nil)
		require.Error(t, err)
		assert.IsType(t, &platformSvc.InvalidDataError{}, err)
	})

	t.Run("rejects nil announcement", func(t *testing.T) {
		err := svc.UpdateAnnouncement(context.Background(), nil, operatorID, nil)
		require.Error(t, err)
		assert.IsType(t, &platformSvc.InvalidDataError{}, err)
	})

	t.Run("rejects update for nonexistent announcement", func(t *testing.T) {
		a := &platform.Announcement{
			Title:    "Nonexistent",
			Content:  "Content",
			Type:     platform.TypeAnnouncement,
			Severity: platform.SeverityInfo,
		}
		a.ID = 999999
		a.CreatedBy = operatorID
		err := svc.UpdateAnnouncement(context.Background(), a, operatorID, nil)
		require.Error(t, err)
		assert.IsType(t, &platformSvc.AnnouncementNotFoundError{}, err)
	})

	t.Run("rejects invalid data on update", func(t *testing.T) {
		a := &platform.Announcement{
			Title:    "Valid",
			Content:  "Content",
			Type:     platform.TypeAnnouncement,
			Severity: platform.SeverityInfo,
		}
		err := svc.CreateAnnouncement(context.Background(), a, operatorID, nil)
		require.NoError(t, err)
		defer cleanupIntegrationAnnouncement(t, db, a.ID)

		a.Title = ""
		err = svc.UpdateAnnouncement(context.Background(), a, operatorID, nil)
		require.Error(t, err)
		assert.IsType(t, &platformSvc.InvalidDataError{}, err)
	})
}

// --- Integration Tests: DeleteAnnouncement ---

func TestAnnouncementServiceIntegration_DeleteAnnouncement(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	svc, db := setupAnnouncementService(t)

	operatorID := createIntegrationOperator(t, db, "svc-delete@test.com")
	defer cleanupIntegrationOperator(t, db, operatorID)
	defer cleanupIntegrationAuditLogs(t, db, operatorID)

	t.Run("deletes existing announcement", func(t *testing.T) {
		a := &platform.Announcement{
			Title:    "To Delete",
			Content:  "Content",
			Type:     platform.TypeAnnouncement,
			Severity: platform.SeverityInfo,
		}
		err := svc.CreateAnnouncement(context.Background(), a, operatorID, nil)
		require.NoError(t, err)

		err = svc.DeleteAnnouncement(context.Background(), a.ID, operatorID, net.ParseIP("127.0.0.1"))
		require.NoError(t, err)

		_, err = svc.GetAnnouncement(context.Background(), a.ID)
		require.Error(t, err)
		assert.IsType(t, &platformSvc.AnnouncementNotFoundError{}, err)
	})

	t.Run("rejects delete for nonexistent announcement", func(t *testing.T) {
		err := svc.DeleteAnnouncement(context.Background(), 999999, operatorID, nil)
		require.Error(t, err)
		assert.IsType(t, &platformSvc.AnnouncementNotFoundError{}, err)
	})
}

// --- Integration Tests: Publish/Unpublish ---

func TestAnnouncementServiceIntegration_PublishUnpublish(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	svc, db := setupAnnouncementService(t)

	operatorID := createIntegrationOperator(t, db, "svc-publish@test.com")
	defer cleanupIntegrationOperator(t, db, operatorID)
	defer cleanupIntegrationAuditLogs(t, db, operatorID)

	t.Run("publish sets published_at", func(t *testing.T) {
		a := &platform.Announcement{
			Title:    "To Publish",
			Content:  "Content",
			Type:     platform.TypeAnnouncement,
			Severity: platform.SeverityInfo,
		}
		err := svc.CreateAnnouncement(context.Background(), a, operatorID, nil)
		require.NoError(t, err)
		defer cleanupIntegrationAnnouncement(t, db, a.ID)

		err = svc.PublishAnnouncement(context.Background(), a.ID, operatorID, net.ParseIP("127.0.0.1"))
		require.NoError(t, err)

		got, err := svc.GetAnnouncement(context.Background(), a.ID)
		require.NoError(t, err)
		require.NotNil(t, got.PublishedAt)
	})

	t.Run("unpublish clears published_at", func(t *testing.T) {
		a := &platform.Announcement{
			Title:    "To Unpublish",
			Content:  "Content",
			Type:     platform.TypeAnnouncement,
			Severity: platform.SeverityInfo,
		}
		err := svc.CreateAnnouncement(context.Background(), a, operatorID, nil)
		require.NoError(t, err)
		defer cleanupIntegrationAnnouncement(t, db, a.ID)

		err = svc.PublishAnnouncement(context.Background(), a.ID, operatorID, nil)
		require.NoError(t, err)

		err = svc.UnpublishAnnouncement(context.Background(), a.ID, operatorID, net.ParseIP("127.0.0.1"))
		require.NoError(t, err)

		got, err := svc.GetAnnouncement(context.Background(), a.ID)
		require.NoError(t, err)
		assert.Nil(t, got.PublishedAt)
	})

	t.Run("publish nonexistent announcement fails", func(t *testing.T) {
		err := svc.PublishAnnouncement(context.Background(), 999999, operatorID, nil)
		require.Error(t, err)
		assert.IsType(t, &platformSvc.AnnouncementNotFoundError{}, err)
	})

	t.Run("unpublish nonexistent announcement fails", func(t *testing.T) {
		err := svc.UnpublishAnnouncement(context.Background(), 999999, operatorID, nil)
		require.Error(t, err)
		assert.IsType(t, &platformSvc.AnnouncementNotFoundError{}, err)
	})
}

// --- Integration Tests: ListAnnouncements ---

func TestAnnouncementServiceIntegration_ListAnnouncements(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	svc, db := setupAnnouncementService(t)

	operatorID := createIntegrationOperator(t, db, "svc-list@test.com")
	defer cleanupIntegrationOperator(t, db, operatorID)
	defer cleanupIntegrationAuditLogs(t, db, operatorID)

	a1 := &platform.Announcement{
		Title:    "List Active",
		Content:  "Content",
		Type:     platform.TypeAnnouncement,
		Severity: platform.SeverityInfo,
		Active:   true,
	}
	err := svc.CreateAnnouncement(context.Background(), a1, operatorID, nil)
	require.NoError(t, err)
	defer cleanupIntegrationAnnouncement(t, db, a1.ID)

	a2 := &platform.Announcement{
		Title:    "List Inactive",
		Content:  "Content",
		Type:     platform.TypeAnnouncement,
		Severity: platform.SeverityInfo,
		Active:   true,
	}
	err = svc.CreateAnnouncement(context.Background(), a2, operatorID, nil)
	require.NoError(t, err)
	defer cleanupIntegrationAnnouncement(t, db, a2.ID)

	// Deactivate a2 via update
	a2.Active = false
	err = svc.UpdateAnnouncement(context.Background(), a2, operatorID, nil)
	require.NoError(t, err)

	t.Run("active only", func(t *testing.T) {
		list, err := svc.ListAnnouncements(context.Background(), false)
		require.NoError(t, err)

		foundA1, foundA2 := false, false
		for _, a := range list {
			if a.ID == a1.ID {
				foundA1 = true
			}
			if a.ID == a2.ID {
				foundA2 = true
			}
		}
		assert.True(t, foundA1, "active announcement should be in list")
		assert.False(t, foundA2, "inactive announcement should not be in active-only list")
	})

	t.Run("include inactive", func(t *testing.T) {
		list, err := svc.ListAnnouncements(context.Background(), true)
		require.NoError(t, err)

		foundA1, foundA2 := false, false
		for _, a := range list {
			if a.ID == a1.ID {
				foundA1 = true
			}
			if a.ID == a2.ID {
				foundA2 = true
			}
		}
		assert.True(t, foundA1)
		assert.True(t, foundA2, "inactive announcement should be in include-inactive list")
	})
}

// --- Integration Tests: GetUnreadForUser / CountUnread (service pass-through) ---

func TestAnnouncementServiceIntegration_UnreadAndCount(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	svc, db := setupAnnouncementService(t)

	operatorID := createIntegrationOperator(t, db, "svc-unread@test.com")
	defer cleanupIntegrationOperator(t, db, operatorID)
	defer cleanupIntegrationAuditLogs(t, db, operatorID)

	orgID := createIntegrationOrg(t, db, "svc-unread-org")
	defer cleanupIntegrationOrg(t, db, orgID)
	schoolID := createIntegrationSchool(t, db, "svc-unread-school", orgID)
	defer cleanupIntegrationSchool(t, db, schoolID)

	accountID := createIntegrationAccount(t, db, "svc-unread-user@test.com")
	defer cleanupIntegrationAccount(t, db, accountID)
	createIntegrationAccountTenant(t, db, accountID, schoolID)
	defer cleanupIntegrationAccountTenant(t, db, accountID, schoolID)

	// Create and publish a global announcement
	a := &platform.Announcement{
		Title:    "Unread Global",
		Content:  "Content",
		Type:     platform.TypeAnnouncement,
		Severity: platform.SeverityInfo,
	}
	err := svc.CreateAnnouncement(context.Background(), a, operatorID, nil)
	require.NoError(t, err)
	defer cleanupIntegrationAnnouncement(t, db, a.ID)

	err = svc.PublishAnnouncement(context.Background(), a.ID, operatorID, nil)
	require.NoError(t, err)

	// Get baseline
	baselineCount, err := svc.CountUnread(context.Background(), accountID, []string{"teacher"}, schoolID, orgID)
	require.NoError(t, err)

	t.Run("count includes published global announcement", func(t *testing.T) {
		assert.GreaterOrEqual(t, baselineCount, 1)
	})

	t.Run("GetUnreadForUser returns published global announcement", func(t *testing.T) {
		announcements, err := svc.GetUnreadForUser(context.Background(), accountID, []string{"teacher"}, schoolID, orgID)
		require.NoError(t, err)

		found := false
		for _, ann := range announcements {
			if ann.ID == a.ID {
				found = true
			}
		}
		assert.True(t, found, "published global announcement should be unread")
	})

	t.Run("MarkSeen excludes from unread", func(t *testing.T) {
		err := svc.MarkSeen(context.Background(), accountID, a.ID)
		require.NoError(t, err)

		count, err := svc.CountUnread(context.Background(), accountID, []string{"teacher"}, schoolID, orgID)
		require.NoError(t, err)
		assert.Equal(t, baselineCount-1, count)
	})
}

// --- Integration Tests: GetStats (service level with existence check) ---

func TestAnnouncementServiceIntegration_GetStats(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	svc, db := setupAnnouncementService(t)

	operatorID := createIntegrationOperator(t, db, "svc-stats@test.com")
	defer cleanupIntegrationOperator(t, db, operatorID)
	defer cleanupIntegrationAuditLogs(t, db, operatorID)

	t.Run("returns stats for existing announcement", func(t *testing.T) {
		orgID := createIntegrationOrg(t, db, "svc-stats-org")
		defer cleanupIntegrationOrg(t, db, orgID)
		schoolID := createIntegrationSchool(t, db, "svc-stats-school", orgID)
		defer cleanupIntegrationSchool(t, db, schoolID)

		accountID := createIntegrationAccount(t, db, "svc-stats-user@test.com")
		defer cleanupIntegrationAccount(t, db, accountID)
		createIntegrationAccountTenant(t, db, accountID, schoolID)
		defer cleanupIntegrationAccountTenant(t, db, accountID, schoolID)

		a := &platform.Announcement{
			Title:           "Stats Tenant",
			Content:         "Content",
			Type:            platform.TypeAnnouncement,
			Severity:        platform.SeverityInfo,
			TargetTenantIDs: []int64{schoolID},
		}
		err := svc.CreateAnnouncement(context.Background(), a, operatorID, nil)
		require.NoError(t, err)
		defer cleanupIntegrationAnnouncement(t, db, a.ID)

		stats, err := svc.GetStats(context.Background(), a.ID)
		require.NoError(t, err)
		assert.Equal(t, a.ID, stats.AnnouncementID)
		assert.Equal(t, 1, stats.TargetCount)
		assert.Equal(t, 0, stats.SeenCount)
		assert.Equal(t, 0, stats.DismissedCount)
	})

	t.Run("returns not found for nonexistent announcement", func(t *testing.T) {
		_, err := svc.GetStats(context.Background(), 999999)
		require.Error(t, err)
		assert.IsType(t, &platformSvc.AnnouncementNotFoundError{}, err)
	})
}

// --- Integration Tests: GetViewDetails (service level with existence check) ---

func TestAnnouncementServiceIntegration_GetViewDetails(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	svc, db := setupAnnouncementService(t)

	operatorID := createIntegrationOperator(t, db, "svc-viewdetails@test.com")
	defer cleanupIntegrationOperator(t, db, operatorID)
	defer cleanupIntegrationAuditLogs(t, db, operatorID)

	t.Run("returns details for existing announcement", func(t *testing.T) {
		a := &platform.Announcement{
			Title:    "View Details",
			Content:  "Content",
			Type:     platform.TypeAnnouncement,
			Severity: platform.SeverityInfo,
		}
		err := svc.CreateAnnouncement(context.Background(), a, operatorID, nil)
		require.NoError(t, err)
		defer cleanupIntegrationAnnouncement(t, db, a.ID)

		accountID := createIntegrationAccount(t, db, "svc-viewdetails-user@test.com")
		defer cleanupIntegrationAccount(t, db, accountID)

		err = svc.MarkSeen(context.Background(), accountID, a.ID)
		require.NoError(t, err)

		details, err := svc.GetViewDetails(context.Background(), a.ID)
		require.NoError(t, err)
		require.Len(t, details, 1)
		assert.Equal(t, accountID, details[0].UserID)
	})

	t.Run("returns not found for nonexistent announcement", func(t *testing.T) {
		_, err := svc.GetViewDetails(context.Background(), 999999)
		require.Error(t, err)
		assert.IsType(t, &platformSvc.AnnouncementNotFoundError{}, err)
	})
}

// --- Integration Tests: MarkDismissed ---

func TestAnnouncementServiceIntegration_MarkDismissed(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	svc, db := setupAnnouncementService(t)

	operatorID := createIntegrationOperator(t, db, "svc-dismiss@test.com")
	defer cleanupIntegrationOperator(t, db, operatorID)
	defer cleanupIntegrationAuditLogs(t, db, operatorID)

	a := &platform.Announcement{
		Title:    "Dismiss Test",
		Content:  "Content",
		Type:     platform.TypeAnnouncement,
		Severity: platform.SeverityInfo,
	}
	err := svc.CreateAnnouncement(context.Background(), a, operatorID, nil)
	require.NoError(t, err)
	defer cleanupIntegrationAnnouncement(t, db, a.ID)

	accountID := createIntegrationAccount(t, db, "svc-dismiss-user@test.com")
	defer cleanupIntegrationAccount(t, db, accountID)

	err = svc.MarkDismissed(context.Background(), accountID, a.ID)
	require.NoError(t, err)

	details, err := svc.GetViewDetails(context.Background(), a.ID)
	require.NoError(t, err)
	require.Len(t, details, 1)
	assert.True(t, details[0].Dismissed)
}

// --- Integration Tests: Audit Log Verification ---

func TestAnnouncementServiceIntegration_AuditLogging(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	svc, db := setupAnnouncementService(t)

	operatorID := createIntegrationOperator(t, db, "svc-audit@test.com")
	defer cleanupIntegrationOperator(t, db, operatorID)
	defer cleanupIntegrationAuditLogs(t, db, operatorID)

	t.Run("create logs audit entry", func(t *testing.T) {
		a := &platform.Announcement{
			Title:    "Audit Create",
			Content:  "Content",
			Type:     platform.TypeAnnouncement,
			Severity: platform.SeverityInfo,
		}
		err := svc.CreateAnnouncement(context.Background(), a, operatorID, net.ParseIP("10.0.0.1"))
		require.NoError(t, err)
		defer cleanupIntegrationAnnouncement(t, db, a.ID)

		// Verify audit log was created
		var count int
		err = db.NewRaw(`
			SELECT COUNT(*) FROM platform.operator_audit_log
			WHERE operator_id = ? AND action = 'create' AND resource_type = 'announcement' AND resource_id = ?
		`, operatorID, a.ID).Scan(context.Background(), &count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "create should produce an audit log entry")
	})

	t.Run("update logs audit entry with changes", func(t *testing.T) {
		a := &platform.Announcement{
			Title:       "Audit Update",
			Content:     "Content",
			Type:        platform.TypeAnnouncement,
			Severity:    platform.SeverityInfo,
			TargetRoles: []string{"admin"},
		}
		err := svc.CreateAnnouncement(context.Background(), a, operatorID, nil)
		require.NoError(t, err)
		defer cleanupIntegrationAnnouncement(t, db, a.ID)

		a.Title = "Audit Update Modified"
		a.TargetRoles = []string{"admin", "teacher"}
		err = svc.UpdateAnnouncement(context.Background(), a, operatorID, net.ParseIP("10.0.0.2"))
		require.NoError(t, err)

		// Verify update audit log
		var count int
		err = db.NewRaw(`
			SELECT COUNT(*) FROM platform.operator_audit_log
			WHERE operator_id = ? AND action = 'update' AND resource_type = 'announcement' AND resource_id = ?
		`, operatorID, a.ID).Scan(context.Background(), &count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "update should produce an audit log entry")
	})

	t.Run("delete logs audit entry", func(t *testing.T) {
		a := &platform.Announcement{
			Title:    "Audit Delete",
			Content:  "Content",
			Type:     platform.TypeAnnouncement,
			Severity: platform.SeverityInfo,
		}
		err := svc.CreateAnnouncement(context.Background(), a, operatorID, nil)
		require.NoError(t, err)
		deletedID := a.ID

		err = svc.DeleteAnnouncement(context.Background(), a.ID, operatorID, net.ParseIP("10.0.0.3"))
		require.NoError(t, err)

		var count int
		err = db.NewRaw(`
			SELECT COUNT(*) FROM platform.operator_audit_log
			WHERE operator_id = ? AND action = 'delete' AND resource_type = 'announcement' AND resource_id = ?
		`, operatorID, deletedID).Scan(context.Background(), &count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "delete should produce an audit log entry")
	})

	t.Run("publish logs audit entry", func(t *testing.T) {
		a := &platform.Announcement{
			Title:    "Audit Publish",
			Content:  "Content",
			Type:     platform.TypeAnnouncement,
			Severity: platform.SeverityInfo,
		}
		err := svc.CreateAnnouncement(context.Background(), a, operatorID, nil)
		require.NoError(t, err)
		defer cleanupIntegrationAnnouncement(t, db, a.ID)

		err = svc.PublishAnnouncement(context.Background(), a.ID, operatorID, net.ParseIP("10.0.0.4"))
		require.NoError(t, err)

		var count int
		err = db.NewRaw(`
			SELECT COUNT(*) FROM platform.operator_audit_log
			WHERE operator_id = ? AND action = 'publish' AND resource_type = 'announcement' AND resource_id = ?
		`, operatorID, a.ID).Scan(context.Background(), &count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "publish should produce an audit log entry")
	})
}
