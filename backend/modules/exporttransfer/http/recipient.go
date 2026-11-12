// Package exporttransferhttp serves the recipient of a Träger of Export
// Transfer's Träger scope (#6824, ADR 0217) in the Träger portal.
package exporttransferhttp

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer"
)

// journalLimit is how many handovers the recipient page lists.
const journalLimit = 20

// RecipientResource serves /traeger/export-recipient: the recipient of the
// session's Träger without its password, its newest handovers, saving and
// removing it. Every route runs in the Träger transaction of the session's
// Träger behind the right the root binds.
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
		route.Get("/", rs.recipient)
		route.Put("/", rs.save)
		route.Delete("/", rs.remove)
	})
	return r
}

// RecipientRequest is the body of PUT. A missing password keeps the stored
// one, as long as address, port, user and fingerprint stay.
type RecipientRequest struct {
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

func (rs *RecipientResource) recipient(w http.ResponseWriter, r *http.Request) {
	organizationID, _, ok := session(w, r)
	if !ok {
		return
	}
	page, err := rs.transfers.Recipient(r.Context(), organizationID, journalLimit)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, page, "Export recipient retrieved successfully")
}

func (rs *RecipientResource) save(w http.ResponseWriter, r *http.Request) {
	organizationID, accountID, ok := session(w, r)
	if !ok {
		return
	}
	req := &RecipientRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingExportRecipientInvalid))
		return
	}
	recipient, err := rs.transfers.SaveRecipient(r.Context(), organizationID, accountID, exporttransfer.TraegerRecipientInput(*req))
	var invalid *exporttransfer.RecipientError
	switch {
	case errors.As(err, &invalid):
		fields := make([]common.FieldError, 0, len(invalid.Fields))
		for _, field := range invalid.Fields {
			fields = append(fields, common.FieldError{Field: field, Reason: "invalid"})
		}
		common.RenderError(w, r, &common.ErrResponse{Err: err, HTTPStatusCode: http.StatusBadRequest,
			Detail: "invalid recipient: " + strings.Join(invalid.Fields, ", "), Code: common.CodeParentBillingExportRecipientInvalid, Errors: fields})
	case err != nil:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	default:
		common.Respond(w, r, http.StatusOK, recipient, "Export recipient saved successfully")
	}
}

func (rs *RecipientResource) remove(w http.ResponseWriter, r *http.Request) {
	organizationID, _, ok := session(w, r)
	if !ok {
		return
	}
	recipient, err := rs.transfers.DeleteRecipient(r.Context(), organizationID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, recipient, "Export recipient removed successfully")
}
