package operator

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// The people of a Träger-Büro (ADR 0084, #4649): the operator lists them and
// assigns the first person with a Träger role.

type assignTraegerMemberRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (req *assignTraegerMemberRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(req.Email)
	req.Role = strings.TrimSpace(req.Role)
	return nil
}

// ListTraegerMembers answers GET /operator/organizations/{id}/traeger-members.
func (rs *ProvisioningResource) ListTraegerMembers(w http.ResponseWriter, r *http.Request) {
	common.IDFetch("id", "invalid organization ID", rs.service.ListTraegerMembers, ProvisioningErrorRenderer, "Träger members retrieved successfully")(w, r)
}

// AssignTraegerMember answers POST /operator/organizations/{id}/traeger-members.
func (rs *ProvisioningResource) AssignTraegerMember(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
	if !ok {
		return
	}
	req := &assignTraegerMemberRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}
	member, err := rs.service.AssignTraegerMember(r.Context(), organizationtenancy.AssignTraegerMember{
		OrganizationID: organizationID, Email: req.Email, Role: req.Role,
		OperatorID: int64(sessiontoken.ClaimsFromCtx(r.Context()).ID), ClientIP: common.ParseClientIP(r),
	})
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, member, "Träger member assigned successfully")
}

// resetTraegerMemberMFARequest is the body of the second-factor reset; the
// reason goes to the operator audit.
type resetTraegerMemberMFARequest struct {
	Reason string `json:"reason"`
}

func (req *resetTraegerMemberMFARequest) Bind(_ *http.Request) error {
	req.Reason = strings.TrimSpace(req.Reason)
	return nil
}

// ResetTraegerMemberMFA answers DELETE
// /operator/organizations/{id}/traeger-members/{accountId}/mfa: the person
// sets up the second factor again at the next sign-in (#4651).
func (rs *ProvisioningResource) ResetTraegerMemberMFA(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
	if !ok {
		return
	}
	accountID, ok := common.ParseInt64IDWithError(w, r, "accountId", "invalid account ID")
	if !ok {
		return
	}
	req := &resetTraegerMemberMFARequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}
	err := rs.service.ResetTraegerMemberMFA(r.Context(), organizationtenancy.ResetTraegerMemberMFA{
		OrganizationID: organizationID, AccountID: accountID, Reason: req.Reason,
		OperatorID: int64(sessiontoken.ClaimsFromCtx(r.Context()).ID), ClientIP: common.ParseClientIP(r),
	})
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Second factor reset successfully")
}

// traegerErrorRenderer maps the Träger membership refusals.
func traegerErrorRenderer(err error) (render.Renderer, bool) {
	switch {
	case errors.Is(err, organizationtenancy.ErrTraegerAccountNotFound):
		return common.OperatorRejection(http.StatusNotFound, common.CodeIdentityAccountNotFound, "No account with this e-mail address"), true
	case errors.Is(err, organizationtenancy.ErrTraegerMemberNotFound):
		return common.OperatorRejection(http.StatusNotFound, common.CodeIdentityAccountNotFound, "Account is no member of this Träger-Büro"), true
	case errors.Is(err, organizationtenancy.ErrOrganizationInactive):
		return common.OperatorRejection(http.StatusConflict, common.CodeGeneralBusinessRejection, "Organization is inactive"), true
	}
	return nil, false
}

// deviceOrTraegerErrorRenderer maps the device, Träger membership and Träger
// lifecycle refusals.
func deviceOrTraegerErrorRenderer(err error) (render.Renderer, bool) {
	if renderer, ok := deviceErrorRenderer(err); ok {
		return renderer, true
	}
	if renderer, ok := traegerLifecycleErrorRenderer(err); ok {
		return renderer, true
	}
	return traegerErrorRenderer(err)
}
