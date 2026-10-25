package parentportal

import "github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"

// Care Plan values the portal hands across its boundary. They name the
// workflow's own snapshots, so a field cannot be lost between two copies
// (#4604); the root still maps the Care Plan contract onto them.
type (
	ArrivalExceptionSnapshot            = care.ArrivalException
	ArrivalScheduleSnapshot             = care.ArrivalSchedule
	CourseCatalogSnapshot               = care.CourseCatalog
	CourseCatalogItemSnapshot           = care.CourseCatalogItem
	CreateCourseRequestInputSnapshot    = care.CreateCourseRequestInput
	CreateOfferingChangeInputSnapshot   = care.CreateOfferingChangeInput
	DepartureExceptionSnapshot          = care.DepartureException
	EffectivePickupTimeSnapshot         = care.EffectivePickupTime
	ExcusedAbsenceRequestSnapshot       = care.ExcusedAbsenceRequest
	ExcusedRequestCreateInputSnapshot   = care.ExcusedRequestCreateInput
	ExcusedRequestEditInputSnapshot     = care.ExcusedRequestEditInput
	GuardianAbsenceReportSnapshot       = care.GuardianAbsenceReport
	GuardianPickupChangeSnapshot        = care.GuardianPickupChange
	NoteDataSnapshot                    = care.NoteData
	OfferingChangeCatalogSnapshot       = care.OfferingChangeCatalog
	OfferingChangeCatalogItemSnapshot   = care.OfferingChangeCatalogItem
	OfferingChangeDecisionSnapshot      = care.OfferingChangeDecision
	OfferingChangeDiffEntrySnapshot     = care.OfferingChangeDiffEntry
	OfferingChangeRequestSnapshot       = care.OfferingChangeRequest
	OfferingChangeRequestedItemSnapshot = care.OfferingChangeRequestedItem
	OfferingChangeSelectionSnapshot     = care.OfferingChangeSelection
	OfferingChangeViewSnapshot          = care.OfferingChangeView
	OfferingOverrideSnapshot            = care.OfferingOverride
	PickupExceptionSnapshot             = care.PickupException
	PickupScheduleSnapshot              = care.PickupSchedule
	StudentScheduleFilterSnapshot       = care.StudentScheduleFilter
	StudentStatusDaySnapshot            = care.StudentStatusDay
)
