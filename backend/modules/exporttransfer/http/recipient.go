// Package exporttransferhttp serves the recipients of a Träger of Export
// Transfer's Träger scope (#6824, ADR 0218; #6911, ADR 0221) in the Träger
// portal.
package exporttransferhttp

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/exporttransfer"
)

// journalLimit is how many handovers a page lists.
const journalLimit = 20

// RecipientResource serves /traeger/export-recipient: the recipients of the
// session's Träger without their passwords, the recipient of each format,
// the newest handovers, adding, saving and removing a recipient and
// assigning the formats. Every route runs in the Träger transaction of the
// session's Träger behind the right the root binds.
type RecipientResource struct {
	transfers exporttransfer.TraegerTransfers
	require   common.Middleware
}

// NewRecipientResource builds the resource.
func NewRecipientResource(transfers exporttransfer.TraegerTransfers, require common.Middleware) *RecipientResource {
	if transfers == nil || require == nil {
		panic("export transfer http: the Träger transfers and the right check are required")
	}
	return &RecipientResource{transfers: transfers, require: require}
}

// Router returns the routes mounted at /traeger/export-recipient.
func (rs *RecipientResource) Router(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		route := r.With(rs.require, withTx)
		route.Get("/", rs.recipients)
		route.Post("/", rs.create)
		route.Put("/formats", rs.assign)
		route.Get("/{id}", rs.recipient)
		route.Put("/{id}", rs.save)
		route.Delete("/{id}", rs.remove)
	})
	return r
}

// RecipientRequest is the body of POST and PUT. A missing password keeps the
// stored one, as long as address, port, user and fingerprint stay.
type RecipientRequest struct {
	Name               string  `json:"name"`
	Enabled            bool    `json:"enabled"`
	Host               string  `json:"host"`
	Port               int     `json:"port"`
	Username           string  `json:"username"`
	Password           *string `json:"password"`
	RemoteDirectory    string  `json:"remote_directory"`
	HostKeyFingerprint string  `json:"host_key_fingerprint"`
}

// Bind accepts the body; the capability validates the values.
func (req *RecipientRequest) Bind(*http.Request) error { return nil }

// FormatsRequest is the body of PUT /formats: the recipient of each named
// format, null for the download.
type FormatsRequest struct {
	Assignments []exporttransfer.TraegerAssignment `json:"assignments"`
}

// Bind requires the list.
func (req *FormatsRequest) Bind(*http.Request) error {
	if req.Assignments == nil {
		return errors.New("assignments are required")
	}
	return nil
}

// session reads the Träger and the account of the session; never the
// request.
func session(w http.ResponseWriter, r *http.Request) (organizationID, accountID int64, ok bool) {
	principal, err := common.CurrentPrincipal(r.Context())
	if err != nil || principal.OrganizationID() <= 0 || principal.AccountID() <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return 0, 0, false
	}
	return principal.OrganizationID(), principal.AccountID(), true
}

// recipientID reads the recipient of the path; a malformed one is not found.
func recipientID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(exporttransfer.ErrRecipientNotFound, common.CodeParentBillingExportRecipientNotFound))
		return 0, false
	}
	return id, true
}

// renderError answers a refused recipient write or read.
func renderError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *exporttransfer.RecipientError
	switch {
	case errors.As(err, &invalid):
		fields := make([]common.FieldError, 0, len(invalid.Fields))
		for _, field := range invalid.Fields {
			fields = append(fields, common.FieldError{Field: field, Reason: "invalid"})
		}
		common.RenderError(w, r, &common.ErrResponse{Err: err, HTTPStatusCode: http.StatusBadRequest,
			Detail: "invalid recipient: " + strings.Join(invalid.Fields, ", "), Code: common.CodeParentBillingExportRecipientInvalid, Errors: fields})
	case errors.Is(err, exporttransfer.ErrRecipientNameTaken):
		common.RenderError(w, r, &common.ErrResponse{Err: err, HTTPStatusCode: http.StatusConflict, Detail: err.Error(),
			Code: common.CodeParentBillingExportRecipientNameTaken, Errors: []common.FieldError{{Field: exporttransfer.RecipientFieldName, Reason: "taken"}}})
	case errors.Is(err, exporttransfer.ErrRecipientNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingExportRecipientNotFound))
	case errors.Is(err, exporttransfer.ErrAssignmentInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingExportFormatsInvalid))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

func (rs *RecipientResource) recipients(w http.ResponseWriter, r *http.Request) {
	organizationID, _, ok := session(w, r)
	if !ok {
		return
	}
	page, err := rs.transfers.Recipients(r.Context(), organizationID, journalLimit)
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, page, "Export recipients retrieved successfully")
}

func (rs *RecipientResource) recipient(w http.ResponseWriter, r *http.Request) {
	organizationID, _, ok := session(w, r)
	if !ok {
		return
	}
	id, ok := recipientID(w, r)
	if !ok {
		return
	}
	page, err := rs.transfers.Recipient(r.Context(), organizationID, id, journalLimit)
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, page, "Export recipient retrieved successfully")
}

// bindRecipient reads the body of POST and PUT.
func bindRecipient(w http.ResponseWriter, r *http.Request) (exporttransfer.TraegerRecipientInput, bool) {
	req := &RecipientRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingExportRecipientInvalid))
		return exporttransfer.TraegerRecipientInput{}, false
	}
	return exporttransfer.TraegerRecipientInput(*req), true
}

func (rs *RecipientResource) create(w http.ResponseWriter, r *http.Request) {
	organizationID, accountID, ok := session(w, r)
	if !ok {
		return
	}
	input, ok := bindRecipient(w, r)
	if !ok {
		return
	}
	recipient, err := rs.transfers.CreateRecipient(r.Context(), organizationID, accountID, input)
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, recipient, "Export recipient added successfully")
}

func (rs *RecipientResource) save(w http.ResponseWriter, r *http.Request) {
	organizationID, accountID, ok := session(w, r)
	if !ok {
		return
	}
	id, ok := recipientID(w, r)
	if !ok {
		return
	}
	input, ok := bindRecipient(w, r)
	if !ok {
		return
	}
	recipient, err := rs.transfers.SaveRecipient(r.Context(), organizationID, accountID, id, input)
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, recipient, "Export recipient saved successfully")
}

func (rs *RecipientResource) remove(w http.ResponseWriter, r *http.Request) {
	organizationID, _, ok := session(w, r)
	if !ok {
		return
	}
	id, ok := recipientID(w, r)
	if !ok {
		return
	}
	page, err := rs.transfers.DeleteRecipient(r.Context(), organizationID, id)
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, page, "Export recipient removed successfully")
}

func (rs *RecipientResource) assign(w http.ResponseWriter, r *http.Request) {
	organizationID, accountID, ok := session(w, r)
	if !ok {
		return
	}
	req := &FormatsRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingExportFormatsInvalid))
		return
	}
	page, err := rs.transfers.AssignFormats(r.Context(), organizationID, accountID, req.Assignments)
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, page, "Export formats assigned successfully")
}
