package parentbillinghttp

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/sharedkernel/textnorm"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// mountReview adds the review of a calculated monthly run (#5916): the
// invoice date, the exclusion of single invoices and the review list. Every
// route needs the right "Abrechnung" and runs in the Träger transaction.
func (rs *CareBillingResource) mountReview(billing chi.Router) {
	billing.Get("/schools", rs.runSchools)
	billing.Put("/{id}/invoice-date", rs.setInvoiceDate)
	billing.Post("/{id}/exclusions", rs.exclude)
	billing.Delete("/{id}/exclusions/{contractID}", rs.include)
	billing.Get("/{id}/review-export", rs.reviewExport)
	billing.Get("/{id}/cost-shares/{costBearerID}/export", rs.costShareExport)
}

// CareReleaseRequest is the body of POST …/release: what the reviewer saw,
// the count, the sum and the fingerprint of the preview.
type CareReleaseRequest struct {
	DocumentCount int    `json:"document_count"`
	TotalCents    int64  `json:"total_cents"`
	Fingerprint   string `json:"fingerprint"`
}

// Bind accepts the body; the Owner compares it with the preview.
func (req *CareReleaseRequest) Bind(*http.Request) error { return nil }

// CareInvoiceDateRequest is the body of PUT …/invoice-date; null takes the
// day of the release.
type CareInvoiceDateRequest struct {
	InvoiceDate *calendar.Date `json:"invoice_date"`
}

// Bind accepts the body; the Owner checks the date.
func (req *CareInvoiceDateRequest) Bind(*http.Request) error { return nil }

// CareExclusionRequest is the body of POST …/exclusions.
type CareExclusionRequest struct {
	ContractID int64  `json:"contract_id"`
	Reason     string `json:"reason"`
}

// Bind accepts the body; the Owner checks the reason and the invoice.
func (req *CareExclusionRequest) Bind(*http.Request) error { return nil }

// runSchools answers GET …/schools?month=YYYY-MM-01 with the schools a new
// run of the month may bill.
func (rs *CareBillingResource) runSchools(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	month, err := calendar.ParseDate(r.URL.Query().Get("month"))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingCareRunInvalid))
		return
	}
	schools, err := rs.billing.CareRunSchools(r.Context(), organizationID, month)
	if err != nil {
		renderCareError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, schools, "Schools of the monthly run retrieved successfully")
}

func (rs *CareBillingResource) setInvoiceDate(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	req := &CareInvoiceDateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingCareRunInvalid))
		return
	}
	detail, err := rs.billing.SetCareRunInvoiceDate(r.Context(), reviewer, runID, req.InvoiceDate)
	rs.respondReview(w, r, detail, err, "Invoice date saved successfully")
}

func (rs *CareBillingResource) exclude(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	req := &CareExclusionRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingCareRunInvalid))
		return
	}
	detail, err := rs.billing.ExcludeCareInvoice(r.Context(), reviewer, runID, req.ContractID, req.Reason)
	rs.respondReview(w, r, detail, err, "Invoice excluded successfully")
}

func (rs *CareBillingResource) include(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	contractID, err := common.ParseIDParam(r, "contractID")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	detail, err := rs.billing.IncludeCareInvoice(r.Context(), reviewer, runID, contractID)
	rs.respondReview(w, r, detail, err, "Invoice included successfully")
}

// respondReview answers a change of the review with the run, or rolls the
// Träger transaction back and answers the refusal.
func (rs *CareBillingResource) respondReview(w http.ResponseWriter, r *http.Request, detail capability.CareRunDetail, err error, message string) {
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, detail, message)
}

// reviewExport answers GET …/review-export?format=xlsx|csv with the review
// list of a calculated run. A failed record of the download rolls back and
// sends no file.
func (rs *CareBillingResource) reviewExport(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	file, err := rs.billing.ExportCareRunReview(r.Context(), reviewer, runID, format)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareError(w, r, err)
		return
	}
	writeCareExportFile(w, file)
}

// costShareExport answers GET
// …/{id}/cost-shares/{costBearerID}/export?format=pdf|xlsx|csv with the list
// for one cost bearer of the run (#4687).
func (rs *CareBillingResource) costShareExport(w http.ResponseWriter, r *http.Request) {
	reviewer, runID, ok := runKey(w, r)
	if !ok {
		return
	}
	costBearerID, err := common.ParseIDParam(r, "costBearerID")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	file, err := rs.billing.ExportCareCostShares(r.Context(), reviewer, runID, costBearerID, format)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderCareError(w, r, err)
		return
	}
	writeCareExportFile(w, file)
}

// writeCareExportFile sends a rendered list as a download nobody caches.
func writeCareExportFile(w http.ResponseWriter, file capability.ExportFile) {
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", textnorm.ContentDisposition("attachment", file.Filename))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}
