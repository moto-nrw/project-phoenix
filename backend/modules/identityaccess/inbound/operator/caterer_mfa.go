package operator

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// CatererMFA is the operator's recovery of a caterer's second factor
// (#4497): the list of caterer accounts and the reset. A caterer supplies
// several schools, so no school admin can reset it.
type CatererMFA = identityaccess.CatererMFAAdministration

// CatererMFAResource serves /operator/caterers.
type CatererMFAResource struct {
	Caterers CatererMFA
}

// CatererSecurityResponse is one caterer account in the operator's list.
type CatererSecurityResponse struct {
	ID         string `json:"id"`
	AccountID  string `json:"account_id"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	MFAEnabled bool   `json:"mfa_enabled"`
}

func (rs *CatererMFAResource) available(w http.ResponseWriter, r *http.Request) bool {
	if rs == nil || rs.Caterers == nil {
		common.RenderError(w, r, common.OperatorInternal("caterer MFA endpoints are not configured"))
		return false
	}
	return true
}

// ListCaterers answers GET /operator/caterers.
func (rs *CatererMFAResource) ListCaterers(w http.ResponseWriter, r *http.Request) {
	if !rs.available(w, r) {
		return
	}
	caterers, err := rs.Caterers.ListCaterersForOperator(r.Context())
	if err != nil {
		common.RenderError(w, r, common.OperatorInternal("failed to list caterers"))
		return
	}
	rows := make([]CatererSecurityResponse, 0, len(caterers))
	for _, caterer := range caterers {
		rows = append(rows, CatererSecurityResponse{
			ID: strconv.FormatInt(caterer.ID, 10), AccountID: strconv.FormatInt(caterer.AccountID, 10),
			Name: caterer.Name, Email: caterer.Email, MFAEnabled: caterer.MFAEnabled,
		})
	}
	common.Respond(w, r, http.StatusOK, rows, "Caterers retrieved successfully")
}

// ResetCatererMFA answers DELETE
// /operator/caterers/{id}/accounts/{accountId}/mfa: the caterer
// signs in with its password alone until it switches the factor on again.
// The reason goes to the operator audit; the caterer gets a notice.
func (rs *CatererMFAResource) ResetCatererMFA(w http.ResponseWriter, r *http.Request) {
	if !rs.available(w, r) {
		return
	}
	catererID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid caterer ID")
	if !ok {
		return
	}
	accountID, ok := common.ParseInt64IDWithError(w, r, "accountId", "invalid account ID")
	if !ok {
		return
	}
	operatorID := operatorIDFromClaims(r)
	if operatorID == 0 {
		common.RenderError(w, r, common.OperatorUnauthorized())
		return
	}
	req := &MFAAdminResetRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}
	ctx := identityaccess.WithOperatorAuditClientIP(r.Context(), common.ParseClientIP(r))
	err := rs.Caterers.OperatorResetCatererMFA(ctx, operatorID, catererID, accountID, req.Reason)
	switch {
	case err == nil:
		common.RespondNoContent(w, r)
	case errors.Is(err, identityaccess.ErrCatererNotFound):
		common.RenderError(w, r, common.OperatorRejection(http.StatusNotFound, common.CodeIdentityAccountNotFound, "No account of this caterer"))
	case errors.Is(err, identityaccess.ErrCatererMFANotEnabled):
		common.RenderError(w, r, common.OperatorRejection(http.StatusConflict, common.CodeGeneralBusinessRejection, "The caterer's second factor is not switched on"))
	case errors.Is(err, identityaccess.ErrCatererMFAResetReasonRequired):
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
	case errors.Is(err, identityaccess.ErrOperatorAuditUnavailable):
		common.RenderError(w, r, common.OperatorRejection(http.StatusServiceUnavailable, common.CodeOperatorAuditUnavailable, "Operator audit unavailable"))
	default:
		common.RenderError(w, r, common.OperatorInternal("caterer MFA reset failed"))
	}
}
