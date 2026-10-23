package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
)

// mountPublicRoutes registers unauthenticated root-level routes: the
// landing/health probes, the public image/legal-document servers, and the
// bearer-protected metrics endpoint.
func mountPublicRoutes(root chi.Router, metricsToken string) {
	root.Get("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("MOTO API - Phoenix Project"))
	})

	root.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("OK"))
	})

	// Note: Avatar files are served through authenticated endpoints, not as static files
	// This prevents unauthorized access to user avatars

	// Public login image serving (no auth - displayed on the login page before authentication)
	root.Get("/public/login-image/{filename}", func(w http.ResponseWriter, r *http.Request) {
		filename := chi.URLParam(r, "filename")
		apiCommon.ServeImage(w, r, "public/uploads/login-images", filename, "public, max-age=86400", nativeUploads.UploadFiles{})
	})

	// Public enrollment legal document serving (no auth - parents read it before submitting).
	root.Get("/public/enrollment-legal-documents/{filename}", func(w http.ResponseWriter, r *http.Request) {
		filename := chi.URLParam(r, "filename")
		apiCommon.ServeFile(w, r, "public/uploads/enrollment-legal-documents", filename, "public, max-age=86400", nativeUploads.UploadFiles{})
	})
	root.Get("/public/enrollment-form-legal-documents/{filename}", func(w http.ResponseWriter, r *http.Request) {
		filename := chi.URLParam(r, "filename")
		apiCommon.ServeFile(w, r, "public/uploads/enrollment-form-legal-documents", filename, "public, max-age=86400", nativeUploads.UploadFiles{})
	})

	root.With(metricsAuthMiddleware(metricsToken)).Handle("/internal/metrics", metricsHandler())
}

func newDatabaseStatsRouter(read DatabaseStatsReader, logger *slog.Logger) chi.Router {
	router := chi.NewRouter()
	ProtectedTenantRoutes(router, func(router chi.Router, withTx apiCommon.Middleware) {
		// The reader redacts every count the caller may not read, so the
		// route opens for any permission authorize.NewDatabaseStatsCapabilities
		// honours: the Datenverwaltung hub shows its counts to a lead role a
		// school defines itself, not only to system administrators (#3469).
		router.With(apiCommon.RequiresAnyPermission(DatabaseStatsPermissions()...), withTx).
			Get("/stats", func(w http.ResponseWriter, r *http.Request) { serveDatabaseStats(w, r, read, logger) })
	})
	return router
}

func serveDatabaseStats(w http.ResponseWriter, r *http.Request, read DatabaseStatsReader, logger *slog.Logger) {
	stats, err := read(r.Context())
	if err != nil {
		logger.Error("failed to get database stats",
			"error", err,
		)
		apiCommon.RenderError(w, r, apiCommon.ErrorInternalServerWrap("Internal server error", err))
		return
	}
	apiCommon.RespondWithJSON(w, r, http.StatusOK, stats)
}
