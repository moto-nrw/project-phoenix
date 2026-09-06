package enrollment

import (
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

const (
	PhaseKindSchoolYear                  = capability.PhaseKindSchoolYear
	PhaseKindHoliday                     = capability.PhaseKindHoliday
	PhaseKindCustom                      = capability.PhaseKindCustom
	PhaseCareOverflowWaitlist            = capability.PhaseCareOverflowWaitlist
	PhaseCareOverflowReject              = capability.PhaseCareOverflowReject
	PhaseCareOverflowAllow               = capability.PhaseCareOverflowAllow
	PhaseCareOfferingSelectionOptional   = capability.PhaseCareOfferingSelectionOptional
	PhaseCareOfferingSelectionAtLeastOne = capability.PhaseCareOfferingSelectionAtLeastOne
	PhaseCareOfferingSelectionExactlyOne = capability.PhaseCareOfferingSelectionExactlyOne
	PhaseRolloverModeOptIn               = capability.PhaseRolloverModeOptIn
	PhaseRolloverModeOptOut              = capability.PhaseRolloverModeOptOut
	PhaseAudienceOpen                    = capability.PhaseAudienceOpen
	PhaseAudienceNewStudents             = capability.PhaseAudienceNewStudents
	PhaseAudienceExistingStudents        = capability.PhaseAudienceExistingStudents
	PhaseAudienceLinkedParents           = capability.PhaseAudienceLinkedParents
)
