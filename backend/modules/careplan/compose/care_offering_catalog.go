package compose

import (
	"context"
	"errors"
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
type OfferingSourceRefusal = ports.OfferingSourceRefusal
type OfferingSourceRefusalKind = ports.OfferingSourceRefusalKind

// The offering-source refusals the editor words itself (#2516).
const (
	OfferingSourceTooMany       = ports.OfferingSourceTooMany
	OfferingSourceNotFound      = ports.OfferingSourceNotFound
	OfferingSourceInactive      = ports.OfferingSourceInactive
	OfferingSourceMixedPhases   = ports.OfferingSourceMixedPhases
	OfferingSourceOutsidePeriod = ports.OfferingSourceOutsidePeriod
)

type OfferingGradeCount = ports.OfferingGradeCount
type SourcedTemplateResyncer = ports.SourcedTemplateResyncer
type PickupResyncer = ports.PickupResyncer

// CatalogRowNotFound marks an owner's not-found error for the catalog: a
// phase, activity group or calendar period that does not exist in the
// tenant. The error keeps its text.
func CatalogRowNotFound(err error) error { return ports.CatalogRowNotFound(err) }

// CareOfferingCatalogDependencies bind the catalog to its owners and the
// resyncers an edit keeps in step.
type CareOfferingCatalogDependencies struct {
	Records      CatalogRecords
	Phases       CatalogPhases
	Timetable    CatalogTimetable
	Calendar     CatalogCalendar
	Bookings     CatalogBookings
	Settings     CatalogSettings
	Translations CatalogTranslations
	SourceRules  OfferingSourceRules

	SourcedTemplates SourcedTemplateResyncer
	Pickup           PickupResyncer

	LockTemplateRecurrence func(context.Context) error
	Today                  func() calendar.Date
	Logger                 *slog.Logger
}

// NewCareOfferingCatalogWithBookings composes the catalog and the booking
// materialization it resyncs through; their mutual references must be unset.
func NewCareOfferingCatalogWithBookings(catalog CareOfferingCatalogDependencies, bookings BookingMaterializationDependencies) (careplan.CareOfferingCatalogCapability, careplan.BookingMaterializationCapability, error) {
	if bookings.Catalog != nil {
		return nil, nil, errors.New("care offering catalog with bookings: the catalog is composed here and must be unset")
	}
	return application.NewCareOfferingCatalogWithBookings(catalogDependencies(catalog), bookingDependencies(bookings, nil))
}

func catalogDependencies(deps CareOfferingCatalogDependencies) application.CareOfferingCatalogDependencies {
	return application.CareOfferingCatalogDependencies{
		Records: deps.Records, Phases: deps.Phases, Timetable: deps.Timetable, Calendar: deps.Calendar,
		Bookings: deps.Bookings, Settings: deps.Settings, Translations: deps.Translations, SourceRules: deps.SourceRules,
		SourcedTemplates: deps.SourcedTemplates, Pickup: deps.Pickup,
		LockTemplateRecurrence: deps.LockTemplateRecurrence,
		MarkRollback:           tenant.MarkRollback,
		Today:                  deps.Today,
		Logger:                 deps.Logger,
	}
}

// NewCareOfferings serves the catalog, the booking materialization it feeds,
// the offering-change review and the pickup adjustments as one Care Plan
// capability.
func NewCareOfferings(
	catalog careplan.CareOfferingCatalogCapability,
	bookings careplan.BookingMaterializationCapability,
	changes careplan.OfferingChangeCapability,
	adjustments careplan.PickupAdjustments,
) careplan.CareOfferingCapability {
	return application.CareOfferings{CareOfferingCatalogCapability: catalog, BookingMaterializationCapability: bookings, OfferingChangeCapability: changes, PickupAdjustments: adjustments}
}
