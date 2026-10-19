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

type RosterEnrollment = ports.RosterEnrollment
type SourcedTemplate = ports.SourcedTemplate
type BookingRosters = ports.BookingRosters
type BookingTemplates = ports.BookingTemplates
type BookingPeriods = ports.BookingPeriods
type BookingPhase = ports.BookingPhase
type BookingRequest = ports.BookingRequest
type BookingChild = ports.BookingChild
type ApprovedOfferingChild = ports.ApprovedOfferingChild
type BookingEnrollment = ports.BookingEnrollment
type BookingStudent = ports.BookingStudent
type BookingStudents = ports.BookingStudents
type BookingSettings = ports.BookingSettings
type BookingCommands = ports.BookingCommands
type BookingWithdrawals = ports.BookingWithdrawals
type AdjustmentAudit = ports.AdjustmentAudit
type PickupWeekdayRow = ports.PickupWeekdayRow
type BookingPickup = ports.BookingPickup

// BookingRowNotFound marks an owner's not-found error for the booking
// materialization: a request, request child, phase or student that does not
// exist in the tenant. The error keeps its text.
func BookingRowNotFound(err error) error { return ports.BookingRowNotFound(err) }

func bookingDependencies(deps BookingMaterializationDependencies, catalog *application.CareOfferingCatalog) application.BookingMaterializationDependencies {
	return application.BookingMaterializationDependencies{
		Catalog: catalog, Rosters: deps.Rosters, Templates: deps.Templates, Periods: deps.Periods,
		Enrollment: deps.Enrollment, Students: deps.Students, Settings: deps.Settings, Bookings: deps.Bookings,
		Withdrawals: deps.Withdrawals, Audit: deps.Audit, Pickup: deps.Pickup,
		RunInTx:                     tenant.NewTransactionRunner().RunInTx,
		LockTemplateRecurrence:      deps.LockTemplateRecurrence,
		ResyncPickupAutoExcusals:    deps.ResyncPickupAutoExcusals,
		ClearPickupWeekdayExtension: deps.ClearPickupWeekdayExtension,
		AnnouncePickupChange:        application.AnnounceOfferingPickupChange(deps.PickupStaff, deps.PickupGuardians, TenantHooks{}, deps.Logger),
		Today:                       deps.Today,
		Logger:                      deps.Logger,
	}
}

// BookingMaterializationDependencies bind the booking materialization (#3560)
// to its owners. Catalog comes from NewCareOfferingCatalog, unset for
// NewCareOfferingCatalogWithBookings; the hooks are nil-safe.
type BookingMaterializationDependencies struct {
	Catalog     careplan.CareOfferingCatalogCapability
	Rosters     BookingRosters
	Templates   BookingTemplates
	Periods     BookingPeriods
	Enrollment  BookingEnrollment
	Students    BookingStudents
	Settings    BookingSettings
	Bookings    BookingCommands
	Withdrawals BookingWithdrawals
	Audit       AdjustmentAudit
	Pickup      BookingPickup

	LockTemplateRecurrence      func(context.Context) error
	ResyncPickupAutoExcusals    func(ctx context.Context, studentIDs []int64) error
	ClearPickupWeekdayExtension func(ctx context.Context, studentID int64, weekday int) error
	// PickupStaff and PickupGuardians hear about a changed offering pickup
	// projection after commit; either may be nil.
	PickupStaff     ports.PickupScheduleStaff
	PickupGuardians ports.PickupScheduleGuardians

	Today  func() calendar.Date
	Logger *slog.Logger
}
