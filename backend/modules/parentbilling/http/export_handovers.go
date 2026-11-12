package parentbillinghttp

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// ExportHandoverResource serves the handover of finance exports to the
// recipient of the Träger (#6824, ADR 0217) at /traeger/finance-handovers:
// the recipient behind the right "Finanz-Export", the handover of the
// Sollstellung and the DATEV booking batch behind "Finanz-Export" and of
// the PAIN.008 file behind "Abrechnung", like their downloads. The
// recipient routes run in the Träger transaction of the session's Träger;
// a handover opens its own, because the transport commits each step.
type ExportHandoverResource struct {
	handovers      capability.FinanceExportHandovers
	recipients     capability.FinanceExportTransfer
	requireExport  common.Middleware
	requireBilling common.Middleware
}

// handoverJournalLimit is how many handovers the recipient page lists.
const handoverJournalLimit = 20

// NewExportHandoverResource builds the resource.
func NewExportHandoverResource(handovers capability.FinanceExportHandovers, recipients capability.FinanceExportTransfer,
	requireExport, requireBilling common.Middleware,
) *ExportHandoverResource {
	if handovers == nil || recipients == nil || requireExport == nil || requireBilling == nil {
		panic("parent billing http: the handovers, the recipient and the right checks are required")
	}
	return &ExportHandoverResource{handovers: handovers, recipients: recipients, requireExport: requireExport, requireBilling: requireBilling}
}

// Router returns the routes mounted at /traeger/finance-handovers.
func (rs *ExportHandoverResource) Router(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		recipient := r.With(rs.requireExport, withTx)
		recipient.Get("/recipient", rs.recipient)
		recipient.Put("/recipient", rs.saveRecipient)
		recipient.Delete("/recipient", rs.deleteRecipient)
		export := r.With(rs.requireExport)
		export.Post("/runs/{kind}/{id}/sollstellung", rs.handOver(capability.ExportFormatSollstellung))
		export.Post("/runs/{kind}/{id}/datev", rs.handOver(capability.ExportFormatDatev))
		r.With(rs.requireBilling).Post("/runs/{kind}/{id}/pain008", rs.handOver(capability.ExportFormatPain008))
	})
	return r
}

// ExportRecipientRequest is the body of PUT …/recipient. A missing
// password keeps the stored one.
type ExportRecipientRequest struct {
	Enabled            bool    `json:"enabled"`
	Host               string  `json:"host"`
	Port               int     `json:"port"`
	Username           string  `json:"username"`
	Password           *string `json:"password"`
	RemoteDirectory    string  `json:"remote_directory"`
	HostKeyFingerprint string  `json:"host_key_fingerprint"`
}

// Bind accepts the body; the transport validates the values.
func (req *ExportRecipientRequest) Bind(*http.Request) error { return nil }

// HandoverRequestBody is the body of POST …/runs/{kind}/{id}/{format}.
type HandoverRequestBody struct {
	Repeat bool `json:"repeat"`
}

// Bind accepts the body.
func (req *HandoverRequestBody) Bind(*http.Request) error { return nil }

func (rs *ExportHandoverResource) recipient(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	recipient, err := rs.recipients.ExportRecipient(r.Context(), reviewer.OrganizationID)
	if err != nil {
		renderHandoverError(w, r, err)
		return
	}
	handovers, err := rs.recipients.ExportHandovers(r.Context(), reviewer.OrganizationID, handoverJournalLimit)
	if err != nil {
		renderHandoverError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, capability.ExportRecipientPage{Recipient: recipient, Handovers: handovers},
		"Export recipient retrieved successfully")
}

func (rs *ExportHandoverResource) saveRecipient(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	req := &ExportRecipientRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingExportRecipientInvalid))
		return
	}
	recipient, err := rs.recipients.SaveExportRecipient(r.Context(), reviewer.OrganizationID, reviewer.AccountID,
		capability.ExportRecipientInput(*req))
	if err != nil {
		renderHandoverError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, recipient, "Export recipient saved successfully")
}

func (rs *ExportHandoverResource) deleteRecipient(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	if err := rs.recipients.DeleteExportRecipient(r.Context(), reviewer.OrganizationID); err != nil {
		renderHandoverError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Export recipient removed successfully")
}

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

// renderHandoverError answers a refused handover or recipient; the refusals
// of the producers keep the answers of their downloads.
func renderHandoverError(w http.ResponseWriter, r *http.Request, err error) {
	var recipient *capability.ExportRecipientError
	switch {
	case errors.As(err, &recipient):
		fields := make([]common.FieldError, 0, len(recipient.Fields))
		for _, field := range recipient.Fields {
			fields = append(fields, common.FieldError{Field: field, Reason: "invalid"})
		}
		response := &common.ErrResponse{Err: err, HTTPStatusCode: http.StatusBadRequest, Detail: "invalid recipient: " +
			strings.Join(recipient.Fields, ", "), Code: common.CodeParentBillingExportRecipientInvalid, Errors: fields}
		common.RenderError(w, r, response)
	case errors.Is(err, capability.ErrExportRecipientNotReady):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingExportRecipientNotReady))
	case errors.Is(err, capability.ErrExportAlreadyHandedOver):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingExportAlreadyHandedOver))
	case errors.Is(err, capability.ErrExportHandoverFormat):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(capability.ErrExportRunNotFound, common.CodeParentBillingExportRunNotFound))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// renderHandoverFileError answers a refused handover: the refusals of the
// handover itself, else those of the file's download.
func renderHandoverFileError(w http.ResponseWriter, r *http.Request, format string, err error) {
	switch {
	case errors.Is(err, capability.ErrExportRecipientNotReady), errors.Is(err, capability.ErrExportAlreadyHandedOver),
		errors.Is(err, capability.ErrExportHandoverFormat):
		renderHandoverError(w, r, err)
	case format == capability.ExportFormatPain008:
		renderDebitError(w, r, err)
	default:
		renderExportError(w, r, err)
	}
}
