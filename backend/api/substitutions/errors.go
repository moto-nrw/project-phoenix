package substitutions

import "errors"

var (
	ErrSubstitutionNotFound      = errors.New("substitution not found")
	ErrSubstitutionConflict      = errors.New("substitution conflict")
	ErrInvalidSubstitutionData   = errors.New("invalid substitution data")
	ErrSubstitutionDateRange     = errors.New("invalid substitution date range")
	ErrStaffAlreadySubstituting  = errors.New("staff member is already substituting another group")
	ErrGroupAlreadyHasSubstitute = errors.New("group already has an active substitute")
)
