package parentbillinghttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-ogs/project-phoenix/api/common"
	capability "github.com/moto-ogs/project-phoenix/modules/parentbilling"
)

// DocumentKindResource serves the kind of document of each service of a
// Träger at /traeger/billing-document-kinds (#4688). Every route needs the
// Träger right "Abrechnung" and runs in the Träger transaction of the
// session's Träger, which comes from the session, never from the request.
type DocumentKindResource struct {
	kinds          capability.BillingDocumentKinds
	requireBilling common.Middleware
}

// NewDocumentKindResource builds the resource.
func NewDocumentKindResource(kinds capability.BillingDocumentKinds, requireBilling common.Middleware) *DocumentKindResource {
	if kinds == nil || requireBilling == nil {
		panic("parent billing http: the document kinds and the right check are required")
	}
	return &DocumentKindResource{kinds: kinds, requireBilling: requireBilling}
}

// Router returns the routes mounted at /traeger/billing-document-kinds.
func (rs *DocumentKindResource) Router(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		billing := r.With(rs.requireBilling, withTx)
		billing.Get("/", rs.list)
		billing.Put("/{service}", rs.set)
	})
	return r
}

// DocumentKindRequest is the body of PUT /traeger/billing-document-kinds/{service}.
type DocumentKindRequest struct {
	DocumentKind string `json:"document_kind"`
}

// Bind accepts the body; the Owner validates the kind.
func (req *DocumentKindRequest) Bind(*http.Request) error { return nil }

func (rs *DocumentKindResource) list(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	overview, err := rs.kinds.ListDocumentKinds(r.Context(), organizationID)
	if err != nil {
		renderDocumentKindError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, overview, "Document kinds retrieved successfully")
}

func (rs *DocumentKindResource) set(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	req := &DocumentKindRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	overview, err := rs.kinds.SetDocumentKind(r.Context(), organizationID, chi.URLParam(r, "service"), req.DocumentKind)
	if err != nil {
		renderDocumentKindError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, overview, "Document kind saved successfully")
}

func renderDocumentKindError(w http.ResponseWriter, r *http.Request, err error) {
	var fields validation.Errors
	if errors.As(err, &fields) {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingDocumentKindInvalid))
		return
	}
	common.RenderError(w, r, common.ErrorInternalServer(err))
}
