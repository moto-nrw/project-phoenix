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
func BookingRowNotFound(err error) error {
	return &bookingRowNotFound{err: err}
}

type bookingRowNotFound struct{ err error }

func (e *bookingRowNotFound) Error() string { return e.err.Error() }

func (e *bookingRowNotFound) Unwrap() []error { return []error{e.err, ports.ErrBookingRowNotFound} }

// BookingMaterializationDependencies bind the booking materialization (#3560)
// to its owners. Catalog is the care-offering catalog composed by
// NewCareOfferingCatalog; the hooks are nil-safe.
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
	// AnnouncePickupChange announces a changed offering pickup projection of
	// the students; it runs after the tenant transaction commits.
	AnnouncePickupChange func(tenantID int64, studentIDs []int64)

	Today  func() calendar.Date
	Logger *slog.Logger
}

// afterCommit defers an announcement until the tenant transaction commits,
// so a rolled-back change stays silent. Nil stays nil.
func afterCommit(announce func(tenantID int64, studentIDs []int64)) func(context.Context, []int64) {
	if announce == nil {
		return nil
	}
	return func(ctx context.Context, studentIDs []int64) {
		tenantID := tenant.FromContext(ctx)
		tenant.RegisterAfterCommit(ctx, func() {
			if tenantID > 0 {
				announce(tenantID, studentIDs)
			}
		})
	}
}

// NewBookingMaterialization composes the Care Plan booking materialization
// over the catalog it applies the timetable-link rules of.
func NewBookingMaterialization(deps BookingMaterializationDependencies) (careplan.BookingMaterializationCapability, error) {
	catalog, ok := deps.Catalog.(*application.CareOfferingCatalog)
	if !ok {
		return nil, errors.New("booking materialization: the care-offering catalog must be composed by NewCareOfferingCatalog")
	}
	return application.NewBookingMaterialization(application.BookingMaterializationDependencies{
		Catalog: catalog, Rosters: deps.Rosters, Templates: deps.Templates, Periods: deps.Periods,
		Enrollment: deps.Enrollment, Students: deps.Students, Settings: deps.Settings, Bookings: deps.Bookings,
		Withdrawals: deps.Withdrawals, Audit: deps.Audit, Pickup: deps.Pickup,
		RunInTx:                     tenant.NewTransactionRunner().RunInTx,
		LockTemplateRecurrence:      deps.LockTemplateRecurrence,
		ResyncPickupAutoExcusals:    deps.ResyncPickupAutoExcusals,
		ClearPickupWeekdayExtension: deps.ClearPickupWeekdayExtension,
		AnnouncePickupChange:        afterCommit(deps.AnnouncePickupChange),
		Today:                       deps.Today,
		Logger:                      deps.Logger,
	})
}
