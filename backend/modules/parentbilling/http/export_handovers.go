package parentbillinghttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// ExportHandoverResource serves the handover of finance exports to the
// recipient of the Träger (#6824, ADR 0217) at /traeger/finance-handovers:
// the Sollstellung and the DATEV booking batch behind "Finanz-Export" and
// the PAIN.008 file behind "Abrechnung", like their downloads. A handover
// runs outside the request's Träger transaction: the transport commits each
// step in its own.
type ExportHandoverResource struct {
	handovers      capability.FinanceExportHandovers
	requireExport  common.Middleware
	requireBilling common.Middleware
}

// NewExportHandoverResource builds the resource.
func NewExportHandoverResource(handovers capability.FinanceExportHandovers, requireExport, requireBilling common.Middleware,
) *ExportHandoverResource {
	if handovers == nil || requireExport == nil || requireBilling == nil {
		panic("parent billing http: the handovers and the right checks are required")
	}
	return &ExportHandoverResource{handovers: handovers, requireExport: requireExport, requireBilling: requireBilling}
}

// Router returns the routes mounted at /traeger/finance-handovers.
func (rs *ExportHandoverResource) Router(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, _ common.Middleware) {
		export := r.With(rs.requireExport)
		export.Post("/runs/{kind}/{id}/sollstellung", rs.handOver(capability.ExportFormatSollstellung))
		export.Post("/runs/{kind}/{id}/datev", rs.handOver(capability.ExportFormatDatev))
		r.With(rs.requireBilling).Post("/runs/{kind}/{id}/pain008", rs.handOver(capability.ExportFormatPain008))
	})
	return r
}

// HandoverRequestBody is the body of POST …/runs/{kind}/{id}/{format}.
type HandoverRequestBody struct {
	Repeat bool `json:"repeat"`
}

// Bind accepts the body.
func (req *HandoverRequestBody) Bind(*http.Request) error { return nil }

// handOver hands the file of a run to the recipient. A transport failure is
// a result (200 with state failed), not an error.
func (rs *ExportHandoverResource) handOver(format string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		body := &HandoverRequestBody{}
		if r.ContentLength != 0 {
			if err := render.Bind(r, body); err != nil {
				common.RenderError(w, r, common.ErrorInvalidRequest(err))
				return
			}
		}
		request := capability.HandoverRequest{Format: format, RunKind: kind, RunID: id, Repeat: body.Repeat}
		if format == capability.ExportFormatDatev {
			if request.Datev, err = datevFileOptions(r); err != nil {
				common.RenderError(w, r, common.ErrorInvalidRequest(err))
				return
			}
		}
		result, err := rs.handovers.HandOver(r.Context(), reviewer, request)
		if err != nil {
			renderHandoverFileError(w, r, format, err)
			return
		}
		common.Respond(w, r, http.StatusOK, result, "Export handed over")
	}
}

// renderHandoverFileError answers a refused handover: the refusals of the
// handover itself, else those of the file's download.
func renderHandoverFileError(w http.ResponseWriter, r *http.Request, format string, err error) {
	switch {
	case errors.Is(err, capability.ErrExportRecipientNotReady):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingExportRecipientNotReady))
	case errors.Is(err, capability.ErrExportAlreadyHandedOver):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingExportAlreadyHandedOver))
	case errors.Is(err, capability.ErrExportHandoverFormat):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingExportRunNotFound))
	case format == capability.ExportFormatPain008:
		renderDebitError(w, r, err)
	default:
		renderExportError(w, r, err)
	}
}
