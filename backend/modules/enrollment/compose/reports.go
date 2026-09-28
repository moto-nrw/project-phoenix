package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/application"
)

// Ports the composition root binds for Enrollment's reports.
type (
	ReportRequests         = application.ReportRequests
	ReportChildren         = application.ReportChildren
	ReportGuardians        = application.ReportGuardians
	ReportSchemas          = application.ReportSchemas
	ReportPhases           = application.ReportPhases
	ReportOfferings        = application.ReportOfferings
	RosterStudent          = application.RosterStudent
	RosterPerson           = application.RosterPerson
	RosterStudents         = application.RosterStudents
	RosterPersons          = application.RosterPersons
	RosterGroups           = application.RosterGroups
	GuardianContactRow     = application.GuardianContactRow
	RosterGuardianContacts = application.RosterGuardianContacts
	RosterCompanions       = application.RosterCompanions
	ClassListEntry         = application.ClassListEntry
	ClassListEntries       = application.ClassListEntries
	PickupSchedules        = application.PickupSchedules
	CareParticipation      = application.CareParticipation
	ReportSettings         = application.ReportSettings
	ExportAccess           = application.ExportAccess
	ExportAccessLog        = application.ExportAccessLog
	ReportDependencies     = application.ReportDependencies
)

// NewReports composes Enrollment's phase reports and the class roster of a
// day over the owners the root binds.
func NewReports(deps ReportDependencies) enrollment.Reports {
	return application.NewReports(deps)
}
