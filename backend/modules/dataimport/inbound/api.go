package importapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
)

const (
	maxFileSize = 10 * 1024 * 1024 // 10MB

	// Error messages (S1192 - avoid duplicate string literals)
	errTemplateCreation = "Fehler beim Erstellen der Vorlage"

	// Route paths shared by every import domain (S1192)
	routeTemplate = "/template"
	routePreview  = "/preview"
	routeImport   = "/import"

	// Permissions (S1192)
	permUsersRead          = "users:read"
	permUsersCreate        = "users:create"
	permTimeTrackingManage = "time_tracking:manage"
)

// Resource serves import routes over the public Data Import contract.
type Resource struct {
	files                  importModels.FileDecoder
	studentImportService   importModels.RowImporter[importModels.StudentImportRow]
	staffImportService     importModels.RowImporter[importModels.StaffImportRow]
	classListImportService importModels.RowImporter[importModels.ClassListEntryImportRow]
	runtime                Runtime
}

func NewResource(deps Dependencies) *Resource {
	return &Resource{files: deps.Files, studentImportService: deps.Students,
		staffImportService: deps.Staff, classListImportService: deps.ClassList, runtime: deps.Runtime}
}

// Router returns a configured router for import endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	r.Group(func(r chi.Router) {
		r.Use(rs.runtime.Middleware...)
		withTx := rs.runtime.TemplateTransaction

		// Student import endpoints
		r.Route("/students", func(r chi.Router) {
			// Template download - requires UsersRead
			r.With(rs.runtime.RequireAnyPermission(permUsersRead), withTx).Get(routeTemplate, rs.downloadStudentTemplate)

			// Preview - requires UsersCreate
			// Note: no withTx here — the handler owns its tenant transaction
			// so the GDPR audit row is committed before the success response.
			r.With(rs.runtime.RequireAnyPermission(permUsersCreate)).Post(routePreview, rs.previewStudentImport)

			// Actual import - requires UsersCreate
			// No withTx: the workflow commits bounded batches and checkpoints.
			r.With(rs.runtime.RequireAnyPermission(permUsersCreate)).Post(routeImport, rs.importStudents)
		})

		// Opening balance (Eröffnungssalden) import endpoints (#2132).
		// Stundenkonto and vacation takeover values are payroll data —
		// everything sits behind time_tracking:manage.
		r.Route("/opening-balances", func(r chi.Router) {
			r.With(rs.runtime.RequireAnyPermission(permTimeTrackingManage), withTx).Get(routeTemplate, rs.DownloadOpeningBalanceTemplate)
			// Note: no withTx on the preview either — the handler owns its
			// tenant transaction so the GDPR audit row is committed before
			// the success response.
			r.With(rs.runtime.RequireAnyPermission(permTimeTrackingManage)).Post(routePreview, rs.PreviewOpeningBalanceImport)
			// No withTx: the workflow commits bounded batches and checkpoints.
			r.With(rs.runtime.RequireAnyPermission(permTimeTrackingManage)).Post(routeImport, rs.ImportOpeningBalances)
		})

		// Class-list entry (Klassenlisteneintrag, #2382) import endpoints
		r.Route("/class-list-entries", func(r chi.Router) {
			r.With(rs.runtime.RequireAnyPermission(permUsersRead), withTx).Get(routeTemplate, rs.DownloadClassListTemplate)
			// No withTx: the workflow commits its audit before the response,
			// in one preview transaction or bounded import transactions.
			r.With(rs.runtime.RequireAnyPermission(permUsersCreate)).Post(routePreview, rs.PreviewClassListImport)
			r.With(rs.runtime.RequireAnyPermission(permUsersCreate)).Post(routeImport, rs.ImportClassList)
		})

		// Staff (Mitarbeiter) import endpoints
		r.Route("/teachers", func(r chi.Router) {
			// Template download - the template carries no tenant data, so
			// everyone who may import (users:create) gets it as well as the
			// directory readers; the import page opens on users:create alone
			// (#2906).
			r.With(rs.runtime.RequireAnyPermission(permUsersRead, permUsersCreate), withTx).Get(routeTemplate, rs.DownloadStaffTemplate)

			// Preview - requires UsersCreate
			// Note: no withTx here — the handler owns its tenant transaction
			// so the GDPR audit row is committed before the success response.
			r.With(rs.runtime.RequireAnyPermission(permUsersCreate)).Post(routePreview, rs.PreviewStaffImport)

			// Actual import - requires UsersCreate
			// No withTx: the workflow commits bounded batches and checkpoints.
			r.With(rs.runtime.RequireAnyPermission(permUsersCreate)).Post(routeImport, rs.ImportStaff)
		})
	})

	return r
}

// downloadStudentTemplate handles template download (CSV or Excel)
func (rs *Resource) downloadStudentTemplate(w http.ResponseWriter, r *http.Request) {
	rs.downloadTemplate(w, r, importModels.StudentTemplate, "schueler-import-vorlage")
}

// previewStudentImport handles import preview (dry-run)
func (rs *Resource) previewStudentImport(w http.ResponseWriter, r *http.Request) {
	// Validate and parse CSV file
	uploadResult, ok := rs.validateAndParseCSVFile(w, r)
	if !ok {
		return // Error already handled by validateAndParseCSVFile
	}
	mode, ok := rs.importModeFromRequest(w, r)
	if !ok {
		return
	}

	// Get account ID for audit logging (GDPR: audit tracks auth identity)
	accountID, err := rs.runtime.AccountID(r.Context())
	if err != nil {
		rs.runtime.Failure(w, r, Failure{Status: http.StatusUnauthorized, Cause: err})
		return
	}

	// The preview owns its tenant transaction (no route-level withTx): the
	// GDPR audit row must be committed before the success response is
	// written — a middleware transaction commits only after the handler
	// returns, when a commit failure can no longer be reported. Staff ID
	// resolution happens inside the TX because the lookup is RLS-scoped.
	tenantID := rs.runtime.TenantID(r.Context())
	var result *importModels.ImportResult[importModels.StudentImportRow]
	var staffResolutionErr error
	if err := rs.runtime.WithinTenant(r.Context(), func(ctx context.Context) error {
		staffID, staffErr := rs.runtime.StaffID(ctx)
		if staffErr != nil {
			staffResolutionErr = staffErr
			return staffErr
		}

		request := importModels.ImportRequest[importModels.StudentImportRow]{
			Rows:            uploadResult.Rows,
			Mode:            mode,
			DryRun:          true,  // PREVIEW ONLY
			StopOnError:     false, // Collect all errors
			UserID:          staffID,
			SkipInvalidRows: false,
		}

		var txErr error
		result, txErr = rs.studentImportService.Import(ctx, request)
		if txErr != nil {
			return txErr
		}
		// GDPR Compliance: Audit log for preview (Article 30).
		return rs.studentImportService.RecordAuditInTransaction(ctx, "student", uploadResult.Filename, result, accountID, true, tenantID)
	}); err != nil {
		if staffResolutionErr != nil {
			rs.runtime.Failure(w, r, Failure{Status: http.StatusUnauthorized, Cause: staffResolutionErr})
			return
		}
		rs.runtime.Failure(w, r, Failure{Status: http.StatusInternalServerError, Cause: err, Message: "Import-Vorschau fehlgeschlagen"})
		return
	}

	rs.runtime.Success(w, r, http.StatusOK, result, "Import-Vorschau erfolgreich")
}

// importStudents handles actual student import
func (rs *Resource) importStudents(w http.ResponseWriter, r *http.Request) {
	// Validate and parse CSV file
	uploadResult, ok := rs.validateAndParseCSVFile(w, r)
	if !ok {
		return // Error already handled by validateAndParseCSVFile
	}
	mode, ok := rs.importModeFromRequest(w, r)
	if !ok {
		return
	}

	// Get account ID for audit logging (GDPR: audit tracks auth identity)
	accountID, err := rs.runtime.AccountID(r.Context())
	if err != nil {
		rs.runtime.Failure(w, r, Failure{Status: http.StatusUnauthorized, Cause: err})
		return
	}

	// Resolve the actor in a short tenant transaction. The workflow owns the
	// subsequent bounded write transactions and their audit checkpoints.
	var staffID int64
	if err := rs.runtime.WithinTenant(r.Context(), func(ctx context.Context) error {
		var err error
		staffID, err = rs.runtime.StaffID(ctx)
		return err
	}); err != nil {
		rs.runtime.Failure(w, r, Failure{Status: http.StatusInternalServerError, Cause: err, Message: "Import fehlgeschlagen"})
		return
	}
	result, err := rs.studentImportService.ImportBatches(r.Context(), importModels.ImportRequest[importModels.StudentImportRow]{
		Rows: uploadResult.Rows, Mode: mode, UserID: staffID, SkipInvalidRows: true,
	}, importModels.BatchAudit{EntityType: "student", Filename: uploadResult.Filename, AccountID: accountID})
	if err != nil {
		renderBatchImportError(rs.runtime, w, r, result, err)
		return
	}

	// Log import summary
	slog.Default().Info("Student import completed",
		slog.Int("created", result.CreatedCount),
		slog.Int("updated", result.UpdatedCount),
		slog.Int("errors", result.ErrorCount),
		slog.String("filename", uploadResult.Filename))

	// Build success message
	message := fmt.Sprintf("Import abgeschlossen: %d erstellt, %d aktualisiert, %d Fehler",
		result.CreatedCount, result.UpdatedCount, result.ErrorCount)

	rs.runtime.Success(w, r, http.StatusOK, result, message)
}

// A failed batch may follow committed batches. Keep that progress and the
// stable row errors available without reporting the upload as successful.
func renderBatchImportError[T any](runtime Runtime, w http.ResponseWriter, r *http.Request, result *importModels.ImportResult[T], err error) {
	if result == nil {
		runtime.Failure(w, r, Failure{Status: http.StatusInternalServerError, Cause: err, Message: "Import fehlgeschlagen"})
		return
	}
	runtime.Failure(w, r, Failure{Status: http.StatusInternalServerError, Cause: err,
		Message: "Import fehlgeschlagen", Code: "import_batch_failed", Result: result})
}
