package parent

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// Withdrawal and termination of a care contract (#4682, § 312k BGB): the
// page behind „Verträge hier kündigen“ reads what the contract allows, and
// „jetzt kündigen“ declares. The account comes from the claims; the service
// checks the relationship right and the address.

// contractNoticeKey reads the account and the contract from the request.
func contractNoticeKey(w http.ResponseWriter, r *http.Request) (int64, int64, int64, bool) {
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(sessiontoken.ErrTokenUnauthorized))
		return 0, 0, 0, false
	}
	organizationID, err := common.ParseIDParam(r, "organizationId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return 0, 0, 0, false
	}
	contractID, err := common.ParseIDParam(r, "contractId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return 0, 0, 0, false
	}
	return int64(claims.ID), organizationID, contractID, true
}

// contractNoticeOptions answers what the parents can declare today.
func (rs *Resource) contractNoticeOptions(w http.ResponseWriter, r *http.Request) {
	accountID, organizationID, contractID, ok := contractNoticeKey(w, r)
	if !ok {
		return
	}
	options, err := rs.ParentService.ContractNoticeOptions(r.Context(), accountID, organizationID, contractID)
	if err != nil {
		renderContractNoticeError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, options, "Contract notice options retrieved successfully")
}

// contractNoticeRequest is the body of the parents' declaration.
type contractNoticeRequest struct {
	parentService.ContractNoticeInput
}

func (req *contractNoticeRequest) Bind(*http.Request) error { return nil }

// declareContractNotice withdraws or terminates the contract.
func (rs *Resource) declareContractNotice(w http.ResponseWriter, r *http.Request) {
	accountID, organizationID, contractID, ok := contractNoticeKey(w, r)
	if !ok {
		return
	}
	req := &contractNoticeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	options, err := rs.ParentService.DeclareContractNotice(r.Context(), accountID, organizationID, contractID, req.ContractNoticeInput)
	if err != nil {
		renderContractNoticeError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, options, "Contract notice recorded successfully")
}

var contractNoticeConflicts = []struct {
	err  error
	code string
}{
	{parentService.ErrContractNotEndable, common.CodeCareContractContractNotEndable},
	{parentService.ErrContractWithdrawalClosed, common.CodeCareContractWithdrawalClosed},
	{parentService.ErrContractSpecialTerminationClosed, common.CodeCareContractSpecialTerminationClosed},
	{parentService.ErrContractNoticeTemplateMissing, common.CodeCareContractNoticeTemplateMissing},
}

// renderContractNoticeError answers a missing relationship right with 403, a
// contract outside the account's inbox with 404 and a refusal with 409.
func renderContractNoticeError(w http.ResponseWriter, r *http.Request, err error) {
	for _, rule := range contractNoticeConflicts {
		if errors.Is(err, rule.err) {
			common.RenderError(w, r, common.ErrorConflictWithCode(err, rule.code))
			return
		}
	}
	if errors.Is(err, parentService.ErrContractNoticeInvalid) {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCareContractFieldInvalid))
		return
	}
	renderBillingDocumentError(w, r, err)
}
