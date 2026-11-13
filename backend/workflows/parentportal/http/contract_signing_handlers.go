package parent

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	parentService "github.com/moto-ogs/project-phoenix/workflows/parentportal"
)

// Signing a care contract in the parents portal (#4679). The account comes
// from the claims; the service checks the relationship and the address.

// contractSigningKey reads the account, the Träger and the contract.
func contractSigningKey(w http.ResponseWriter, r *http.Request) (int64, int64, int64, bool) {
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

// contractSigning answers the contract to sign.
func (rs *Resource) contractSigning(w http.ResponseWriter, r *http.Request) {
	accountID, organizationID, contractID, ok := contractSigningKey(w, r)
	if !ok {
		return
	}
	view, err := rs.ParentService.ContractSigningView(r.Context(), accountID, organizationID, contractID)
	if err != nil {
		renderContractSigningError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	common.Respond(w, r, http.StatusOK, view, "Contract retrieved")
}

// ContractSignRequest is the signature of the parents.
type ContractSignRequest struct {
	parentService.ContractSignatureInput
}

func (req *ContractSignRequest) Bind(*http.Request) error { return nil }

// signContract signs the contract.
func (rs *Resource) signContract(w http.ResponseWriter, r *http.Request) {
	accountID, organizationID, contractID, ok := contractSigningKey(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 400<<10)
	req := &ContractSignRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	view, err := rs.ParentService.SignContract(r.Context(), accountID, organizationID, contractID, req.ContractSignatureInput)
	if err != nil {
		renderContractSigningError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, view, "Contract signed")
}

func renderContractSigningError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, parentService.ErrContractSignatureInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCareContractSignatureInvalid))
	case errors.Is(err, parentService.ErrContractWithdrawalNoticeRequired):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCareContractWithdrawalNoticeRequired))
	case errors.Is(err, parentService.ErrContractNotSignable):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCareContractContractNotSignable))
	default:
		renderBillingDocumentError(w, r, err)
	}
}
