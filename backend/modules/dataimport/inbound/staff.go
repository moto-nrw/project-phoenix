package importapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
)

// DownloadStaffTemplate handles the staff template download (CSV or Excel).
func (rs *Resource) DownloadStaffTemplate(w http.ResponseWriter, r *http.Request) {
	rs.downloadTemplate(w, r, importModels.StaffTemplate, "mitarbeiter-import-vorlage")
}

// PreviewStaffImport handles the staff import preview (dry-run).
func (rs *Resource) PreviewStaffImport(w http.ResponseWriter, r *http.Request) {
	uploadResult, ok := rs.validateAndParseStaffFile(w, r)
	if !ok {
		return // Error already handled by validateAndParseStaffFile
	}
	mode, ok := rs.importModeFromRequest(w, r)
	if !ok {
		return
	}

	accountID, err := rs.runtime.AccountID(r.Context())
	if err != nil {
		rs.runtime.Failure(w, r, Failure{Status: http.StatusUnauthorized, Cause: err})
		return
	}

	// The preview owns its tenant transaction (no route-level withTx): the
	// GDPR audit row must be committed before the success response is
	// written — a middleware transaction commits only after the handler
	// returns, when a commit failure can no longer be reported.
	ctx := importModels.ContextWithImporterPermissions(r.Context(), rs.runtime.Permissions(r.Context()))
	tenantID := rs.runtime.TenantID(ctx)
	var result *importModels.ImportResult[importModels.StaffImportRow]
	if err := rs.runtime.WithinTenant(ctx, func(ctx context.Context) error {
		request := importModels.ImportRequest[importModels.StaffImportRow]{
			Rows:            uploadResult.Rows,
			Mode:            mode,
			DryRun:          true,
			StopOnError:     false,
			UserID:          accountID,
			SkipInvalidRows: false,
		}

		var txErr error
		result, txErr = rs.staffImportService.Import(ctx, request)
		if txErr != nil {
			return txErr
		}
		// GDPR Compliance: Audit log for preview (Article 30).
		return rs.staffImportService.RecordAuditInTransaction(ctx, "staff", uploadResult.Filename, result, accountID, true, tenantID)
	}); err != nil {
		rs.renderStaffImportError(w, r, err, "Import-Vorschau fehlgeschlagen")
		return
	}

	rs.runtime.Success(w, r, http.StatusOK, result, "Import-Vorschau erfolgreich")
}

// msgStaffImportModeForbidden is shown when a create-only importer asks for
// update or upsert mode (#2906).
const msgStaffImportModeForbidden = "Bestehende Mitarbeiter ändern geht mit Ihren Berechtigungen nicht. Wählen Sie „Nur neue anlegen“ oder fragen Sie die Leitung."

// renderStaffImportError maps a refused import mode to 403 and everything
// else to a 500 with the given client message.
func (rs *Resource) renderStaffImportError(w http.ResponseWriter, r *http.Request, err error, clientMsg string) {
	if errors.Is(err, importModels.ErrImportModeForbidden) {
		rs.runtime.Failure(w, r, Failure{Status: http.StatusForbidden, Message: msgStaffImportModeForbidden})
		return
	}
	rs.runtime.Failure(w, r, Failure{Status: http.StatusInternalServerError, Cause: err, Message: clientMsg})
}

// ImportStaff handles the actual staff import (Stammdatensätze plus optional invitations).
func (rs *Resource) ImportStaff(w http.ResponseWriter, r *http.Request) {
	uploadResult, ok := rs.validateAndParseStaffFile(w, r)
	if !ok {
		return // Error already handled by validateAndParseStaffFile
	}
	mode, ok := rs.importModeFromRequest(w, r)
	if !ok {
		return
	}

	accountID, err := rs.runtime.AccountID(r.Context())
	if err != nil {
		rs.runtime.Failure(w, r, Failure{Status: http.StatusUnauthorized, Cause: err})
		return
	}

	ctx := importModels.ContextWithImporterPermissions(r.Context(), rs.runtime.Permissions(r.Context()))
	result, err := rs.staffImportService.ImportBatches(ctx, importModels.ImportRequest[importModels.StaffImportRow]{
		Rows: uploadResult.Rows, Mode: mode, UserID: accountID, SkipInvalidRows: true,
	}, importModels.BatchAudit{EntityType: "staff", Filename: uploadResult.Filename, AccountID: accountID})
	if err != nil {
		if result == nil {
			rs.renderStaffImportError(w, r, err, "Import fehlgeschlagen")
		} else {
			renderBatchImportError(rs.runtime, w, r, result, err)
		}
		return
	}

	slog.Default().Info("Staff import completed",
		slog.Int("created", result.CreatedCount),
		slog.Int("updated", result.UpdatedCount),
		slog.Int("errors", result.ErrorCount),
		slog.String("filename", uploadResult.Filename))

	message := fmt.Sprintf("Import abgeschlossen: %d erstellt, %d aktualisiert, %d Fehler",
		result.CreatedCount, result.UpdatedCount, result.ErrorCount)

	rs.runtime.Success(w, r, http.StatusOK, result, message)
}
