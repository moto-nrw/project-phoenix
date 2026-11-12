package compose

import (
	"context"
	"log/slog"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
)

type RosterEnrollment = ports.RosterEnrollment
type SourcedTemplate = ports.SourcedTemplate
type BookingRosters = ports.BookingRosters
type RosterChanges = ports.RosterChanges
type RosterReconcile = ports.RosterReconcile
type TemplateSourceChange = ports.TemplateSourceChange
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

// BookingRowNotFound marks a missing booking row while preserving its cause.
func BookingRowNotFound(err error) error { return ports.BookingRowNotFound(err) }

func bookingDependencies(deps BookingMaterializationDependencies, catalog *application.CareOfferingCatalog) application.BookingMaterializationDependencies {
	return application.BookingMaterializationDependencies{
		Catalog: catalog, Rosters: deps.Rosters, Templates: deps.Templates, Periods: deps.Periods,
		Enrollment: deps.Enrollment, Students: deps.Students, Settings: deps.Settings, Bookings: deps.Bookings,
		Withdrawals: deps.Withdrawals, Audit: deps.Audit, Pickup: deps.Pickup, ManualPlans: deps.ManualPlans,
		RunInTx: tenant.NewTransactionRunner().RunInTx, LockTemplateRecurrence: deps.LockTemplateRecurrence,
		ResyncPickupAutoExcusals:    deps.ResyncPickupAutoExcusals,
		ClearPickupWeekdayExtension: deps.ClearPickupWeekdayExtension,
		AnnouncePickupChange:        application.AnnounceOfferingPickupChange(deps.PickupStaff, deps.PickupGuardians, TenantHooks{}, deps.Logger),
		PlanningStaffing:            deps.PlanningStaffing,
		Today:                       deps.Today, Logger: deps.Logger,
	}
}

// BookingMaterializationDependencies bind bookings to their owners.
// Catalog stays unset for NewCareOfferingCatalogWithBookings.
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
	ManualPlans ports.BookingManualPlans

	LockTemplateRecurrence      func(context.Context) error
	ResyncPickupAutoExcusals    func(ctx context.Context, studentIDs []int64) error
	ClearPickupWeekdayExtension func(ctx context.Context, studentID int64, weekday int) error
	// These optional ports announce committed booking and pickup changes (#5498).
	PickupStaff      ports.PickupScheduleStaff
	PickupGuardians  ports.PickupScheduleGuardians
	PlanningStaffing ports.BookingPlanningStaffing

	Today  func() calendar.Date
	Logger *slog.Logger
}
