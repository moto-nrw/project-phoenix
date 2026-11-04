// Package enrollmenttest exposes the Enrollment values used by row fixtures.
package enrollmenttest

import (
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// Phase and Date let shared fixtures use owner values without a production dependency.
type Phase = enrollment.Phase
type Date = enrollment.Date

type Request = enrollment.Request
type RequestChild = enrollment.RequestChild
type RequestChildOffering = enrollment.RequestChildOffering

type CareOfferingLink = enrollment.CareOfferingLink

const ChildStatusSubmitted = enrollment.ChildStatusSubmitted

const ChildStatusApproved = enrollment.ChildStatusApproved

// The phase, request and child values the shared fixtures write.
const (
	PhaseKindSchoolYear                  = enrollment.PhaseKindSchoolYear
	PhaseKindCustom                      = enrollment.PhaseKindCustom
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
