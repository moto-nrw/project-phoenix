package substitutions

import "errors"

var (
	ErrSubstitutionNotFound     = errors.New("substitution not found")
	ErrInvalidSubstitutionData  = errors.New("invalid substitution data")
	ErrSubstitutionDateRange    = errors.New("invalid substitution date range")
	ErrStaffAlreadySubstituting = errors.New("staff member is already substituting another group")
	ErrSubstitutionBackdated    = errors.New("substitutions cannot be created or updated for past dates")
)
