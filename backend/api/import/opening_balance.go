package importapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
)

// DownloadOpeningBalanceTemplate handles the template download (CSV or Excel).
func (rs *Resource) DownloadOpeningBalanceTemplate(w http.ResponseWriter, r *http.Request) {
	rs.downloadTemplate(w, r, importModels.OpeningBalanceTemplate, "eroeffnungssalden-import-vorlage")
}

// openingBalanceRequestContext bundles the per-request import parameters
// parsed from the upload form. The acting staff member is resolved separately
// (resolveOpeningBalanceDecider) because that lookup is tenant-scoped and the
// real-import handler owns its transaction.
type openingBalanceRequestContext struct {
	Rows          []importModels.OpeningBalanceImportRow
	Filename      string
	EffectiveDate string
	Note          string
	AccountID     int64
}

// resolveOpeningBalanceDecider maps the authenticated account to its staff
// row. The query is tenant-scoped, so it MUST run inside a tenant transaction
// — outside one the RLS role is missing and the lookup fails.
func (rs *Resource) resolveOpeningBalanceDecider(ctx context.Context, accountID int64) (int64, error) {
	staffID, err := rs.runtime.OpeningDecider(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("kein Mitarbeiterprofil für dieses Konto: %w", err)
	}
	return staffID, nil
}

// parseOpeningBalanceRequest validates the upload, parses the file, and reads
// the shared Stichtag + Begründung form fields.
func (rs *Resource) parseOpeningBalanceRequest(w http.ResponseWriter, r *http.Request) (*openingBalanceRequestContext, bool) {
	file, header, isExcel, ok := rs.openValidatedUploadFile(w, r)
	if !ok {
		return nil, false
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Default().Error("failed to close file", slog.String("error", err.Error()))
		}
	}()

	rows, err := rs.files.OpeningBalances(file, uploadFormat(isExcel))
	if err != nil {
		rs.runtime.Failure(w, r, Failure{Status: http.StatusBadRequest, Cause: fmt.Errorf("Datei-Fehler: %s", err.Error())})
		return nil, false
	}

	effectiveDate, err := rs.runtime.ValidateOpeningDate(r.FormValue("effective_date"))
	if err != nil {
		rs.runtime.Failure(w, r, Failure{Status: http.StatusBadRequest, Cause: err})
		return nil, false
	}
	note := strings.TrimSpace(r.FormValue("note"))
	if note == "" {
		rs.runtime.Failure(w, r, Failure{Status: http.StatusBadRequest, Cause: fmt.Errorf("begründung ist erforderlich")})
		return nil, false
	}

	accountID, err := rs.runtime.AccountID(r.Context())
	if err != nil {
		rs.runtime.Failure(w, r, Failure{Status: http.StatusUnauthorized, Cause: err})
		return nil, false
	}

	return &openingBalanceRequestContext{
		Rows:          rows,
		Filename:      header.Filename,
		EffectiveDate: effectiveDate,
		Note:          note,
		AccountID:     accountID,
	}, true
}

// PreviewOpeningBalanceImport handles the dry-run preview.
func (rs *Resource) PreviewOpeningBalanceImport(w http.ResponseWriter, r *http.Request) {
	if rs.runtime.OpeningImport == nil {
		rs.runtime.Failure(w, r, Failure{Status: http.StatusInternalServerError, Cause: fmt.Errorf("opening balance import is not configured")})
		return
	}
	reqCtx, ok := rs.parseOpeningBalanceRequest(w, r)
	if !ok {
		return
	}

	// The preview owns its tenant transaction (no route-level withTx): the
	// GDPR audit row must be committed before the success response is
	// written — a middleware transaction commits only after the handler
	// returns, when a commit failure can no longer be reported. The
	// tenant-scoped staff lookup therefore also runs inside the TX.
	tenantID := rs.runtime.TenantID(r.Context())
	var (
		result       *importModels.ImportResult[importModels.OpeningBalanceImportRow]
		deciderError error
	)
	if err := rs.runtime.WithinTenant(r.Context(), func(ctx context.Context) error {
		decidedBy, err := rs.resolveOpeningBalanceDecider(ctx, reqCtx.AccountID)
		if err != nil {
			deciderError = err
			return err
		}

		svc, err := rs.runtime.OpeningImport(reqCtx.EffectiveDate, reqCtx.Note, decidedBy)
		if err != nil {
			return err
		}
		var txErr error
		result, txErr = svc.Import(ctx, importModels.ImportRequest[importModels.OpeningBalanceImportRow]{
			Rows:            reqCtx.Rows,
			Mode:            importModels.ImportModeCreate,
			DryRun:          true,
			StopOnError:     false,
			UserID:          reqCtx.AccountID,
			SkipInvalidRows: false,
		})
		if txErr != nil {
			return txErr
		}
		// GDPR Compliance: Audit log for preview (Article 30).
		return svc.RecordAuditInTransaction(ctx, "opening_balance", reqCtx.Filename, result, reqCtx.AccountID, true, tenantID)
	}); err != nil {
		if deciderError != nil {
			rs.runtime.Failure(w, r, Failure{Status: http.StatusUnauthorized, Cause: deciderError})
			return
		}
		rs.runtime.Failure(w, r, Failure{Status: http.StatusInternalServerError, Cause: err, Message: "Import-Vorschau fehlgeschlagen"})
		return
	}

	rs.runtime.Success(w, r, http.StatusOK, result, "Import-Vorschau erfolgreich")
}

// ImportOpeningBalances resolves the actor, then lets the workflow own its
// bounded tenant transactions. A failure rolls back the current batch only.
func (rs *Resource) ImportOpeningBalances(w http.ResponseWriter, r *http.Request) {
	if rs.runtime.OpeningImport == nil {
		rs.runtime.Failure(w, r, Failure{Status: http.StatusInternalServerError, Cause: fmt.Errorf("opening balance import is not configured")})
		return
	}
	reqCtx, ok := rs.parseOpeningBalanceRequest(w, r)
	if !ok {
		return
	}

	var decidedBy int64
	var deciderError error
	if err := rs.runtime.WithinTenant(r.Context(), func(ctx context.Context) error {
		decidedBy, deciderError = rs.resolveOpeningBalanceDecider(ctx, reqCtx.AccountID)
		return deciderError
	}); err != nil {
		if deciderError != nil {
			rs.runtime.Failure(w, r, Failure{Status: http.StatusUnauthorized, Cause: deciderError})
		} else {
			rs.runtime.Failure(w, r, Failure{Status: http.StatusInternalServerError, Cause: err, Message: "Import fehlgeschlagen"})
		}
		return
	}
	svc, err := rs.runtime.OpeningImport(reqCtx.EffectiveDate, reqCtx.Note, decidedBy)
	if err != nil {
		rs.runtime.Failure(w, r, Failure{Status: http.StatusInternalServerError, Cause: err, Message: "Import fehlgeschlagen"})
		return
	}
	result, err := svc.ImportBatches(r.Context(), importModels.ImportRequest[importModels.OpeningBalanceImportRow]{
		Rows: reqCtx.Rows, Mode: importModels.ImportModeCreate, UserID: reqCtx.AccountID, SkipInvalidRows: true,
	}, importModels.BatchAudit{
		EntityType: "opening_balance", Filename: reqCtx.Filename, AccountID: reqCtx.AccountID,
		Options: fmt.Sprintf("%s\n%d\n%s", reqCtx.EffectiveDate, decidedBy, reqCtx.Note),
	})
	if err != nil {
		renderBatchImportError(rs.runtime, w, r, result, err)
		return
	}

	slog.Default().Info("Opening balance import completed",
		slog.Int("created", result.CreatedCount),
		slog.Int("errors", result.ErrorCount),
		slog.String("filename", reqCtx.Filename))

	message := fmt.Sprintf("Import abgeschlossen: %d übernommen, %d Fehler",
		result.CreatedCount, result.ErrorCount)

	rs.runtime.Success(w, r, http.StatusOK, result, message)
}
