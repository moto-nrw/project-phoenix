package parentbillinghttp

import (
	"errors"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// renderDebtorNumberFormat answers a debtor number in another format than
// the Träger's (#6675) as a field error on debtor_number with the format in
// its details, so the text can name it: "5 Ziffern, zum Beispiel 20815".
// It reports whether err was such a refusal.
func renderDebtorNumberFormat(w http.ResponseWriter, r *http.Request, err error) bool {
	fields, ok := errors.AsType[validation.Errors](err)
	if !ok {
		return false
	}
	refusal, ok := errors.AsType[*capability.DebtorNumberFormatError](fields["debtor_number"])
	if !ok {
		return false
	}
	format := refusal.Format
	code, details := common.CodeParentBillingDebtorNumberDigitRange,
		map[string]any{"min_digits": format.MinDigits, "max_digits": format.MaxDigits, "example": format.Example()}
	if format.MinDigits == format.MaxDigits {
		code, details = common.CodeParentBillingDebtorNumberDigits, map[string]any{"digits": format.MinDigits, "example": format.Example()}
	}
	response, _ := common.ErrorInvalidRequestWithCode(err, code).(*common.ErrResponse)
	response.Details = details
	common.RenderError(w, r, response)
	return true
}

// renderDebtorNumberRefusal answers a refused debtor number of a holiday
// registration: another format (#6675) or a number that differs from the
// family's, named with the family (#6692). It reports whether err was one.
func renderDebtorNumberRefusal(w http.ResponseWriter, r *http.Request, err error) bool {
	if renderDebtorNumberFormat(w, r, err) {
		return true
	}
	conflict, ok := errors.AsType[*capability.FamilyDebtorNumberConflict](err)
	if !ok {
		return false
	}
	common.RenderError(w, r, common.ErrorConflictWithDetails(err, common.CodeParentBillingFamilyDebtorNumberConflict, map[string]any{
		"family_debtor_number": conflict.FamilyDebtorNumber, "debtor_number": conflict.DebtorNumber,
		"family_email": conflict.FamilyEmail,
	}))
	return true
}
