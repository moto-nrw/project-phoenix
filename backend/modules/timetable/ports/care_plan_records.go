package ports

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// The retained models/schedule student-schedule and care-request rows are
// Care Plan records (#3351). This composition serves those row repositories
// to the retained consumers; Care Plan's capability arrives through the
// CarePlanRecords seam below, in this composition's own vocabulary, because
// the Timetable owner may not name Care Plan. The composition root binds the
// capability behind it.

// MarkRecordError marks err with one of this composition's record answers
// (ErrCareScheduleRecordNotFound, ...). The message stays err's own, and
// errors.Is still finds every error in err's chain.
func MarkRecordError(mark, err error) error {
	return markedRecordError{mark: mark, err: err}
}

type markedRecordError struct{ mark, err error }

func (e markedRecordError) Error() string   { return e.err.Error() }
func (e markedRecordError) Unwrap() []error { return []error{e.err, e.mark} }

// CareDate is a calendar day as YYYY-MM-DD.
type CareDate string

// CareArrivalSchedule is one weekly arrival time of a child.
type CareArrivalSchedule struct {
	ID              int64
	TenantID        int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	StudentID       int64
	Weekday         int
	ExpectedArrival time.Time
	Notes           *string
	CreatedBy       int64
	Source          string
	SourceClass     string
	SourceLabel     string
}

// CareArrivalException is one dated arrival deviation of a child.
type CareArrivalException struct {
	ID                int64
	TenantID          int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	StudentID         int64
	ExceptionDate     CareDate
	ExpectedArrival   *time.Time
	Reason            *string
	Source            string
	CreatedBy         int64
	CreatedByGuardian *int64
}

// CareArrivalNote is one dated arrival note of a child.
type CareArrivalNote struct {
	ID        int64
	TenantID  int64
	CreatedAt time.Time
	UpdatedAt time.Time
	StudentID int64
	NoteDate  CareDate
	Content   string
	CreatedBy int64
}

// CarePickupSchedule is one weekly pickup time of a child.
type CarePickupSchedule struct {
	ID               int64
	TenantID         int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	StudentID        int64
	Weekday          int
	PickupTime       time.Time
	Notes            *string
	CreatedBy        int64
	Source           string
	CareOfferingID   *int64
	CareOfferingName string
}

// CarePickupException is one dated pickup deviation of a child.
type CarePickupException struct {
	ID                    int64
	TenantID              int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
	StudentID             int64
	ExceptionDate         CareDate
	PickupTime            *time.Time
	Reason                *string
	ExcusedFrom           *time.Time
	ExcusedReason         *string
	ExcusedCreatedBy      *int64
	ExcusedOwnsPickupTime bool
	ExcusedAuto           bool
	Source                string
	CreatedBy             int64
	CreatedByGuardian     *int64
}

// CarePickupNote is a dated or a weekday-recurring pickup note of a child.
type CarePickupNote struct {
	ID        int64
	TenantID  int64
	CreatedAt time.Time
	UpdatedAt time.Time
	StudentID int64
	NoteDate  CareDate
	Weekday   int
	Content   string
	CreatedBy int64
}

// CareScheduleFilter selects schedule rows. Limit, Offset and Sort page a
// retained listing; zero values keep the owner's order and every row.
type CareScheduleFilter struct {
	StudentIDs    []int64
	Weekday       int
	Date          CareDate
	From          CareDate
	To            CareDate
	UpcomingFrom  CareDate
	Limit, Offset int
	Sort          []CareScheduleSort
}

// CareScheduleSort orders a retained listing by one column.
type CareScheduleSort struct {
	Field      string
	Descending bool
}

// CareScheduleRequest is one care-schedule change request.
type CareScheduleRequest struct {
	ID               int64
	TenantID         int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	StudentID        int64
	SubmittedBy      int64
	RequestKind      string
	Payload          json.RawMessage
	Status           string
	DecisionReason   *string
	ReviewedBy       *int64
	ReviewedAt       *time.Time
	AppliedAt        *time.Time
	DecisionSnapshot json.RawMessage
}

// CareScheduleRequestFilter selects care-schedule change requests.
type CareScheduleRequestFilter struct {
	StudentID    int64
	RequestKinds []string
	Statuses     []string
}

// CareScheduleRequestDecision decides one care-schedule change request.
type CareScheduleRequestDecision struct {
	ID         int64
	Status     string
	Reason     *string
	ReviewedBy *int64
	Applied    bool
}

// The Care Plan answers the root marks its errors with, so the row
// repositories keep their retained error shapes.
var (
	ErrCareScheduleRecordNotFound  = errors.New("care plan schedule record not found")
	ErrCareRequestRecordNotFound   = errors.New("care plan schedule request not found")
	ErrCareRequestRecordNotPending = errors.New("care plan schedule request not pending")
	ErrCareRequestRecordNotDecided = errors.New("care plan schedule request not decided")
)

// CareArrivalRecords are Care Plan's arrival schedules, exceptions and notes.
type CareArrivalRecords interface {
	CreateArrivalSchedule(context.Context, CareArrivalSchedule) (CareArrivalSchedule, error)
	FindArrivalSchedule(context.Context, int64) (CareArrivalSchedule, error)
	UpdateArrivalSchedule(context.Context, CareArrivalSchedule) error
	UpsertArrivalSchedule(context.Context, CareArrivalSchedule) (CareArrivalSchedule, error)
	DeleteArrivalSchedulesByStudent(context.Context, int64) error
	ListArrivalSchedules(context.Context, CareScheduleFilter) ([]CareArrivalSchedule, error)

	CreateArrivalException(context.Context, CareArrivalException) (CareArrivalException, error)
	FindArrivalException(context.Context, int64) (CareArrivalException, error)
	UpdateArrivalException(context.Context, CareArrivalException) error
	DeleteArrivalExceptionsByStudent(context.Context, int64) error
	ListArrivalExceptions(context.Context, CareScheduleFilter) ([]CareArrivalException, error)

	CreateArrivalNote(context.Context, CareArrivalNote) (CareArrivalNote, error)
	FindArrivalNote(context.Context, int64) (CareArrivalNote, error)
	UpdateArrivalNote(context.Context, CareArrivalNote) error
	DeleteArrivalNotesByStudent(context.Context, int64) error
	ListArrivalNotes(context.Context, CareScheduleFilter) ([]CareArrivalNote, error)
}

// CarePickupRecords are Care Plan's pickup schedules, exceptions and notes.
type CarePickupRecords interface {
	CreatePickupSchedule(context.Context, CarePickupSchedule) (CarePickupSchedule, error)
	FindPickupSchedule(context.Context, int64) (CarePickupSchedule, error)
	UpdatePickupSchedule(context.Context, CarePickupSchedule) error
	UpsertPickupSchedule(context.Context, CarePickupSchedule) (CarePickupSchedule, error)
	DeletePickupSchedulesByStudent(context.Context, int64) error
	ListPickupSchedules(context.Context, CareScheduleFilter) ([]CarePickupSchedule, error)

	CreatePickupException(context.Context, CarePickupException) (CarePickupException, error)
	FindPickupException(context.Context, int64) (CarePickupException, error)
	UpdatePickupException(context.Context, CarePickupException) error
	DeletePickupExceptionsByStudent(context.Context, int64) error
	ListPickupExceptions(context.Context, CareScheduleFilter) ([]CarePickupException, error)

	CreatePickupNote(context.Context, CarePickupNote) (CarePickupNote, error)
	FindPickupNote(context.Context, int64) (CarePickupNote, error)
	UpdatePickupNote(context.Context, CarePickupNote) error
	DeletePickupNotesByStudent(context.Context, int64) error
	ListPickupNotes(context.Context, CareScheduleFilter) ([]CarePickupNote, error)
}

// CareRequestRecords are Care Plan's care-schedule change requests.
type CareRequestRecords interface {
	CreateCareScheduleRequest(context.Context, CareScheduleRequest) (CareScheduleRequest, error)
	FindCareScheduleRequest(context.Context, int64) (CareScheduleRequest, error)
	ListCareScheduleRequests(context.Context, CareScheduleRequestFilter) ([]CareScheduleRequest, error)
	DecideCareScheduleRequest(context.Context, CareScheduleRequestDecision) error
}

// CarePlanRecords is the Care Plan capability the row repositories run over.
type CarePlanRecords interface {
	CareArrivalRecords
	CarePickupRecords
	CareRequestRecords
}

// RetainedRecordErrors builds the retained repository error shape the row
// callers still classify on; the root binds the Transaction Runtime's
// (transactionruntime.RecordErrors).
type RetainedRecordErrors interface {
	WrapRecordError(operation string, err error) error
	MissingRecordError(operation string) error
}

// CarePlanRows binds the Care Plan records to the row repositories: the
// records, the retained error shape and Care Plan's today.
type CarePlanRows struct {
	Records CarePlanRecords
	Errors  RetainedRecordErrors
	Today   func() CareDate
}

// ScheduleError keeps the retained repository error shape: a missing
// schedule row classifies as not found, everything else is wrapped with its
// operation.
func (c CarePlanRows) ScheduleError(op string, err error) error {
	if errors.Is(err, ErrCareScheduleRecordNotFound) {
		return c.Errors.MissingRecordError(op)
	}
	return c.Errors.WrapRecordError(op, err)
}
