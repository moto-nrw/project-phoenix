package importapi

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/render"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
)

// Class-list entry import (#2382): the bulk form of the minimal
// Klassenlisteneintrag — Vorname, Nachname, Klasse, nothing else.

// ClassListFileUploadResult carries the parsed rows plus the original
// filename for the audit trail.
type ClassListFileUploadResult struct {
	Rows     []importModels.ClassListEntryImportRow
	Filename string
}

// validateAndParseClassListFile handles upload validation and parsing for
// class-list entry imports. Supports both CSV and Excel (.xlsx) files.
func (rs *Resource) validateAndParseClassListFile(w http.ResponseWriter, r *http.Request) (*ClassListFileUploadResult, bool) {
	file, header, isExcel, ok := rs.openValidatedUploadFile(w, r)
	if !ok {
		return nil, false
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Default().Error("failed to close file", slog.String("error", err.Error()))
		}
	}()

	rows, err := rs.files.ClassList(file, uploadFormat(isExcel))
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		rs.runtime.Failure(w, r, Failure{Status: http.StatusBadRequest, Cause: fmt.Errorf("Datei-Fehler: %s", err.Error())})
		return nil, false
	}

	return &ClassListFileUploadResult{
		Rows:     rows,
		Filename: header.Filename,
	}, true
}

// DownloadClassListTemplate handles the template download (CSV or Excel).
func (rs *Resource) DownloadClassListTemplate(w http.ResponseWriter, r *http.Request) {
	rs.downloadTemplate(w, r, importModels.ClassListTemplate, "klassenliste-import-vorlage")
}

// PreviewClassListImport handles the class-list import preview (dry-run).
func (rs *Resource) PreviewClassListImport(w http.ResponseWriter, r *http.Request) {
	rs.runClassListImport(w, r, true)
}

// ImportClassList handles the actual class-list entry import.
func (rs *Resource) ImportClassList(w http.ResponseWriter, r *http.Request) {
	rs.runClassListImport(w, r, false)
}

// runClassListImport delegates transaction ownership to the workflow. Audit
// records commit with the preview or each bounded batch before the response.
func (rs *Resource) runClassListImport(w http.ResponseWriter, r *http.Request, dryRun bool) {
	uploadResult, ok := rs.validateAndParseClassListFile(w, r)
	if !ok {
		return
	}

	accountID, err := rs.runtime.AccountID(r.Context())
	if err != nil {
		rs.runtime.Failure(w, r, Failure{Status: http.StatusUnauthorized, Cause: err})
		return
	}

	result, err := rs.classListImportService.ImportBatches(r.Context(), importModels.ImportRequest[importModels.ClassListEntryImportRow]{
		Rows: uploadResult.Rows, Mode: importModels.ImportModeCreate, DryRun: dryRun, UserID: accountID, SkipInvalidRows: !dryRun,
	}, importModels.BatchAudit{EntityType: "class_list_entries", Filename: uploadResult.Filename, AccountID: accountID})
	if err != nil {
		renderBatchImportError(rs.runtime, w, r, result, err)
		return
	}

	if dryRun {
		rs.runtime.Success(w, r, http.StatusOK, result, "Import-Vorschau erfolgreich")
		return
	}

	slog.Default().Info("Class list entry import completed",
		slog.Int("created", result.CreatedCount),
		slog.Int("errors", result.ErrorCount),
		slog.String("filename", uploadResult.Filename))

	message := fmt.Sprintf("Import abgeschlossen: %d erstellt, %d Fehler",
		result.CreatedCount, result.ErrorCount)

	rs.runtime.Success(w, r, http.StatusOK, result, message)
}
