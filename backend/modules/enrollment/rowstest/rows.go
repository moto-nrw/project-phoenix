// Package rowstest exposes the Enrollment values the shared row fixtures in
// backend/test write. It imports the owner's public package only: every
// database test binary links backend/test, so a dependency on the composed
// owner there selected about 200 test binaries for each change inside
// Enrollment (#6784). Suites that need the composed owner use enrollmenttest.
package rowstest

import "github.com/moto-ogs/project-phoenix/modules/enrollment"

type (
	Phase        = enrollment.Phase
	Date         = enrollment.Date
	Request      = enrollment.Request
	RequestChild = enrollment.RequestChild
)

// The phase, request and child values the shared fixtures write.
const (
	ChildStatusSubmitted                 = enrollment.ChildStatusSubmitted
	ChildStatusApproved                  = enrollment.ChildStatusApproved
	PhaseKindSchoolYear                  = enrollment.PhaseKindSchoolYear
	PhaseCareOverflowWaitlist            = enrollment.PhaseCareOverflowWaitlist
	PhaseCareOfferingSelectionOptional   = enrollment.PhaseCareOfferingSelectionOptional
	PhaseCareOfferingSelectionAtLeastOne = enrollment.PhaseCareOfferingSelectionAtLeastOne
	RequestSourcePublic                  = enrollment.RequestSourcePublic
	RequestSourceAdminManual             = enrollment.RequestSourceAdminManual
	ChildActivationImmediate             = enrollment.ChildActivationImmediate
	ChildActivationScheduled             = enrollment.ChildActivationScheduled
)

// Care-offering values as the column CHECK constraints of
// enrollment.care_offerings store them. No public package of the owners
// declares them, so test support names the stored values the shared fixtures
// write.
const (
	DaysOfWeekModeFixed        = "fixed"
	DaysOfWeekModeParentChoice = "parent_choice"
)
