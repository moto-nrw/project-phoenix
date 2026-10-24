package operator

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// DeactivateOperator locks the account of a team member who left (#4047).
// The signed-in operator cannot deactivate their own account.
func (rs *InvitationsResource) DeactivateOperator(w http.ResponseWriter, r *http.Request) {
	rs.changeOperatorStatus(w, r, false)
}

// ReactivateOperator lets a deactivated operator sign in again.
func (rs *InvitationsResource) ReactivateOperator(w http.ResponseWriter, r *http.Request) {
	rs.changeOperatorStatus(w, r, true)
}

func (rs *InvitationsResource) changeOperatorStatus(w http.ResponseWriter, r *http.Request, active bool) {
	operatorID, err := common.ParseIDParam(r, "id")
	if err != nil || operatorID <= 0 {
		common.RenderError(w, r, common.OperatorInvalidRequest(errors.New("invalid operator ID")))
		return
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	operator, err := rs.invitationService.ChangeOperatorStatus(r.Context(), identityaccess.OperatorStatusChange{
		OperatorID: operatorID, ActorID: int64(claims.ID), IPAddress: clientIP(r), Active: active,
	})
	if err != nil {
		common.RenderError(w, r, operatorStatusErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, operatorListItem(operator), "")
}

func operatorListItem(operator Operator) *OperatorListItem {
	item := &OperatorListItem{
		ID: operator.ID, Email: operator.Email, DisplayName: operator.DisplayName, Active: operator.Active,
		LastLogin: operator.LastLogin, DeactivatedAt: operator.DeactivatedAt, CreatedAt: operator.CreatedAt,
	}
	if operator.DeactivatedAt != nil {
		due := identityaccess.OperatorDeletionDue(*operator.DeactivatedAt)
		item.DeletionDueAt = &due
	}
	return item
}

func operatorStatusErrorRenderer(err error) render.Renderer {
	switch {
	case errors.Is(err, ErrOperatorNotFound):
		return common.OperatorRejection(http.StatusNotFound, common.CodeIdentityAccountNotFound, "Operator not found")
	case errors.Is(err, ErrOperatorSelfDeactivation):
		return common.OperatorRejection(http.StatusConflict, common.CodeIdentityOperatorSelfDeactivation, "an operator cannot deactivate their own account")
	default:
		return common.OperatorInternal("Failed to change the operator status")
	}
}
