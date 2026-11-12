package enrollmenthttp

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-ogs/project-phoenix/api/common"
	capability "github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// The children of the family on the status page of a registration (#6335),
// gated only by the status token like the other parent-facing /requests
// routes: the token names one request and nothing else.

// FamilyChildrenRequest is the number the parents state.
type FamilyChildrenRequest struct {
	Children int `json:"children"`
}

func (req *FamilyChildrenRequest) Bind(*http.Request) error { return nil }

var familyChildrenErrorRules = []common.ErrorRule{
	{Target: capability.ErrRequestNotFound, Render: func(err error) render.Renderer {
		return common.ErrorNotFoundWithCode(err, common.CodeEnrollmentStatusLinkInvalid)
	}},
	{Target: capability.ErrFamilyChildrenClosed, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeEnrollmentFamilyChildrenClosed)
	}},
	{Target: capability.ErrFamilyChildrenInvalid, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentFamilyChildrenInvalid)
	}},
}

func (rs *Resource) getFamilyChildren(w http.ResponseWriter, r *http.Request) {
	token, ok := rs.familyChildrenToken(w, r)
	if !ok {
		return
	}
	children, err := rs.FamilyChildren.FamilyChildrenByStatusToken(r.Context(), token)
	rs.respondFamilyChildren(w, r, children, err, "Kinderzahl geladen")
}

func (rs *Resource) putFamilyChildren(w http.ResponseWriter, r *http.Request) {
	token, ok := rs.familyChildrenToken(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)
	req := &FamilyChildrenRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	children, err := rs.FamilyChildren.StateFamilyChildrenByStatusToken(r.Context(), token, req.Children)
	rs.respondFamilyChildren(w, r, children, err, "Kinderzahl gespeichert")
}

func (rs *Resource) familyChildrenToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	if rs.FamilyChildren == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment family children not configured")))
		return "", false
	}
	token := strings.TrimSpace(chi.URLParam(r, "statusToken"))
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(errors.New("status token is required"), common.CodeEnrollmentStatusLinkInvalid))
		return "", false
	}
	return token, true
}

func (rs *Resource) respondFamilyChildren(w http.ResponseWriter, r *http.Request, children capability.RegistrationFamilyChildren, err error, message string) {
	if err != nil {
		common.RenderError(w, r, common.RenderWithRules(err, familyChildrenErrorRules, func(err error) render.Renderer {
			return common.ErrorInternalServerWrap("enrollment family children failed", err)
		}))
		return
	}
	common.Respond(w, r, http.StatusOK, children, message)
}
