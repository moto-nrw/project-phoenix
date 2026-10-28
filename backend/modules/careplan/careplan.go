// Package careplan is the public Care Plan capability. It owns care offerings,
// student statuses, family requests, exits, companions, care documents, and
// the reversible removal ledger. Other owners use Query or Command; the
// Postgres tables stay hidden behind the module.
package careplan

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/offeringrequests"
)

const DateLayout = excusedrequests.DateLayout

// Date is a calendar day in canonical YYYY-MM-DD form. It is declared in the
// excused-absence contract so that contract stays self-contained; the facade
// and every other Care Plan surface use this alias.
type Date = excusedrequests.Date

const (
	OfferingOrderCatalog      = "catalog"
	OfferingOrderID           = "id"
	OfferingOrderPhaseCatalog = "phase_catalog"

	ChangeOrderReviewed = "reviewed"
	ChangeOrderCreated  = "created"
	ChangeOrderUpdated  = "updated"

	OfferingChangePending   = "pending"
	OfferingChangeApproved  = "approved"
	OfferingChangeRejected  = "rejected"
	OfferingChangeWithdrawn = "withdrawn"
	OfferingChangeDone      = "done"
	OfferingChangeCareEnded = "care_ended"

	GuardianPermissionPortalAccess     = "parent_portal.access"
	GuardianPermissionEnrollmentsView  = "parent_portal.enrollments.view"
	GuardianPermissionEnrollmentSubmit = "parent_portal.enrollment.submit"

	StudentStatusPending  = "pending"
	StudentStatusActive   = "active"
	StudentStatusInactive = "inactive"
	StudentStatusAlumnus  = "alumnus"
)

var (
	ErrCareOfferingNotFound = errors.New("care offering not found")
	// ErrOfferingChangeNotFound and ErrOfferingChangeNotPending are part of the
	// decision error contract (decision_errors.go): handlers render their texts.
	ErrOfferingChangeNotFound     = errors.New("enrollment: offering change request not found")
	ErrOfferingChangeNotPending   = errors.New("enrollment: offering change request is not pending")
	ErrOfferingChangeAlreadyOpen  = errors.New("offering change request already pending")
	ErrInvalidCareOffering        = errors.New("invalid care offering")
	ErrInvalidOfferingChange      = errors.New("invalid offering change request")
	ErrCareOfferingTriggerInvalid = errors.New("care offering auto trigger is outside the tenant")
)

type InvalidError struct {
	Kind   error
	Reason string
}

func (e *InvalidError) Error() string { return e.Reason }
func (e *InvalidError) Unwrap() error { return e.Kind }

// CareOffering is the owner view of one enrollment.care_offerings row.
// Calendar dates do not occur on the row. JSON-backed rules remain raw so the
// capability does not leak the enrollment package's model types.
type CareOffering struct {
	ID                  int64
	TenantID            int64
	CreatedAt           time.Time
	UpdatedAt           time.Time
	PhaseID             int64
	ActivityGroupID     *int64
	Name                string
	Description         *string
	DaysOfWeekMode      string
	AvailableDays       []string
	IncludesHolidayCare bool
	IncludesLunch       bool
	Capacity            *int
	PriceCents          *int
	IsActive            bool
	IsRequired          bool
	CountsAsCare        bool
	AutoAddGradeLevels  []int
	AvailabilityRule    json.RawMessage
	SortOrder           int
	SelectionGroup      string
	SelectionRule       string
	PickupTimes         map[string]string
	// Translations is the school-written translation document of Name and
	// Description (#3377). Enrollment owns its shape; Care Plan stores it.
	Translations              json.RawMessage
	AutoAddTriggerOfferingIDs []int64
	// TemplateGapAcks maps a planning period id to the school classes whose
	// booked children need no Regeltermin of this offering in that period,
	// as the school decided (#5140); "" stands for children without a class.
	TemplateGapAcks map[int64][]string
}

// CareOfferingFields is the writable catalog state. Trigger IDs are part of
// the command so the offering and its selection links commit atomically.
type CareOfferingFields struct {
	PhaseID             int64
	ActivityGroupID     *int64
	Name                string
	Description         *string
	DaysOfWeekMode      string
	AvailableDays       []string
	IncludesHolidayCare bool
	IncludesLunch       bool
	Capacity            *int
	PriceCents          *int
	IsActive            bool
	IsRequired          bool
	CountsAsCare        bool
	AutoAddGradeLevels  []int
	AvailabilityRule    json.RawMessage
	SortOrder           int
	SelectionGroup      string
	SelectionRule       string
	PickupTimes         map[string]string
	// Translations is the school-written translation document of Name and
	// Description (#3377). Enrollment owns its shape; Care Plan stores it.
	Translations              json.RawMessage
	AutoAddTriggerOfferingIDs []int64
}

type CreateCareOffering struct{ CareOfferingFields }

type UpdateCareOffering struct {
	ID int64
	CareOfferingFields
}

// CareOfferingFilter has closed filtering and ordering semantics. Nil IDs
// means no ID filter; an explicitly empty slice matches no rows.
type CareOfferingFilter struct {
	IDs              []int64
	PhaseIDs         []int64
	ActivityGroupIDs []int64
	ActiveOnly       bool
	LockForUpdate    bool
	Order            string
}

// OfferingChangeRequest is one requested effective booking change. Dates use
// DateLayout. Payload and snapshot are deliberately opaque JSON contracts.
type OfferingChangeRequest = offeringrequests.Request

type OfferingChangeFilter struct {
	IDs           []int64
	StudentID     int64
	StudentIDs    []int64
	Statuses      []string
	UrgentOnly    *bool
	UrgentDate    string
	BeforeInstant time.Time
	BeforeID      int64
	Limit         int
	LockForUpdate bool
	Order         string
}

type UpdatePendingOfferingChange struct {
	ID            int64
	Payload       json.RawMessage
	EffectiveFrom string
	ParentNote    *string
}

type DecideOfferingChange struct {
	ID         int64
	Status     string
	Reason     *string
	ReviewedBy *int64
	Applied    bool
}

type Query interface {
	OfferingBookingQueries
	CareRecordsQuery
	WithdrawalQuery
	StudentSchedulesQuery
	CareRequestsQuery
	StudentStatusDaysQuery
	FindCareOffering(context.Context, int64) (CareOffering, error)
	ListCareOfferings(context.Context, CareOfferingFilter) ([]CareOffering, error)
	CountCareOfferingsByPhase(context.Context, int64) (int, error)

	FindOfferingChange(context.Context, int64, bool) (OfferingChangeRequest, error)
	ListOfferingChanges(context.Context, OfferingChangeFilter) ([]OfferingChangeRequest, error)
}

type Command interface {
	OfferingBookingCommands
	CareRecordsCommand
	WithdrawalCommand
	StudentDeletionCommand
	StudentSchedulesCommand
	CareRequestsCommand
	StudentStatusDaysCommand
	CareOfferingTemplateGapCommand
	CreateCareOffering(context.Context, CreateCareOffering) (CareOffering, error)
	UpdateCareOffering(context.Context, UpdateCareOffering) (CareOffering, error)
	DeleteCareOffering(context.Context, int64) error
	ReplaceAutoAddTriggers(context.Context, int64, []int64) error

	CreateOfferingChange(context.Context, OfferingChangeRequest) (OfferingChangeRequest, error)
	UpdateOfferingChangeEffectiveFrom(context.Context, int64, string) error
	UpdateApprovedCompleteWithdrawal(context.Context, int64, bool) error
	UpdatePendingOfferingChange(context.Context, UpdatePendingOfferingChange) error
	DecideOfferingChange(context.Context, DecideOfferingChange) error
	UpdateOfferingChangeSnapshot(context.Context, int64, json.RawMessage) error
	ClosePendingOfferingChanges(context.Context, []int64, string, *int64, time.Time) (int64, error)
}

type Capability interface {
	Query
	Command
	WeekdayPickupNotesCommand
	DepartureExceptions
	CareDayLocker
}

// CareDayLocker exposes the Care Plan owner's canonical lock order to other
// owners that change effective care for one child-day.
type CareDayLocker interface {
	LockStudentAndExceptionDay(context.Context, int64, string) error
	LockExceptionDay(context.Context, int64, string) error
}

// Owner is the composed Care Plan owner: the capability and the nightly
// status-day retention.
type Owner interface {
	Capability
	StudentStatusDayRetention
}

// ErrorCode is the stable low-cardinality label used by runtime evidence.
func ErrorCode(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrCareOfferingNotFound), errors.Is(err, ErrOfferingChangeNotFound), errors.Is(err, ErrCareDocumentNotFound), errors.Is(err, ErrStudentScheduleNotFound), errors.Is(err, ErrStudentStatusDayNotFound),
		errors.Is(err, ErrExcusedRequestNotFound), errors.Is(err, ErrCareScheduleRequestNotFound), errors.Is(err, ErrStudentDataRequestNotFound):
		return "not_found"
	case errors.Is(err, ErrOfferingChangeNotPending), errors.Is(err, ErrExcusedRequestNotPending),
		errors.Is(err, ErrCareScheduleRequestNotPending), errors.Is(err, ErrStudentDataRequestNotPending):
		return "not_pending"
	case errors.Is(err, ErrExcusedRequestNotDecided), errors.Is(err, ErrCareScheduleRequestNotDecided), errors.Is(err, ErrStudentDataRequestNotDecided):
		return "not_decided"
	case errors.Is(err, ErrOfferingChangeAlreadyOpen):
		return "already_pending"
	case errors.Is(err, ErrInvalidCareOffering), errors.Is(err, ErrInvalidOfferingChange),
		errors.Is(err, ErrInvalidCareExit), errors.Is(err, ErrInvalidCompanion), errors.Is(err, ErrInvalidCareDocument),
		errors.Is(err, ErrCareExitInvalidReason), errors.Is(err, ErrCareExitNoteRequired),
		errors.Is(err, ErrCareExitNoteNotAllowed), errors.Is(err, ErrCareExitNoteTooLong), errors.Is(err, ErrInvalidStudentSchedule):
		return "invalid"
	case errors.Is(err, ErrCareOfferingTriggerInvalid):
		return "invalid_trigger"
	default:
		return "internal_error"
	}
}
