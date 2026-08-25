package operator

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	platformModel "github.com/moto-nrw/project-phoenix/models/platform"
	billingSvc "github.com/moto-nrw/project-phoenix/services/billing"
)

// InvoicesResource is the operator's maintenance surface for a school's
// payment schedule (#1459 demo). The school reads the same rows on /vertrag
// and cannot change them.
//
// Deliberately no payment provider: "bezahlt" here means a human at moto saw
// the money. That is the whole point of the demo — a truthful, manually kept
// overview beats an automated one that does not exist yet.
type InvoicesResource struct {
	service billingSvc.Service
}

// NewInvoicesResource builds the operator invoice resource.
func NewInvoicesResource(service billingSvc.Service) *InvoicesResource {
	return &InvoicesResource{service: service}
}

// invoiceRequest is the write payload. Dates travel as "YYYY-MM-DD"
// (timezone.Date marshals that way), amounts as integer cents.
type invoiceRequest struct {
	PeriodLabel   string `json:"period_label"`
	InvoiceNumber string `json:"invoice_number"`
	AmountCents   int64  `json:"amount_cents"`
	DueDate       string `json:"due_date"`
	Status        string `json:"status"`
	PaidOn        string `json:"paid_on"`
	Note          string `json:"note"`
}

// errInvalidDueDate / errInvalidPaidOn are German because the operator UI
// shows them verbatim.
var (
	errInvalidDueDate = errors.New("Fälligkeitsdatum muss im Format JJJJ-MM-TT angegeben werden.")
	errInvalidPaidOn  = errors.New("Zahlungsdatum muss im Format JJJJ-MM-TT angegeben werden.")
)

// toInput parses the wire payload into the service input.
func (req invoiceRequest) toInput() (billingSvc.InvoiceInput, error) {
	input := billingSvc.InvoiceInput{
		PeriodLabel:   req.PeriodLabel,
		InvoiceNumber: req.InvoiceNumber,
		AmountCents:   req.AmountCents,
		Status:        req.Status,
		Note:          req.Note,
	}

	dueDate, err := parseInvoiceDate(req.DueDate)
	if err != nil {
		return input, errInvalidDueDate
	}
	if dueDate == nil {
		return input, platformModel.ErrInvoiceDueDateRequired
	}
	input.DueDate = *dueDate

	paidOn, err := parseInvoiceDate(req.PaidOn)
	if err != nil {
		return input, errInvalidPaidOn
	}
	input.PaidOn = paidOn

	return input, nil
}

// ListSchoolInvoices returns one school's payment schedule.
func (rs *InvoicesResource) ListSchoolInvoices(w http.ResponseWriter, r *http.Request) {
	schoolID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid school ID")
	if !ok {
		return
	}

	invoices, err := rs.service.ListInvoices(r.Context(), schoolID)
	if err != nil {
		renderInvoiceError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, invoices, "")
}

// CreateSchoolInvoice appends a billing period to a school's schedule.
func (rs *InvoicesResource) CreateSchoolInvoice(w http.ResponseWriter, r *http.Request) {
	schoolID, input, ok := rs.decodeInvoiceRequest(w, r)
	if !ok {
		return
	}

	invoice, err := rs.service.CreateInvoice(r.Context(), schoolID, input)
	if err != nil {
		renderInvoiceError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusCreated, invoice, "Rechnung wurde angelegt.")
}

// UpdateSchoolInvoice replaces the editable fields of one invoice — including
// the payment status, which is how an operator records an incoming payment.
func (rs *InvoicesResource) UpdateSchoolInvoice(w http.ResponseWriter, r *http.Request) {
	schoolID, input, ok := rs.decodeInvoiceRequest(w, r)
	if !ok {
		return
	}
	invoiceID, ok := common.ParseInt64IDWithError(w, r, "invoiceId", "invalid invoice ID")
	if !ok {
		return
	}

	invoice, err := rs.service.UpdateInvoice(r.Context(), schoolID, invoiceID, input)
	if err != nil {
		renderInvoiceError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, invoice, "Rechnung wurde gespeichert.")
}

// DeleteSchoolInvoice removes an invoice that should never have existed.
func (rs *InvoicesResource) DeleteSchoolInvoice(w http.ResponseWriter, r *http.Request) {
	schoolID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid school ID")
	if !ok {
		return
	}
	invoiceID, ok := common.ParseInt64IDWithError(w, r, "invoiceId", "invalid invoice ID")
	if !ok {
		return
	}

	if err := rs.service.DeleteInvoice(r.Context(), schoolID, invoiceID); err != nil {
		renderInvoiceError(w, r, err)
		return
	}

	common.RespondNoContent(w, r)
}

// decodeInvoiceRequest parses the school id and the JSON body shared by
// create and update. It writes the error response itself and reports ok=false.
func (rs *InvoicesResource) decodeInvoiceRequest(w http.ResponseWriter, r *http.Request) (int64, billingSvc.InvoiceInput, bool) {
	var input billingSvc.InvoiceInput

	schoolID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid school ID")
	if !ok {
		return 0, input, false
	}

	var req invoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err)) //nolint:errcheck
		return 0, input, false
	}

	input, err := req.toInput()
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(err)) //nolint:errcheck
		return 0, input, false
	}

	return schoolID, input, true
}

// renderInvoiceError maps the billing service errors onto operator responses.
// Everything the service declares is a client mistake (404 / 400); anything
// else is ours (500).
func renderInvoiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, billingSvc.ErrInvoiceNotFound):
		render.Render(w, r, ErrNotFound(err.Error())) //nolint:errcheck
	case errors.Is(err, billingSvc.ErrInvoiceNumberTaken):
		render.Render(w, r, ErrConflict(err.Error())) //nolint:errcheck
	case isInvoiceValidationError(err):
		render.Render(w, r, ErrInvalidRequest(err)) //nolint:errcheck
	default:
		render.Render(w, r, ErrInternal(err.Error())) //nolint:errcheck
	}
}

// isInvoiceValidationError recognises the model's validation sentinels.
func isInvoiceValidationError(err error) bool {
	for _, sentinel := range []error{
		platformModel.ErrInvoicePeriodLabelRequired,
		platformModel.ErrInvoiceAmountNegative,
		platformModel.ErrInvoiceDueDateRequired,
		platformModel.ErrInvoiceUnknownStatus,
		platformModel.ErrInvoicePaidOnRequired,
		platformModel.ErrInvoicePaidOnNotAllowed,
	} {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}
