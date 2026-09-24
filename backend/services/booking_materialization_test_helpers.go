package services

import (
	"context"
	"log/slog"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	auditService "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// BookingTestCatalog is the care-offering catalog a suite hands the booking
// materialization.
type BookingTestCatalog = careplan.CareOfferingCatalogCapability

// BookingTestWithdrawals is the complete-withdrawal follow-up a suite hands
// the booking materialization.
type BookingTestWithdrawals = careplanCompose.BookingWithdrawals

// BookingMaterializationTestOptions replace collaborators of Care Plan's
// booking materialization (#3560) a suite drives. Nil values keep the tenant
// settings over the test database, a catalog composed like the server's, no
// recurrence lock, no withdrawal follow-up, no pickup auto-excusal resync or
// weekday task and no announcement, and the real calendar day.
type BookingMaterializationTestOptions struct {
	Settings interface {
		ResolveBool(context.Context, string) (bool, error)
	}
	Catalog                     careplan.CareOfferingCatalogCapability
	LockRecurrence              func(context.Context) error
	Withdrawals                 careplanCompose.BookingWithdrawals
	ResyncPickupAutoExcusals    func(ctx context.Context, studentIDs []int64) error
	ClearPickupWeekdayExtension func(ctx context.Context, studentID int64, weekday int) error
	Broadcaster                 realtime.Broadcaster
	GuardianNotifier            interface {
		BroadcastChildUpdateToGuardians(tenantID, studentID int64)
	}
	Today func() calendar.Date
}

// BookingMaterializationTestModule is the booking materialization over the
// test database with the owner bindings of services.Factory, and the catalog
// it applies the timetable-link rules of.
type BookingMaterializationTestModule struct {
	Bookings careplan.BookingMaterializationCapability
	Catalog  careplan.CareOfferingCatalogCapability
}

// NewBookingMaterializationTestModule composes the booking materialization
// over the test database.
func NewBookingMaterializationTestModule(db *bun.DB, unit tenant.UnitOfWork, options BookingMaterializationTestOptions) (BookingMaterializationTestModule, error) {
	auditCommand, err := auditService.NewCommand(repositories.NewTestAuditStore(db), func(auditService.AppendObservation) {})
	if err != nil {
		return BookingMaterializationTestModule{}, err
	}
	repos, err := repositories.NewStudentTestRepositories(db, auditCommand)
	if err != nil {
		return BookingMaterializationTestModule{}, err
	}
	settings, err := bookingTestSettings(db, unit, options)
	if err != nil {
		return BookingMaterializationTestModule{}, err
	}
	catalog, err := bookingTestCatalog(repos.TimetableTestRepositories, settings, options)
	if err != nil {
		return BookingMaterializationTestModule{}, err
	}
	people, err := repositories.NewPeopleDirectory(db)
	if err != nil {
		return BookingMaterializationTestModule{}, err
	}
	accounts, err := identityaccessCompose.New(identityaccessCompose.Dependencies{DB: db, Observe: func(identityaccessCompose.Observation) {}})
	if err != nil {
		return BookingMaterializationTestModule{}, err
	}
	approved := enrollment.NewApprovedOfferingProjection(repos.Enrollment(), offeringStudents{query: people})
	pickup, err := careplanCompose.NewPickupBaselines(repos.CarePlan, approved, func(ctx context.Context) (bool, error) {
		return settings.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
	})
	if err != nil {
		return BookingMaterializationTestModule{}, err
	}
	bookings, err := newBookingMaterialization(bookingMaterializationInputs{
		Catalog: catalog, Timetable: repos.Timetable, Rosters: repos.RosterMaintenance(slog.Default(), nil),
		Students: people, Periods: repos.SchoolCalendar(), Enrollment: repos.Enrollment(), Approved: approved,
		Settings: settings, Bookings: repos.CarePlan, Withdrawals: options.Withdrawals,
		Adjustments: repos.EnrollmentOfferingAdjustment, Persons: repos.Person, Accounts: accounts,
		Pickup: pickup, PickupRows: repos.CarePlan,
		LockRecurrence:              options.LockRecurrence,
		ResyncPickupAutoExcusals:    options.ResyncPickupAutoExcusals,
		ClearPickupWeekdayExtension: options.ClearPickupWeekdayExtension,
		Broadcaster:                 options.Broadcaster,
		GuardianNotifier:            options.GuardianNotifier,
		Today:                       options.Today,
		Logger:                      slog.Default(),
	})
	if err != nil {
		return BookingMaterializationTestModule{}, err
	}
	return BookingMaterializationTestModule{Bookings: bookings, Catalog: catalog}, nil
}

type bookingTestSettingsReads interface {
	ResolveBool(context.Context, string) (bool, error)
	ResolveInt(context.Context, string) (int, error)
}

// bookingTestSettings are the suite's settings, or the tenant settings over
// the test database. The catalog also reads the grade range from them.
func bookingTestSettings(db *bun.DB, unit tenant.UnitOfWork, options BookingMaterializationTestOptions) (bookingTestSettingsReads, error) {
	if settings, ok := options.Settings.(bookingTestSettingsReads); ok {
		return settings, nil
	}
	module, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return nil, err
	}
	if options.Settings != nil {
		return bookingTestOverlaySettings{bools: options.Settings, ints: module.Settings}, nil
	}
	return module.Settings, nil
}

// bookingTestOverlaySettings answers the flags from the suite's settings and
// the grade range from the tenant settings.
type bookingTestOverlaySettings struct {
	bools interface {
		ResolveBool(context.Context, string) (bool, error)
	}
	ints interface {
		ResolveInt(context.Context, string) (int, error)
	}
}

func (s bookingTestOverlaySettings) ResolveBool(ctx context.Context, key string) (bool, error) {
	return s.bools.ResolveBool(ctx, key)
}

func (s bookingTestOverlaySettings) ResolveInt(ctx context.Context, key string) (int, error) {
	return s.ints.ResolveInt(ctx, key)
}

func bookingTestCatalog(r repositories.TimetableTestRepositories, settings bookingTestSettingsReads, options BookingMaterializationTestOptions) (careplan.CareOfferingCatalogCapability, error) {
	if options.Catalog != nil {
		return options.Catalog, nil
	}
	return newTestCareOfferingCatalog(r, settings, CareOfferingCatalogTestOptions{
		LockRecurrence: options.LockRecurrence, Today: options.Today,
	})
}
