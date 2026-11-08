package parent

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// Children of the family on a care contract of the inbox (#5918): the
// parents read the number of their family and state it. The account comes
// from the claims; the service checks the address and the relationship
// right.

// familyChildren answers the number of the family on the contract.
func (rs *Resource) familyChildren(w http.ResponseWriter, r *http.Request) {
	accountID, organizationID, contractID, ok := contractNoticeKey(w, r)
	if !ok {
		return
	}
	children, err := rs.ParentService.FamilyChildren(r.Context(), accountID, organizationID, contractID)
	if err != nil {
		renderFamilyChildrenError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, children, "Family children retrieved successfully")
}

// familyChildrenRequest is the number the parents state.
type familyChildrenRequest struct {
	Children int `json:"children"`
}

func (req *familyChildrenRequest) Bind(*http.Request) error { return nil }

// stateFamilyChildren stores the number the parents state.
func (rs *Resource) stateFamilyChildren(w http.ResponseWriter, r *http.Request) {
	accountID, organizationID, contractID, ok := contractNoticeKey(w, r)
	if !ok {
		return
	}
	req := &familyChildrenRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingFamilyChildrenInvalid))
		return
	}
	children, err := rs.ParentService.StateFamilyChildren(r.Context(), accountID, organizationID, contractID, req.Children)
	if err != nil {
		renderFamilyChildrenError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, children, "Family children stated successfully")
}

// renderFamilyChildrenError answers an invalid number with 400, a missing
// relationship right with 403 and a contract outside the inbox with 404.
func renderFamilyChildrenError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, parentService.ErrFamilyChildrenInvalid) {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingFamilyChildrenInvalid))
		return
	}
	renderBillingDocumentError(w, r, err)
}
