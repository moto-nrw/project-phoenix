package traeger

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/templates"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// Document templates of the Träger (#4677, ADR 0150) at
// /traeger/master-data/document-templates. Reading and the preview with
// sample data need "Übersicht lesen", saving a version "Einstellungen".

// TemplateRequest is the body of POST …/document-templates/{kind}/versions
// and …/preview.
type TemplateRequest struct {
	Subject    string `json:"subject"`
	Salutation string `json:"salutation"`
	Body       string `json:"body"`
	Closing    string `json:"closing"`
}

// Bind accepts the body; the Owner validates it.
func (req *TemplateRequest) Bind(*http.Request) error { return nil }

func (req *TemplateRequest) template() templates.Template {
	return templates.Template(*req)
}

func templateKind(r *http.Request) templates.Kind {
	return templates.Kind(chi.URLParam(r, "kind"))
}

func (rs *MasterDataResource) listTemplates(w http.ResponseWriter, r *http.Request) {
	organizationID, _, ok := session(w, r)
	if !ok {
		return
	}
	list, err := rs.templates.Overview(r.Context(), organizationID)
	if err != nil {
		renderTemplateError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, list, "Document templates retrieved successfully")
}

func (rs *MasterDataResource) templateDetail(w http.ResponseWriter, r *http.Request) {
	organizationID, _, ok := session(w, r)
	if !ok {
		return
	}
	detail, err := rs.templates.Detail(r.Context(), organizationID, templateKind(r))
	if err != nil {
		renderTemplateError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, detail, "Document template retrieved successfully")
}

func (rs *MasterDataResource) saveTemplate(w http.ResponseWriter, r *http.Request) {
	organizationID, accountID, ok := session(w, r)
	if !ok {
		return
	}
	req := &TemplateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	saved, err := rs.templates.AddVersion(r.Context(), organizationID, accountID, templateKind(r), req.template())
	if err != nil {
		renderTemplateError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, saved, "Document template saved successfully")
}

func (rs *MasterDataResource) previewTemplate(w http.ResponseWriter, r *http.Request) {
	organizationID, _, ok := session(w, r)
	if !ok {
		return
	}
	req := &TemplateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	filled, err := rs.templates.Preview(r.Context(), organizationID, templateKind(r), req.template())
	if err != nil {
		renderTemplateError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, filled, "Document template preview rendered successfully")
}

// renderTemplateError answers a refused template operation. Every refusal
// is decided before the first write.
func renderTemplateError(w http.ResponseWriter, r *http.Request, err error) {
	var fields validation.Errors
	var unknown *templates.UnknownPlaceholdersError
	switch {
	case errors.As(err, &unknown):
		common.RenderError(w, r, common.ErrorInvalidRequestWithDetails(err, common.CodeTraegerDocumentTemplateUnknownPlaceholder,
			map[string]any{"placeholders": strings.Join(unknown.Names, ", ")}))
	case errors.As(err, &fields):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeTraegerDocumentTemplateInvalid))
	case errors.Is(err, organizationtenancy.ErrTraegerDocumentTemplateKindNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeTraegerDocumentTemplateKindNotFound))
	case errors.Is(err, organizationtenancy.ErrTraegerDocumentTemplateUnchanged):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeTraegerDocumentTemplateUnchanged))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
