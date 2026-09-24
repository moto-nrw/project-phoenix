package compose

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type CatalogRecords = ports.CatalogRecords
type CatalogPhases = ports.CatalogPhases
type CatalogTimetable = ports.CatalogTimetable
type CatalogCalendar = ports.CatalogCalendar
type CatalogBookings = ports.CatalogBookings
type CatalogSettings = ports.CatalogSettings
type CatalogTranslations = ports.CatalogTranslations
type OfferingSourceRules = ports.OfferingSourceRules
type OfferingGradeCount = ports.OfferingGradeCount
type SourcedTemplateResyncer = ports.SourcedTemplateResyncer
type PickupResyncer = ports.PickupResyncer

// CatalogRowNotFound marks an owner's not-found error for the catalog: a
// phase, activity group or calendar period that does not exist in the
// tenant. The error keeps its text.
func CatalogRowNotFound(err error) error {
	return &catalogRowNotFound{err: err}
}

type catalogRowNotFound struct{ err error }

func (e *catalogRowNotFound) Error() string { return e.err.Error() }

func (e *catalogRowNotFound) Unwrap() []error { return []error{e.err, ports.ErrCatalogRowNotFound} }

// CareOfferingCatalogDependencies bind the catalog to its owners. The
// resolvers are called on every edit, so the composition can bind the
// Enrollment decision service after the catalog exists.
type CareOfferingCatalogDependencies struct {
	Records      CatalogRecords
	Phases       CatalogPhases
	Timetable    CatalogTimetable
	Calendar     CatalogCalendar
	Bookings     CatalogBookings
	Settings     CatalogSettings
	Translations CatalogTranslations
	SourceRules  OfferingSourceRules

	SourcedTemplates func() SourcedTemplateResyncer
	Pickup           func() PickupResyncer

	LockTemplateRecurrence func(context.Context) error
	Today                  func() calendar.Date
	Logger                 *slog.Logger
}

// NewCareOfferingCatalog composes the Care Plan care-offering catalog.
func NewCareOfferingCatalog(deps CareOfferingCatalogDependencies) (careplan.CareOfferingCatalogCapability, error) {
	return application.NewCareOfferingCatalog(application.CareOfferingCatalogDependencies{
		Records: deps.Records, Phases: deps.Phases, Timetable: deps.Timetable, Calendar: deps.Calendar,
		Bookings: deps.Bookings, Settings: deps.Settings, Translations: deps.Translations, SourceRules: deps.SourceRules,
		SourcedTemplates: deps.SourcedTemplates, Pickup: deps.Pickup,
		LockTemplateRecurrence: deps.LockTemplateRecurrence,
		MarkRollback:           tenant.MarkRollback,
		Today:                  deps.Today,
		Logger:                 deps.Logger,
	})
}
