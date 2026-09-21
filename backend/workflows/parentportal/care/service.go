// Package care holds the retained Care Plan side of the guardian
// portal: the child's today status, weekly care plan and its change requests,
// booked care offerings and their change requests, and course requests
// (#3227). workflows/parentportal/legacy keeps the public Service contract and
// delegates these methods here; no HTTP path, status code, error string,
// authorization check or tenant scoping changed with the move.
package care

import (
	"context"
	"errors"
	"log/slog"
	"time"

	careplan "github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// Request types of the parent request-sharing ledger that this package
// shares and reads. Wire-stable values; workflows/parentportal/legacy re-exports them.
const (
	RequestShareCareSchedule = "care_schedule"
	RequestShareOffering     = "offering"
)

// MaxParentNoteLen bounds a single note so a parent can't paste a novel
// the staff card then has to render. Generous for a "kurze Nachricht".
const MaxParentNoteLen = 2000

var (
	// ErrNotesDisabled means operations.parent_notes_enabled is off for
	// the child's tenant.
	ErrNotesDisabled = errors.New("parent: parent notes disabled for this school")
	// ErrNoteTooLong means the note body exceeded MaxParentNoteLen.
	ErrNoteTooLong = errors.New("parent: note body too long")
	// ErrEmptyNote means the note body was blank after trimming.
	ErrEmptyNote = errors.New("parent: note body must not be empty")
	// ErrPickupChangeCutoffPassed means the school's same-day cutoff
	// (operations.parent_pickup_change_cutoff_time) has passed, so today's
	// pickup time is closed for guardians (#3163). Later days stay open.
	ErrPickupChangeCutoffPassed = errors.New("parent: same-day pickup change cutoff has passed")
)

// RequestShareVisibility answers whether an account may see one request of
// the child, given the family's current sharing choices.
type RequestShareVisibility interface {
	Allows(requestType string, requestID, accountID, submittedBy int64) bool
}

// RequestSharer is the port to the parent request-sharing ledger, which this
// package does not own. ShareRequestInTx writes the family's recipient choice
// inside the caller's transaction; LoadRequestShareVisibility reads the
// current shares inside the caller's tenant transaction.
type RequestSharer interface {
	ShareRequestInTx(ctx context.Context, accountID, studentID int64, requestType string, requestID int64, recipientProfileIDs []int64) error
	LoadRequestShareVisibility(ctx context.Context, studentID int64) (RequestShareVisibility, error)
}

// Config is the dependency bundle of the retained Care Plan portal services.
type Config struct {
	DB     *bun.DB
	Logger *slog.Logger
	Now    func() time.Time

	ChildRepo     parentModels.ChildRepository
	StudentRepo   usersModels.StudentRepository
	Settings      configService.SettingsService
	Attendance    AttendanceReader
	StatusDayRepo activeModels.StudentStatusDayRepository

	ArrivalSchedules careplan.ArrivalScheduleService
	PickupSchedules  careplan.PickupScheduleService
	CareRequests     carerequests.Service

	CarePeriods      enrollmentSvc.StudentCarePeriodReader
	OfferingHistory  enrollmentSvc.OfferingHistoryReader
	CareOfferingRepo enrollmentModels.CareOfferingRepository
	OfferingChanges  enrollmentSvc.OfferingChangeRequestService

	RequestSharing RequestSharer
}

// Service implements the retained Care Plan portal operations.
type Service struct {
	Config
}

// New wires the retained Care Plan portal services. RequestSharing is required.
func New(cfg Config) *Service {
	if cfg.RequestSharing == nil {
		panic("care: request sharing is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = timezone.Now
	}
	return &Service{Config: cfg}
}

func (s *Service) now() time.Time {
	if s.Now == nil {
		return timezone.Now()
	}
	return s.Now()
}

func (s *Service) todayDate() timezone.Date {
	return timezone.DateFromTime(s.now())
}
