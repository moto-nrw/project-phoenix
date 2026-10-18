//go:build testhooks

package api

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
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
// the same owner bindings as the service graph.
type CareOfferingCatalogTestModule struct {
	Catalog careplan.CareOfferingCatalogCapability
}

// NewCareOfferingCatalogTestModule composes the catalog over the test
// database.
func NewCareOfferingCatalogTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, options CareOfferingCatalogTestOptions) (CareOfferingCatalogTestModule, error) {
	r, err := NewTimetableTestRepositories(db)
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
	catalog, err := newTestCareOfferingCatalog(db, r, settings, options)
	if err != nil {
		return CareOfferingCatalogTestModule{}, err
	}
	return CareOfferingCatalogTestModule{Catalog: catalog}, nil
}

// newTestCareOfferingCatalog binds the catalog to the owners of a test
// repository set, with the Care Plan and School Calendar owners over db.
func newTestCareOfferingCatalog(db transactionruntime.Database, r TimetableTestRepositories, settings careOfferingSettingsReads, options CareOfferingCatalogTestOptions) (careplan.CareOfferingCatalogCapability, error) {
	care, err := NewTimetableCarePlanRows(db, r, MustNewMembershipTestRepositories(db))
	if err != nil {
		return nil, err
	}
	calendar, err := NewTimetableSchoolCalendar(db)
	if err != nil {
		return nil, err
	}
	return newCareOfferingCatalog(careOfferingCatalogInputs{
		Records: care.CarePlan, Phases: r.Enrollment(), Bookings: r.Enrollment(),
		Timetable: r.Timetable, Calendar: calendar.SchoolCalendar(), Settings: settings,
		SourcedTemplates: func() careplanCompose.SourcedTemplateResyncer { return options.SourcedTemplates },
		Pickup:           func() careplanCompose.PickupResyncer { return options.Pickup },
		LockRecurrence:   options.LockRecurrence,
		Today:            options.Today,
		Logger:           slog.Default(),
	})
}

// CareOfferingCatalogOption replaces one collaborator of the care-offering
// catalog a suite composes.
type CareOfferingCatalogOption func(*CareOfferingCatalogTestOptions)

// WithCareOfferingSettings replaces the tenant settings the catalog reads the
// grade range from.
func WithCareOfferingSettings(settings interface {
	ResolveInt(context.Context, string) (int, error)
}) CareOfferingCatalogOption {
	return func(options *CareOfferingCatalogTestOptions) { options.Settings = settings }
}

// WithCareOfferingRosterResync binds the roster resync of the templates sourcing
// an edited or deleted offering (the decision service in production).
func WithCareOfferingRosterResync(resync interface {
	ResyncTemplatesSourcedFromOffering(context.Context, int64, calendar.Date) error
	DetachTemplatesSourcedFromOffering(context.Context, int64, calendar.Date) error
}) CareOfferingCatalogOption {
	return func(options *CareOfferingCatalogTestOptions) { options.SourcedTemplates = resync }
}

// WithCareOfferingPickupResync binds the pickup projection refresh after an
// offering edit.
func WithCareOfferingPickupResync(resync interface {
	ReconcileOfferingPickupForOffering(context.Context, int64) error
}) CareOfferingCatalogOption {
	return func(options *CareOfferingCatalogTestOptions) { options.Pickup = resync }
}

// WithCareOfferingRecurrenceLock binds the tenant recurrence gate.
func WithCareOfferingRecurrenceLock(lock func(context.Context) error) CareOfferingCatalogOption {
	return func(options *CareOfferingCatalogTestOptions) { options.LockRecurrence = lock }
}

// WithCareOfferingToday pins the catalog's calendar day.
func WithCareOfferingToday(today func() calendar.Date) CareOfferingCatalogOption {
	return func(options *CareOfferingCatalogTestOptions) { options.Today = today }
}

// NewCareOfferingCatalog composes the Care Plan care-offering catalog (#3559)
// over the test database with the owner bindings of the server and the
// suite's options.
func NewCareOfferingCatalog(db transactionruntime.Database, unit tenant.UnitOfWork, options ...CareOfferingCatalogOption) (CareOfferingCatalogTestModule, error) {
	var chosen CareOfferingCatalogTestOptions
	for _, option := range options {
		option(&chosen)
	}
	return NewCareOfferingCatalogTestModule(db, unit, chosen)
}
