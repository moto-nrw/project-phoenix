//go:build testhooks

package api

import (
	"context"
	"log/slog"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	settingsModule "github.com/moto-nrw/project-phoenix/modules/settings"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
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
	GuardianNotifier            interface {
		BroadcastChildUpdateToGuardians(tenantID, studentID int64)
	}
	Today func() calendar.Date
}

// BookingMaterializationTestModule is the booking materialization over the
// test database with the owner bindings of the service graph, and the catalog
// it applies the timetable-link rules of.
type BookingMaterializationTestModule struct {
	Bookings careplan.BookingMaterializationCapability
	Catalog  careplan.CareOfferingCatalogCapability
}

// NewBookingMaterializationTestModule composes the booking materialization
// over the test database.
func NewBookingMaterializationTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, options BookingMaterializationTestOptions) (BookingMaterializationTestModule, error) {
	auditCommand, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return BookingMaterializationTestModule{}, err
	}
	students, repos, err := NewStudentTestRepositories(db, auditCommand)
	if err != nil {
		return BookingMaterializationTestModule{}, err
	}
	settings, err := bookingTestSettings(db, unit, options)
	if err != nil {
		return BookingMaterializationTestModule{}, err
	}
	catalog, err := bookingTestCatalog(db, repos.TimetableTestRepositories, settings, options)
	if err != nil {
		return BookingMaterializationTestModule{}, err
	}
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return BookingMaterializationTestModule{}, err
	}
	accounts, err := identityaccessCompose.New(identityaccessCompose.Dependencies{DB: db.Pool(), Observe: func(identityaccessCompose.Observation) {}})
	if err != nil {
		return BookingMaterializationTestModule{}, err
	}
	approved := enrollmentCompose.NewApprovedOfferingProjection(repos.Enrollment(), OfferingStudents{Query: people})
	pickup, err := careplanCompose.NewPickupBaselines(students.CarePlan, approved, func(ctx context.Context) (bool, error) {
		return settings.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
	})
	if err != nil {
		return BookingMaterializationTestModule{}, err
	}
	deps, err := bookingMaterializationOwners(
		repos.Timetable, repos.RosterMaintenance(slog.Default(), nil), people, repos.SchoolCalendar(), repos.Enrollment(),
		approved, NewStudentAuditRows(db, auditCommand).EnrollmentOfferingAdjustment, repos.Person, accounts, pickup, students.CarePlan,
	)
	if err != nil {
		return BookingMaterializationTestModule{}, err
	}
	deps.Catalog = catalog
	deps.Settings = settingsModule.NewCareBookingValues(settings)
	deps.Bookings = students.CarePlan
	deps.Withdrawals = options.Withdrawals
	deps.LockTemplateRecurrence = options.LockRecurrence
	deps.ResyncPickupAutoExcusals = options.ResyncPickupAutoExcusals
	deps.ClearPickupWeekdayExtension = options.ClearPickupWeekdayExtension
	deps.PickupGuardians = options.GuardianNotifier
	deps.Today = options.Today
	deps.Logger = slog.Default()
	bookings, err := newBookingMaterialization(deps, nil)
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
func bookingTestSettings(db transactionruntime.Database, unit tenant.UnitOfWork, options BookingMaterializationTestOptions) (bookingTestSettingsReads, error) {
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

func bookingTestCatalog(db transactionruntime.Database, r TimetableTestRepositories, settings bookingTestSettingsReads, options BookingMaterializationTestOptions) (careplan.CareOfferingCatalogCapability, error) {
	if options.Catalog != nil {
		return options.Catalog, nil
	}
	return newTestCareOfferingCatalog(db, r, settings, CareOfferingCatalogTestOptions{
		LockRecurrence: options.LockRecurrence, Today: options.Today,
	})
}

// BookingMaterializationOption replaces one collaborator of the booking
// materialization a suite composes.
type BookingMaterializationOption func(*BookingMaterializationTestOptions)

// WithBookingSettings replaces the tenant settings the materialization reads
// the care-offerings and authoritative-bookings flags from.
func WithBookingSettings(settings interface {
	ResolveBool(context.Context, string) (bool, error)
}) BookingMaterializationOption {
	return func(options *BookingMaterializationTestOptions) { options.Settings = settings }
}

// WithBookingCatalog applies the suite's care-offering catalog instead of a
// freshly composed one.
func WithBookingCatalog(catalog BookingTestCatalog) BookingMaterializationOption {
	return func(options *BookingMaterializationTestOptions) { options.Catalog = catalog }
}

// WithBookingRecurrenceLock binds the tenant recurrence gate.
func WithBookingRecurrenceLock(lock func(context.Context) error) BookingMaterializationOption {
	return func(options *BookingMaterializationTestOptions) { options.LockRecurrence = lock }
}

// WithBookingWithdrawals binds the complete-withdrawal follow-up of an
// authoritative booking change.
func WithBookingWithdrawals(withdrawals BookingTestWithdrawals) BookingMaterializationOption {
	return func(options *BookingMaterializationTestOptions) { options.Withdrawals = withdrawals }
}

// WithBookingPickupWeekdayExtension binds the close of a manual weekly
// pickup's Timetable task.
func WithBookingPickupWeekdayExtension(clear func(context.Context, int64, int) error) BookingMaterializationOption {
	return func(options *BookingMaterializationTestOptions) { options.ClearPickupWeekdayExtension = clear }
}

// WithBookingToday pins the materialization's calendar day.
func WithBookingToday(today func() calendar.Date) BookingMaterializationOption {
	return func(options *BookingMaterializationTestOptions) { options.Today = today }
}

// NewBookingMaterialization composes Care Plan's booking materialization
// (#3560) over the test database with the owner bindings of the server and
// the suite's options.
func NewBookingMaterialization(db transactionruntime.Database, unit tenant.UnitOfWork, options ...BookingMaterializationOption) (BookingMaterializationTestModule, error) {
	var chosen BookingMaterializationTestOptions
	for _, option := range options {
		option(&chosen)
	}
	return NewBookingMaterializationTestModule(db, unit, chosen)
}
