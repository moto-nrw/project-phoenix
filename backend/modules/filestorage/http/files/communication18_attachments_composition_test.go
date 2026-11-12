//go:build testhooks

package files_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"testing"

	"github.com/go-chi/chi/v5"
	rootAPI "github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/modules/delivery/objects"
	filestorage "github.com/moto-ogs/project-phoenix/modules/filestorage"
	filesHTTP "github.com/moto-ogs/project-phoenix/modules/filestorage/http/files"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/uptrace/bun"
)

// Communication18AttachmentRouterForTest wires the same real audience entry,
// settings and tenant File Storage reads as mountDeliveryRoutes. It is a
// test-binary-only composition helper over existing typed-root test support.
// The returned portal exposes its native announcement port for this wiring;
// no implementation import, fake audience, policy allowance or public hook is added.
func Communication18AttachmentRouterForTest(t *testing.T, ctx context.Context, pool *bun.DB) (chi.Router, error) {
	t.Helper()
	testpkg.SetupTestDB(t)
	db := testpkg.NewDatabase(t, pool)
	unit := testpkg.TenantRuntime(t, pool)
	objects, err := objects.PrivateUploadsBackend()
	if err != nil {
		return nil, err
	}
	files, settings, err := rootAPI.NewFileStoreTestModule(db, unit, objects)
	if err != nil {
		return nil, err
	}
	if err := settings.SetValue(ctx, "operations.parent_news_enabled", true, nil, nil); err != nil {
		return nil, err
	}
	rootSettings, err := owners.NewSettingsTestModule(db, unit)
	if err != nil {
		return nil, err
	}
	repos := rootAPI.NewParentMessagingSuiteRepositories(db)
	portal, err := rootAPI.NewParentCareScheduleTestService(db, rootAPI.StudentTestModule{}, rootAPI.ActiveTestModule{}, rootAPI.ActiveSettings{Settings: rootSettings.Settings})
	if err != nil {
		return nil, err
	}
	portal.AnnouncementRepo = owners.BindParentAnnouncementAccess(repos.ParentAnnouncement)

	reads, err := filestorage.TenantReads(files)
	if err != nil {
		return nil, err
	}
	guardian, err := owners.NewParentGuardianAttachments(portal, reads)
	if err != nil {
		return nil, err
	}
	resource := filesHTTP.NewResource(files, slog.Default())
	router := chi.NewRouter()
	// Parent groups do not attach the host caches themselves; match the
	// production host here. Tenant groups reuse these idempotently.
	router.Use(rootAPI.RequestSettingsCacheMiddleware, rootAPI.RequestIdentityCacheMiddleware)
	router.Mount("/api/announcement-attachments", resource.AnnouncementAttachmentRouter(rootAPI.ProtectedTenantRoutes))
	router.Mount("/parent-news-attachments", resource.ParentAnnouncementAttachmentRouter(rootAPI.ProtectedParentGroup, guardian))
	return router, nil
}

func Communication18AttachmentCleanupForTest(tenantID int64) error {
	pub, err := objects.ResolvePublicDir()
	if err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(pub, "uploads", "announcement-attachments", fmt.Sprint(tenantID)))
}
