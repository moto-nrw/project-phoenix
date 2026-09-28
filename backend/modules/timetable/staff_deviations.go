package timetable

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Staff deviations (Vertretungsplan, #1840/#1886, #3424 slice S3) are the
// day-wide absence, presence and substitution writes on planned and active
// blocks. They are the ONLY write path for these deviations; every write
// appends its Änderungsprotokoll entry in the same tenant transaction, and a
// protocol failure rolls the whole mutation back (fail closed). Every method
// runs in the caller's tenant transaction.

// Substitute action strings — the stable per-instance action vocabulary the
// endpoints return.
const (
	SubstituteActionSubstituted       = "substituted"
	SubstituteActionAlreadySubstitute = "already_substituted"
	SubstituteActionAlreadyOnInstance = "already_on_instance"
	SubstituteActionMarkedAbsent      = "marked_absent"
	SubstituteActionAlreadyAbsent     = "already_absent"
	SubstituteActionMarkedPresent     = "marked_present"
	SubstituteActionRemoved           = "substitute_removed"
)

// MaxBulkSubstitutionDates caps one Sammel-Vertretung. 31 dates cover any
// realistic sick-leave window; longer absences are entered in slices.
const MaxBulkSubstitutionDates = 31

// DeviationError carries the exact HTTP mapping a staffing save renders, so
// the deviations wire contract (status, code, message) stays byte-identical.
// Cause is set only for 500 responses that wrap an internal error for logs
// while showing ClientMsg to the client.
type DeviationError struct {
	Status    int
	Code      string
	ClientMsg string
	Cause     error
}

func (e *DeviationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.ClientMsg, e.Cause)
	}
	return e.ClientMsg
}

func (e *DeviationError) Unwrap() error { return e.Cause }

// The HTTP statuses a DeviationError carries. The public contract names the
// numbers, not the router package.
const (
	deviationStatusBadRequest = 400
	deviationStatusNotFound   = 404
	deviationStatusConflict   = 409
	deviationStatusInternal   = 500
)

// deviationSaveErrorMessage is the client message of every internal failure.
const deviationSaveErrorMessage = "Die Änderungen konnten nicht gespeichert werden. Versuchen Sie es erneut."

// DeviationBadRequest is a 400 with the given client message.
func DeviationBadRequest(msg string) *DeviationError {
	return &DeviationError{Status: deviationStatusBadRequest, ClientMsg: msg}
}

// DeviationNotFound is a 404 with the given client message.
func DeviationNotFound(msg string) *DeviationError {
	return &DeviationError{Status: deviationStatusNotFound, ClientMsg: msg}
}

// DeviationConflict is a 409 with a stable code and a client message.
func DeviationConflict(code, msg string) *DeviationError {
	return &DeviationError{Status: deviationStatusConflict, Code: code, ClientMsg: msg}
}

// DeviationInternal is a 500 that keeps the operation and cause for logs.
func DeviationInternal(operation string, cause error) *DeviationError {
	return &DeviationError{
		Status:    deviationStatusInternal,
		ClientMsg: deviationSaveErrorMessage,
		Cause:     fmt.Errorf("%s: %w", operation, cause),
	}
}

// DeviationInternalDetail is a 500 whose cause is a plain detail.
func DeviationInternalDetail(detail string) *DeviationError {
	return &DeviationError{
		Status:    deviationStatusInternal,
		ClientMsg: deviationSaveErrorMessage,
		Cause:     errors.New(detail),
	}
}

// GuardianNoticeInput is the text the person cancelling a block wrote for
// the families (#2601). It is separate from the internal cancel reason on
// purpose: the reason may name a colleague's illness, the notice may not.
type GuardianNoticeInput struct {
	Title   string
	Message string
}

// GuardianNoticeResult reports what a cancellation notice reached.
type GuardianNoticeResult struct {
	AnnouncementID int64
	ChildCount     int
	FamilyCount    int
}

// DeviationAbsenceInput marks one staff member absent. Nil InstanceIDs means
// every plannable same-day appointment; a non-nil list targets exact ones.
type DeviationAbsenceInput struct {
	StaffID     int64
	Reason      *string
	InstanceIDs *[]int64
}

// DeviationSubstitutionInput assigns SubstituteStaffID to cover AbsentStaffID.
// Nil InstanceIDs means every plannable same-day appointment; a non-nil list
// targets exactly those appointments.
type DeviationSubstitutionInput struct {
	AbsentStaffID     int64
	SubstituteStaffID int64
	Reason            *string
	InstanceIDs       *[]int64
}

// DeviationPresenceInput clears an absence. Nil InstanceIDs means every
// plannable same-day appointment; a non-nil list targets exact ones.
type DeviationPresenceInput struct {
	StaffID     int64
	InstanceIDs *[]int64
}

// DeviationSubstitutionRemovalInput removes a substitute assignment without
// changing that person's other appointments. Nil InstanceIDs means every
// plannable same-day appointment; a non-nil list targets exact ones.
type DeviationSubstitutionRemovalInput struct {
	StaffID     int64
	InstanceIDs *[]int64
}

// ApplyDeviationsInput is one parsed Vertretungsplan save. All mutation
// fields are optional; a Cancel request is exclusive. UnderstaffedAck is a
// pointer so an omitted field ("no change") is distinguishable from an
// explicit false ("clear").
type ApplyDeviationsInput struct {
	Cancel               bool
	CancelReason         *string
	UnderstaffedAck      *bool
	UnderstaffedNote     *string
	Absences             []DeviationAbsenceInput
	Substitutions        []DeviationSubstitutionInput
	SubstitutionRemovals []DeviationSubstitutionRemovalInput
	Presences            []DeviationPresenceInput
	ActorAccountID       *int64
	// GuardianNotice informs the families when Cancel is set (#2601). Ignored
	// on a non-cancel save.
	GuardianNotice *GuardianNoticeInput
}

// DeviationAffected is one classified target a save touched.
type DeviationAffected struct {
	InstanceID int64
	Title      string
	StartTime  time.Time
	Action     string
}

// TouchedActivity is the slot identity of a running block a staffing save
// touched; the activity update after commit carries only this refetch hint.
type TouchedActivity struct {
	InstanceID int64
	Date       calendar.Date
	StartTime  time.Time
}

// TouchedActivities maps the active session id of each touched running block
// to its slot identity.
type TouchedActivities map[int64]TouchedActivity

// ApplyDeviationsResult is the outcome of one Vertretungsplan save.
// ActiveTouched plus the counts drive the SSE broadcast and the log line.
type ApplyDeviationsResult struct {
	InstanceID      int64
	Cancelled       bool
	UnderstaffedAck bool
	Affected        []DeviationAffected
	Warnings        []SubstituteTimeConflict
	ActiveTouched   TouchedActivities
	AppliedWrites   int
	// GuardianNotice is the notice outcome of a cancel save, nil otherwise.
	GuardianNotice           *GuardianNoticeResult
	AckChanged               bool
	ClearedAcks              int
	AbsenceCount             int
	PresenceCount            int
	SubstitutionCount        int
	SubstitutionRemovalCount int
	Message                  string
}

// BulkSubstitutionInput is one Sammel-Vertretung (#2284). A nil
// SubstituteStaffID marks the person absent on every selected day without
// assigning cover.
type BulkSubstitutionInput struct {
	AbsentStaffID     int64
	SubstituteStaffID *int64
	Dates             []calendar.Date
	Reason            *string
	ActorAccountID    *int64
}

// BulkSubstitutionDay is the per-day slice of the result: what the save
// touched on that date plus the substitute's time-overlap advisories.
type BulkSubstitutionDay struct {
	Date     calendar.Date
	Affected []DeviationAffected
	Warnings []SubstituteTimeConflict
}

// BulkSubstitutionResult is the outcome of one Sammel-Vertretung.
type BulkSubstitutionResult struct {
	Days          []BulkSubstitutionDay
	ActiveTouched TouchedActivities
	AppliedWrites int
	ClearedAcks   int
}

// SickAbsenceMark stamps one staff assignment absent for a sick report
// (#1843). The assignment is the instance_staff row id.
type SickAbsenceMark struct {
	AssignmentID   int64
	Reason         *string
	SickAbsenceID  int64
	ActorAccountID *int64
}

// SickAbsenceClear releases one assignment a deleted sick report had stamped.
type SickAbsenceClear struct {
	AssignmentID   int64
	SickAbsenceID  int64
	ActorAccountID *int64
}

// SubstituteConflictProbe asks for the time-overlap advisories of one staff
// member covering Targets on Date. TargetIDs are every block the save placed
// the person on, including already-covered ones the advisory must not flag.
type SubstituteConflictProbe struct {
	StaffID   int64
	Date      calendar.Date
	Targets   []SubstituteConflictInstance
	TargetIDs []int64
}

// SubstituteConflictQuery reads the substitute time-overlap advisories.
type SubstituteConflictQuery interface {
	DetectSubstituteConflicts(ctx context.Context, probe SubstituteConflictProbe) ([]SubstituteTimeConflict, error)
}

// StaffDeviations is the Timetable owner's deviation command surface.
type StaffDeviations interface {
	SubstituteConflictQuery
	// ApplyDeviations applies a whole Vertretungsplan save atomically:
	// day-lock, validate and classify every change, then write. Validation
	// and conflict failures return a *DeviationError; a cancel save returns
	// the lifecycle's errors unchanged.
	ApplyDeviations(ctx context.Context, instanceID int64, in ApplyDeviationsInput) (*ApplyDeviationsResult, error)
	// ApplyBulkSubstitution applies one person's day-wide absence, optionally
	// covered by one substitute, to several dates in a single atomic save.
	ApplyBulkSubstitution(ctx context.Context, in BulkSubstitutionInput) (*BulkSubstitutionResult, error)
	// MarkSickAbsence marks one assignment absent for a sick report and
	// records touched running blocks in touched.
	MarkSickAbsence(ctx context.Context, in SickAbsenceMark, touched TouchedActivities) error
	// ClearSickAbsence restores one assignment a deleted sick report had
	// stamped, then clears the block's "deliberately unstaffed"
	// acknowledgement when the block is fully staffed again.
	ClearSickAbsence(ctx context.Context, in SickAbsenceClear, touched TouchedActivities) error
	// QueueActivityUpdates announces every touched running block after the
	// surrounding tenant transaction commits. A rollback announces nothing.
	QueueActivityUpdates(ctx context.Context, touched TouchedActivities)
}
