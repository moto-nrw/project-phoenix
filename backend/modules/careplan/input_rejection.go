package careplan

// Wire codes of rejected care offering values (#2515). The package may not
// import api/common, so each registered code is declared once here.
const (
	CodeCareOfferingNameRequired      = "enrollment.care_offering_name_required"
	CodeCareOfferingCapacityInvalid   = "enrollment.care_offering_capacity_invalid"
	CodeCareOfferingPriceInvalid      = "enrollment.care_offering_price_invalid"
	CodeCareOfferingSelectionInvalid  = "enrollment.care_offering_selection_invalid"
	CodeCareOfferingPickupTimeInvalid = "enrollment.care_offering_pickup_time_invalid"
	CodeCareOfferingAutoAddInvalid    = "enrollment.care_offering_auto_add_invalid"
)

// InvalidInputError is a rejected care offering value: Code is its
// registered wire code, Field the JSON key of the value. The HTTP adapter
// answers 400 with both through common.InputRejection. Err keeps the
// diagnostic text, so Error() stays the text the check returned before.
type InvalidInputError struct {
	Code  string
	Field string
	Err   error
}

func (e *InvalidInputError) Error() string { return e.Err.Error() }

func (e *InvalidInputError) Unwrap() error { return e.Err }

// ErrorCode is the registered wire code of the rejection.
func (e *InvalidInputError) ErrorCode() string { return e.Code }

// ErrorField is the JSON key of the rejected value.
func (e *InvalidInputError) ErrorField() string { return e.Field }

// InvalidInput marks err as the rejected value at field with code.
func InvalidInput(code, field string, err error) error {
	return &InvalidInputError{Code: code, Field: field, Err: err}
}
