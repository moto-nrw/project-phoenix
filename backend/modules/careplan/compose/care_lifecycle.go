package compose

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// The care lifecycle, the companion graph and the child documents (#3427)
// drive owners around Care Plan through consumer-owned ports. The composition
// that holds those owners binds them here.
type (
	CareLifecycleOwners      = application.CareLifecycleOwners
	CareStudentDirectory     = ports.CareStudentDirectory
	CareExitRoster           = ports.CareExitRoster
	CareExitBookings         = ports.CareExitBookings
	CareExitEnrollment       = ports.CareExitEnrollment
	CareExitPresence         = ports.CareExitPresence
	CareCalendarPeriods      = ports.CareCalendarPeriods
	CareExitTagReleaser      = ports.CareExitTagReleaser
	CareEndRecorder          = ports.CareEndRecorder
	CareStudent              = domain.CareStudent
	CarePerson               = domain.CarePerson
	CareExitBooking          = domain.CareExitBooking
	CareExitBookingCap       = domain.CareExitBookingCap
	CareExitBookingChanges   = domain.CareExitBookingChanges
	CareExitBookingRestore   = domain.CareExitBookingRestore
	CareExitApplication      = domain.CareExitApplication
	CareExitOfferingLink     = domain.CareExitOfferingLink
	CareExitOfferingSnapshot = domain.CareExitOfferingSnapshot
	CareExitOfferingRestore  = domain.CareExitOfferingRestore

	CompanionStudents = ports.CompanionStudents
	CompanionStudent  = domain.CompanionStudent

	StudentDocumentAccess      = ports.StudentDocumentAccess
	StudentDocumentPermissions = ports.StudentDocumentPermissions
	StudentDocumentAudit       = ports.StudentDocumentAudit
)

// CareLifecycleDependencies wires the care lifecycle over the Care Plan
// records the same graph composed.
type CareLifecycleDependencies struct {
	DB      *bun.DB
	Records careplan.Capability
	Owners  CareLifecycleOwners
	// LockCareBookingWrites is the transaction-scoped gate authoritative
	// booking writes take.
	LockCareBookingWrites func(context.Context) error
	// BookingsAuthoritative resolves the school's booking-led care setting.
	BookingsAuthoritative func(context.Context) (bool, error)
	// Fingerprint returns the lowercase hexadecimal SHA-256 of the content.
	Fingerprint func([]byte) string
	Logger      *slog.Logger
	// Today defaults to the Berlin calendar day.
	Today func() calendar.Date
}

// NewCareLifecycle composes the one care-lifecycle capability.
func NewCareLifecycle(deps CareLifecycleDependencies) (careplan.CareLifecycle, error) {
	if deps.DB == nil {
		return nil, errors.New("care lifecycle compose: database is required")
	}
	today := deps.Today
	if today == nil {
		today = calendar.TodayDate
	}
	return application.NewCareLifecycle(application.CareLifecycleDependencies{
		Owners: deps.Owners, Records: deps.Records,
		Directory:             postgres.New(carePlanDatabase(deps.DB)),
		Unit:                  tenant.NewTransactionRunner().RunInTx,
		LockCareBookingWrites: deps.LockCareBookingWrites,
		BookingsAuthoritative: deps.BookingsAuthoritative,
		Fingerprint:           deps.Fingerprint,
		Logger:                deps.Logger,
		Today:                 today,
	})
}

// NewCompanions composes the companion capability over the Care Plan records
// and the People Directory rows the companion rules read and lock.
func NewCompanions(records careplan.Capability, students CompanionStudents) (careplan.StudentCompanions, error) {
	return application.NewCompanions(records, students)
}

// StudentDocumentDependencies wires the child document capability.
type StudentDocumentDependencies struct {
	DB          *bun.DB
	Records     careplan.Capability
	Access      StudentDocumentAccess
	Permissions StudentDocumentPermissions
	Audit       StudentDocumentAudit
}

// NewStudentDocuments composes the child document capability. Every command
// commits its own tenant transaction before it returns.
func NewStudentDocuments(deps StudentDocumentDependencies) (careplan.StudentDocuments, error) {
	if deps.DB == nil {
		return nil, errors.New("care plan documents compose: database is required")
	}
	documents, err := application.NewStudentDocuments(application.StudentDocumentDependencies{
		Records: deps.Records, Access: deps.Access, Permissions: deps.Permissions, Audit: deps.Audit,
		Transaction: ownTenantTransaction(deps.DB),
	})
	if err != nil {
		return nil, err
	}
	return studentDocuments{StudentDocuments: documents, documentFileRecords: documentFileRecords{records: deps.Records}}, nil
}

// studentDocuments joins the authorized document application with the file
// bookkeeping File Storage's coordinator settles.
type studentDocuments struct {
	*application.StudentDocuments
	documentFileRecords
}

var _ careplan.StudentDocuments = studentDocuments{}

// documentFileRecords binds File Storage's coordinator store to Care Plan's
// document records. The coordinator acts after its own authorization and
// needs no rule of the Dokumente tab, and each record command already commits
// its own tenant transaction.
type documentFileRecords struct{ records careplan.Capability }

func (r documentFileRecords) MarkFileDeleted(ctx context.Context, documentID int64) error {
	return r.records.MarkCareDocumentFileDeleted(ctx, documentID)
}

func (r documentFileRecords) MarkQueuedCleanupCompleteByFilename(ctx context.Context, storedName string) error {
	return r.records.CompleteCareDocumentCleanupByFilename(ctx, storedName)
}

func (r documentFileRecords) ActivateQueuedCleanup(ctx context.Context, storedName string) error {
	return r.records.ActivateCareDocumentCleanup(ctx, storedName)
}

// ownTenantTransaction runs every command in a transaction of its own for the
// request's school, committed before the command returns.
func ownTenantTransaction(db *bun.DB) ports.UnitOfWork {
	return func(ctx context.Context, command func(context.Context) error) error {
		return tenant.WithTenantTx(ctx, db, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
			return command(txCtx)
		})
	}
}
