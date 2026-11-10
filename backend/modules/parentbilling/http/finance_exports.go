package parentbillinghttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/sharedkernel/textnorm"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// FinanceExportResource serves the finance exports of the Elternabrechnung
// (#4693) at /traeger/finance-exports: the field mapping and the start of
// the file numbers, the released runs with their exports, the export
// protocol, the Sollstellung and the DATEV booking batch (#4694) of a run
// behind the Träger right "Finanz-Export", and the history of an invoice behind the right
// "Abrechnung". Every route runs in the Träger transaction of the session's
// Träger; a download answers its file only after its protocol entry
// committed with it.
type FinanceExportResource struct {
	exports        capability.FinanceExports
	requireExport  common.Middleware
	requireBilling common.Middleware
}

// NewFinanceExportResource builds the resource.
func NewFinanceExportResource(exports capability.FinanceExports, requireExport, requireBilling common.Middleware) *FinanceExportResource {
	if exports == nil || requireExport == nil || requireBilling == nil {
		panic("parent billing http: the finance exports and the right checks are required")
	}
	return &FinanceExportResource{exports: exports, requireExport: requireExport, requireBilling: requireBilling}
}

// Router returns the routes mounted at /traeger/finance-exports.
func (rs *FinanceExportResource) Router(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		export := r.With(rs.requireExport, withTx)
		export.Get("/settings", rs.settings)
		export.Put("/settings/sollstellung", rs.saveMapping)
		export.Delete("/settings/sollstellung", rs.resetMapping)
		export.Put("/settings/file-number-start", rs.setStart)
		export.Get("/runs", rs.runs)
		export.Get("/datev-runs", rs.datevRuns)
		export.Get("/files", rs.files)
		export.Get("/runs/{kind}/{id}/sollstellung", rs.sollstellung)
		export.Get("/runs/{kind}/{id}/datev", rs.datev)
		r.With(rs.requireBilling, withTx).Get("/invoice-history/{kind}/{id}", rs.history)
	})
	return r
}

// exportRunKinds maps the path segment of a run to its kind.
var exportRunKinds = map[string]string{
	"care-runs":     capability.DebitRunCare,
	"holiday-runs":  capability.DebitRunHoliday,
	"notice-months": capability.DebitRunNotice,
}

// historyKinds maps the path segment of an invoice to its kind.
var historyKinds = map[string]string{
	"care-invoices":     capability.HistoryCareInvoice,
	"holiday-documents": capability.HistoryHolidayDocument,
}

// SollstellungMappingRequest is the body of PUT …/settings/sollstellung.
type SollstellungMappingRequest struct {
	capability.SollstellungMapping
}

// Bind accepts the body; the owner validates the mapping.
func (req *SollstellungMappingRequest) Bind(*http.Request) error { return nil }

// FileNumberStartRequest is the body of PUT …/settings/file-number-start.
type FileNumberStartRequest struct {
	Start int64 `json:"start"`
}

// Bind accepts the body; the owner validates the start.
func (req *FileNumberStartRequest) Bind(*http.Request) error { return nil }

// exportPage reads ?offset= of a list; a missing one is the first page.
func exportPage(w http.ResponseWriter, r *http.Request) (capability.ExportPage, bool) {
	raw := r.URL.Query().Get("offset")
	if raw == "" {
		return capability.ExportPage{}, true
	}
	offset, err := strconv.Atoi(raw)
	if err != nil || offset < 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("offset must be a whole number of at least 0")))
		return capability.ExportPage{}, false
	}
	return capability.ExportPage{Offset: offset}, true
}

func (rs *FinanceExportResource) settings(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	settings, err := rs.exports.ExportSettings(r.Context(), reviewer)
	if err != nil {
		renderExportError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, settings, "Export settings retrieved successfully")
}

func (rs *FinanceExportResource) saveMapping(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	req := &SollstellungMappingRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingExportMappingInvalid))
		return
	}
	settings, err := rs.exports.SaveSollstellungMapping(r.Context(), reviewer, req.SollstellungMapping)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderExportError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, settings, "Sollstellung mapping saved successfully")
}

func (rs *FinanceExportResource) resetMapping(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	settings, err := rs.exports.ResetSollstellungMapping(r.Context(), reviewer)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderExportError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, settings, "Sollstellung mapping reset successfully")
}

func (rs *FinanceExportResource) setStart(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	req := &FileNumberStartRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingExportMappingInvalid))
		return
	}
	settings, err := rs.exports.SetExportFileNumberStart(r.Context(), reviewer, req.Start)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderExportError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, settings, "Export file number start set successfully")
}

func (rs *FinanceExportResource) runs(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	page, ok := exportPage(w, r)
	if !ok {
		return
	}
	runs, err := rs.exports.ExportRuns(r.Context(), reviewer, page)
	if err != nil {
		renderExportError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, runs, "Export runs retrieved successfully")
}

// files lists a page of the protocol (?offset=); ?run_kind=care-runs&run_id=…
// narrows it to a run.
func (rs *FinanceExportResource) files(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	kind, runID := "", int64(0)
	if segment := r.URL.Query().Get("run_kind"); segment != "" {
		var known bool
		kind, known = exportRunKinds[segment]
		id, err := strconv.ParseInt(r.URL.Query().Get("run_id"), 10, 64)
		if !known || err != nil || id <= 0 {
			common.RenderError(w, r, common.ErrorNotFoundWithCode(capability.ErrExportRunNotFound, common.CodeParentBillingExportRunNotFound))
			return
		}
		runID = id
	}
	page, ok := exportPage(w, r)
	if !ok {
		return
	}
	files, err := rs.exports.ExportProtocol(r.Context(), reviewer, kind, runID, page)
	if err != nil {
		renderExportError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, files, "Export protocol retrieved successfully")
}

// sollstellung answers the Sollstellung of a run.
func (rs *FinanceExportResource) sollstellung(w http.ResponseWriter, r *http.Request) {
	rs.runFile(w, r, rs.exports.SollstellungFile)
}

// datev answers the DATEV booking batch of a run (#4694): the bookings no
// DATEV file held yet, or with mode=all every booking again (#6433);
// fiscal_year (YYYY-MM-DD, the first day) picks the fiscal year.
func (rs *FinanceExportResource) datev(w http.ResponseWriter, r *http.Request) {
	options, err := datevFileOptions(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	rs.runFile(w, r, func(ctx context.Context, reviewer capability.HolidayReviewer, kind string, id int64) (capability.ExportedFile, error) {
		return rs.exports.DatevFile(ctx, reviewer, kind, id, options)
	})
}

// datevRuns answers the new DATEV bookings and fiscal years of the runs a
// page of the export page shows (#6433): care and holiday name the IDs of
// monthly and holiday runs, separated by commas.
func (rs *FinanceExportResource) datevRuns(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	runs, err := datevRunRefs(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	overviews, err := rs.exports.DatevRunOverviews(r.Context(), reviewer, runs)
	if err != nil {
		renderExportError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, overviews, "DATEV bookings of the runs retrieved successfully")
}

// datevRunRefs reads the runs of the query: care=5,6&holiday=7.
func datevRunRefs(r *http.Request) ([]capability.ExportRunRef, error) {
	var runs []capability.ExportRunRef
	for param, kind := range map[string]string{"care": capability.DebitRunCare, "holiday": capability.DebitRunHoliday} {
		value := r.URL.Query().Get(param)
		if value == "" {
			continue
		}
		for _, part := range strings.Split(value, ",") {
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil || id <= 0 {
				return nil, errors.New(param + " must name run IDs separated by commas")
			}
			runs = append(runs, capability.ExportRunRef{RunKind: kind, RunID: id})
		}
	}
	if len(runs) > capability.ExportPageSize {
		return nil, errors.New("at most one page of runs")
	}
	return runs, nil
}

// datevFileOptions reads mode and fiscal_year of a DATEV download.
func datevFileOptions(r *http.Request) (capability.DatevFileOptions, error) {
	query := r.URL.Query()
	options := capability.DatevFileOptions{}
	switch query.Get("mode") {
	case "", "new":
	case "all":
		options.All = true
	default:
		return options, errors.New("mode must be new or all")
	}
	if value := query.Get("fiscal_year"); value != "" {
		start, err := calendar.ParseDate(value)
		if err != nil {
			return options, errors.New("fiscal_year must be a date YYYY-MM-DD")
		}
		options.FiscalYearStart = start
	}
	return options, nil
}

// runFile answers a file of a run; it leaves the server only in this answer
// and is never stored or cached. Its number and protocol entry commit with
// the answer, or neither does.
func (rs *FinanceExportResource) runFile(w http.ResponseWriter, r *http.Request,
	build func(context.Context, capability.HolidayReviewer, string, int64) (capability.ExportedFile, error),
) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	kind, known := exportRunKinds[chi.URLParam(r, "kind")]
	id, err := common.ParseIDParam(r, "id")
	if !known || err != nil {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(capability.ErrExportRunNotFound, common.CodeParentBillingExportRunNotFound))
		return
	}
	file, err := build(r.Context(), reviewer, kind, id)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderExportError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", textnorm.ContentDisposition("attachment", file.Entry.FileName))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

func (rs *FinanceExportResource) history(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	kind, known := historyKinds[chi.URLParam(r, "kind")]
	id, err := common.ParseIDParam(r, "id")
	if !known || err != nil {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(capability.ErrExportInvoiceNotFound, common.CodeParentBillingExportInvoiceNotFound))
		return
	}
	history, err := rs.exports.InvoiceHistory(r.Context(), reviewer, kind, id)
	if err != nil {
		renderExportError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, history, "Invoice history retrieved successfully")
}

// renderExportError answers a refused export. A refused DATEV batch names
// the missing settings or the refused documents in details.
func renderExportError(w http.ResponseWriter, r *http.Request, err error) {
	var fields validation.Errors
	var notReady *capability.DatevNotReadyError
	var bookings *capability.DatevBookingsError
	switch {
	case errors.As(err, &notReady):
		common.RenderError(w, r, common.ErrorConflictWithDetails(err, common.CodeParentBillingDatevNotReady, map[string]any{
			"missing_settings": notReady.Readiness.MissingSettings, "invalid_settings": notReady.Readiness.InvalidSettings}))
	case errors.As(err, &bookings):
		common.RenderError(w, r, common.ErrorConflictWithDetails(err, common.CodeParentBillingDatevBookingsInvalid, map[string]any{
			"documents": bookings.Problems}))
	case errors.As(err, &fields):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingExportMappingInvalid))
	case errors.Is(err, capability.ErrExportRunNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingExportRunNotFound))
	case errors.Is(err, capability.ErrDatevNoNewBookings):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingDatevNoNewBookings))
	case errors.Is(err, capability.ErrDatevBookingsTaken):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingDatevBookingsTaken))
	case errors.Is(err, capability.ErrExportRunEmpty):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingExportRunEmpty))
	case errors.Is(err, capability.ErrExportFileNumberStartLocked):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingExportFileNumberStartLocked))
	case errors.Is(err, capability.ErrExportInvoiceNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingExportInvoiceNotFound))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
