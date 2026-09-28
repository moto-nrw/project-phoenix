package services

import (
	"context"
	"log/slog"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// CareOfferingCatalogTestOptions replace collaborators of the Care Plan
// care-offering catalog (#3559) a suite drives. Nil values keep the tenant
// settings over the test database, no recurrence lock, no roster or pickup
// resync, and the real calendar day.
type CareOfferingCatalogTestOptions struct {
	Settings interface {
		ResolveInt(context.Context, string) (int, error)
	}
	SourcedTemplates careplanCompose.SourcedTemplateResyncer
	Pickup           careplanCompose.PickupResyncer
	LockRecurrence   func(context.Context) error
	Today            func() calendar.Date
}

// CareOfferingCatalogTestModule is the catalog over the test database with
// the same owner bindings as services.Factory.
type CareOfferingCatalogTestModule struct {
	Catalog careplan.CareOfferingCatalogCapability
}

// NewCareOfferingCatalogTestModule composes the catalog over the test
// database.
func NewCareOfferingCatalogTestModule(db *bun.DB, unit tenant.UnitOfWork, options CareOfferingCatalogTestOptions) (CareOfferingCatalogTestModule, error) {
	r, err := repositories.NewTimetableTestRepositories(db)
	if err != nil {
		return CareOfferingCatalogTestModule{}, err
	}
	settings := options.Settings
	if settings == nil {
		module, err := NewSettingsTestModule(db, unit)
		if err != nil {
			return CareOfferingCatalogTestModule{}, err
		}
		settings = module.Settings
	}
	catalog, err := newTestCareOfferingCatalog(r, settings, options)
	if err != nil {
		return CareOfferingCatalogTestModule{}, err
	}
	return CareOfferingCatalogTestModule{Catalog: catalog}, nil
}

// newTestCareOfferingCatalog binds the catalog to the owners of a test
// repository set.
func newTestCareOfferingCatalog(r repositories.TimetableTestRepositories, settings careOfferingSettingsReads, options CareOfferingCatalogTestOptions) (careplan.CareOfferingCatalogCapability, error) {
	return newCareOfferingCatalog(careOfferingCatalogInputs{
		Records: r.CarePlan, Phases: r.Enrollment(), Bookings: r.Enrollment(),
		Timetable: r.Timetable, Calendar: r.SchoolCalendar(), Settings: settings,
		SourcedTemplates: func() careplanCompose.SourcedTemplateResyncer { return options.SourcedTemplates },
		Pickup:           func() careplanCompose.PickupResyncer { return options.Pickup },
		LockRecurrence:   options.LockRecurrence,
		Today:            options.Today,
		Logger:           slog.Default(),
	})
}
