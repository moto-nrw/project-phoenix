//go:build testhooks

package timetracking

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
)

// scopeTestTenantRoutes binds real native gates supplied by the shared
// fixture catalog without adding an internal-test import of the HTTP runtime.
func scopeTestTenantRoutes(authenticate, scope func(http.Handler) http.Handler, preview func(context.Context) bool, principal common.PrincipalFactsReader, settingsCache, identityCache func(http.Handler) http.Handler) func(chi.Router, func(chi.Router, func(http.Handler) http.Handler)) {
	return func(r chi.Router, register func(chi.Router, func(http.Handler) http.Handler)) {
		common.ProtectedTenantRoutes(r, register, authenticate, common.ReadOnlyPreviewGuard(preview), scope, common.PrincipalFromFacts(principal), settingsCache, identityCache)
	}
}

// Router builds a standalone /staff router for this resource. The composition
// root registers the workforce routes into the shared /staff router via
// RegisterStaffRoutes instead, so membership and workforce keep one prefix.
func (rs *StaffAdminResource) Router(protectedTenantRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTenantRoutes(r, rs.RegisterStaffRoutes)
	return r
}

// CleanupOrphanedStaffDocumentFiles runs Workforce's recovery pass over the
// stored staff document files (workforce.StaffDocumentFileRecovery).
func (rs *StaffAdminResource) CleanupOrphanedStaffDocumentFiles(ctx context.Context) (int, error) {
	return rs.documentFiles.CleanupOrphanedStaffDocumentFiles(ctx)
}
