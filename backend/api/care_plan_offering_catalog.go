package api

import (
	"context"
	"log/slog"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The Care Plan care-offering catalog (#3559) reads the Timetable, the School
// Calendar and Enrollment through its own ports. The adapters below bind
// them over the owners' public capabilities and translate their rows into
// the catalog's vocabulary; they add no read of their own.

// careOfferingCatalogInputs are the owners the catalog is composed over.
type careOfferingCatalogInputs struct {
	Records   careplanCompose.CatalogRecords
	Phases    owners.CareOfferingPhaseReads
	Bookings  owners.CareOfferingBookingReads
	Timetable owners.CareOfferingTimetableReads
	Calendar  owners.CareOfferingCalendarReads
	Settings  owners.CareOfferingSettingsReads
	// SourcedTemplates and Pickup resync the sourced rosters and the pickup
	// projection after an offering edit.
	SourcedTemplates careplanCompose.SourcedTemplateResyncer
	Pickup           careplanCompose.PickupResyncer
	LockRecurrence   func(context.Context) error
	Today            func() calendar.Date
	Logger           *slog.Logger
}

// NewEnrollmentCareOfferingValues serves the Care Plan catalog to the
// enrollment routes in Enrollment's public offering values (#2734).
func NewEnrollmentCareOfferingValues(catalog enrollmentCompose.CareOfferingCatalogAdministration) *enrollmentCompose.CareOfferingValues {
	return enrollmentCompose.NewCareOfferingValues(catalog)
}
