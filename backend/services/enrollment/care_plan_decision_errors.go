package enrollment

import (
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// models/enrollment may not import the Care Plan owner, so its offering-change
// values cannot alias the owner contract the way this package's values do
// (#3558). This package returns these errors instead: each keeps the owner
// value's text, which equals the legacy text, and matches both names under
// errors.Is.
var (
	errOfferingChangeNotFound error = &ownerLegacyError{
		owner: careplan.ErrOfferingChangeNotFound, legacy: enrollmentModels.ErrOfferingChangeNotFound,
	}
	errOfferingChangeNotPending error = &ownerLegacyError{
		owner: careplan.ErrOfferingChangeNotPending, legacy: enrollmentModels.ErrOfferingChangeNotPending,
	}
)

// ownerLegacyError keeps the owner value's text while exposing both the owner
// value and the legacy name to errors.Is.
type ownerLegacyError struct {
	owner  error
	legacy error
}

func (e *ownerLegacyError) Error() string { return e.owner.Error() }

func (e *ownerLegacyError) Unwrap() []error { return []error{e.owner, e.legacy} }
