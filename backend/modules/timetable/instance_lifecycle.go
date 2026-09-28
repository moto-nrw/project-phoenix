package timetable

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The instance lifecycle (WP-B9, #3424 slice S1) drives the state machine of
// one block:
//
//	          start                 complete
//	planned  ─────────►  active  ─────────►  completed
//	   │                   │
//	   │ cancel            │ cancel
//	   ▼                   ▼
//	cancelled           cancelled
//
// Every illegal transition returns ErrInvalidInstanceTransition. Start opens
// the Student Presence session of the block in the caller's tenant
// transaction; Complete and Cancel end it. Every write serializes with the
// day-wide staffing mutations through SubstituteDayLockKey.

// Sentinel errors the lifecycle callers branch on with errors.Is.
var (
	// ErrInstanceNotFound: the id does not resolve in the current tenant.
	ErrInstanceNotFound = errors.New("activity instance not found")
	// ErrInvalidInstanceTransition: the action is not legal from the block's
	// current status; the message carries that status.
	ErrInvalidInstanceTransition = errors.New("invalid instance transition")
	// ErrInvalidInstanceReference: a supplied foreign id does not resolve in
	// the current tenant.
	ErrInvalidInstanceReference = errors.New("invalid instance reference")
	// ErrInstanceWeekend: an update introduces or restores a weekend date.
	// Existing legacy weekend rows may keep their date.
	ErrInstanceWeekend = errors.New("timetable entries can only be scheduled from Monday to Friday")
	// ErrInstanceOutsideActiveCalendarPeriod: a planned block would lie
	// outside every active calendar period.
	ErrInstanceOutsideActiveCalendarPeriod = errors.New("instance date must lie within an active calendar period")
	// ErrAmbiguousTemplateInstanceDelete: deleting one occurrence would need
	// a date-wide cancellation exception while the template has several
	// slots on that date.
	ErrAmbiguousTemplateInstanceDelete = errors.New("template instance delete is ambiguous")
	// ErrUnderstaffedAckStillStaffed: a fully staffed block cannot be marked
	// "deliberately unstaffed".
	ErrUnderstaffedAckStillStaffed = errors.New("cannot acknowledge understaffing while the block is fully staffed")
	// ErrInstanceMoved: a concurrent edit moved the block to another day
	// while the transition waited on the day lock (#1840).
	ErrInstanceMoved = errors.New("instance was moved concurrently")

	ErrInstanceStartTooEarly       = errors.New("activity instance cannot be started yet")
	ErrInstanceStartExpired        = errors.New("activity instance can no longer be started")
	ErrInstanceCompleteEarly       = errors.New("activity instance cannot be completed before planned end")
	ErrCompletionConfirmationStale = errors.New("activity completion confirmation is stale")
	ErrIdempotencyKeyReuse         = errors.New("idempotency key was reused with different request data")

	// ErrInstanceAlreadyInSeries: the occurrence to convert already belongs
	// to a template.
	ErrInstanceAlreadyInSeries = errors.New("activity instance already belongs to a series")

	// ErrGuardianNoticeInvalid marks a cancellation notice refused before
	// anything is written: empty text, no actor, or a block in the past
	// (#2601).
	ErrGuardianNoticeInvalid = errors.New("guardian notice: invalid request")
	// ErrGuardianNoticeDisabled surfaces the school-wide switch.
	ErrGuardianNoticeDisabled = errors.New("guardian notice: disabled for this school")
)

// LifecycleInstance is one block as a lifecycle write leaves it.
type LifecycleInstance struct {
	ID               int64
	Date             calendar.Date
	StartTime        time.Time
	EndTime          time.Time
	Title            string
	RoomID           int64
	ActivityGroupID  *int64
	Status           string
	IsSpontaneous    bool
	UnderstaffedAck  bool
	UnderstaffedNote *string
	ActiveGroupID    *int64
	StartedAt        *time.Time
	CompletedAt      *time.Time
	ReopenUntil      *time.Time
}

// CreateInstanceInput inserts one block outside the materialization.
//
// IsSpontaneous records the creation origin (#2299): nil defaults to a
// planned block the lifecycle time guards apply to; ad-hoc start flows pass
// an explicit true and may still link an offering through ActivityGroupID.
type CreateInstanceInput struct {
	Date             calendar.Date
	StartTime        time.Time // 2000-01-01 HH:MM in UTC
	EndTime          time.Time // 2000-01-01 HH:MM in UTC
	Title            string
	Description      *string
	Notes            *string
	RoomID           int64
	ActivityGroupID  *int64
	ListKind         *string
	IsSpontaneous    *bool
	StaffIDs         []int64
	StudentIDs       []int64
	CreatedByStaffID *int64
	IdempotencyKey   *string
	// RequiredStaff is the optional manual Personalbedarf override (#1839);
	// nil derives it from the Betreuungsschlüssel.
	RequiredStaff *int
}

// UpdateInstanceInput replaces the planning fields and the roster of a
// planned block.
type UpdateInstanceInput struct {
	Date            calendar.Date
	StartTime       time.Time
	EndTime         time.Time
	Title           string
	Description     *string
	Notes           *string
	RoomID          int64
	ActivityGroupID *int64
	ListKind        *string
	StaffIDs        []int64
	StudentIDs      []int64
	// RequiredStaff is the optional manual Personalbedarf override (#1839).
	RequiredStaff *int
	// CalendarPeriodID, when set, stamps the materializer marker (template,
	// period, not spontaneous). Only the series conversion sets it; an
	// ordinary edit leaves hand-linked one-offs outside the roster resync.
	CalendarPeriodID *int64
}

// StartInstanceResult is a started (or reopened) block, its live session
// and the advisory start conflicts.
type StartInstanceResult struct {
	Instance      *LifecycleInstance
	ActiveGroupID int64
	Warnings      []InstanceConflictWarning
}

// ReplanWeekResult is the delete count of a re-plan plus the
// materialization that followed it.
type ReplanWeekResult struct {
	From             calendar.Date
	To               calendar.Date
	DeletedInstances int
	Materialization  *MaterializationResult
}

// GuardianNoticeReach is the preview the cancel dialog shows before sending.
type GuardianNoticeReach struct {
	Enabled     bool
	DefaultOn   bool
	ChildCount  int
	FamilyCount int
}

// CancelInstanceInput is a cancellation with its optional guardian notice.
// A nil GuardianNotice cancels silently.
type CancelInstanceInput struct {
	InstanceID     int64
	Reason         *string
	ActorAccountID *int64
	GuardianNotice *GuardianNoticeInput
}

// CancelInstanceResult is the cancelled block plus the notice outcome (nil
// when none was requested).
type CancelInstanceResult struct {
	Instance       *LifecycleInstance
	GuardianNotice *GuardianNoticeResult
}

// Staff move actions (#1884).
const (
	MoveStaffActionMoved          = "moved"
	MoveStaffActionAssigned       = "assigned"
	MoveStaffActionAlreadyApplied = "already_applied"
)

// MoveStaffInput moves one staff member onto a target block. A nil
// SourceInstanceID assigns a person from the pool instead of relocating an
// existing assignment.
type MoveStaffInput struct {
	StaffID          int64
	SourceInstanceID *int64
	ActorAccountID   *int64
}

// MoveStaffResult is a successful staff move. Warnings are the moved
// person's remaining same-day overlaps with the target (advisory);
// ActiveTouched names the running blocks whose sessions changed.
type MoveStaffResult struct {
	Target        *LifecycleInstance
	Source        *LifecycleInstance
	Action        string
	Warnings      []SubstituteTimeConflict
	ActiveTouched TouchedActivities
}

// ConvertInstanceToSeriesInput turns one planned occurrence into the seed of
// a new recurring template. Template is the complete series definition;
// InstanceNotes stays on the seed.
type ConvertInstanceToSeriesInput struct {
	InstanceID     int64
	Template       CreateTemplateCommand
	InstanceNotes  *string
	ActorAccountID *int64
}

// ConvertInstanceToSeriesResult identifies both sides of a conversion;
// LinkedInstanceID is always the pre-existing occurrence.
type ConvertInstanceToSeriesResult struct {
	TemplateID       int64
	TimeframeID      int64
	ScheduleIDs      []int64
	LinkedInstanceID int64
}

// AutoStartResult summarizes one auto-start tick of one tenant.
type AutoStartResult struct {
	Checked             int
	Started             int
	SkippedBeforeWindow int
	SkippedAfterWindow  int
	SkippedNoStaff      int
	SkippedConflict     int
	SkippedMoved        int
	SkippedNonPlanned   int
	Failed              int
	DurationMS          int64
}

// AutoEndResult summarizes one auto-end tick of one tenant.
type AutoEndResult struct {
	Checked               int
	Completed             int
	SkippedBeforeDeadline int
	SkippedSpontaneous    int
	SkippedNonActive      int
	SkippedConcurrent     int
	Failed                int
	DurationMS            int64
}

// InstanceLifecycle drives the transitions of one block. Every method runs
// in the caller's tenant transaction or opens one.
type InstanceLifecycle interface {
	Start(ctx context.Context, instanceID, startedByStaffID int64) (*StartInstanceResult, error)
	Complete(ctx context.Context, instanceID int64) (*LifecycleInstance, error)
	// Reopen restores the live state captured at completion within the
	// reopen window, for an admin or the account that completed the block.
	Reopen(ctx context.Context, instanceID, accountID int64, isAdmin bool) (*StartInstanceResult, error)
	// Cancel transitions planned|active → cancelled. reason is the optional
	// short "why" (#1840); actorAccountID stamps the Änderungsprotokoll.
	Cancel(ctx context.Context, instanceID int64, reason *string, actorAccountID *int64) (*LifecycleInstance, error)
	// CancelWithNotice cancels like Cancel and, when asked, informs the
	// booked children's families in the same transaction (#2601).
	CancelWithNotice(ctx context.Context, in CancelInstanceInput) (*CancelInstanceResult, error)
	// GuardianNoticeReachFor previews how many children and families a
	// cancellation notice for the block would reach.
	GuardianNoticeReachFor(ctx context.Context, instanceID int64) (*GuardianNoticeReach, error)
	// DeleteCancelled removes a planned or cancelled block; a materialized
	// occurrence leaves a cancellation exception behind.
	DeleteCancelled(ctx context.Context, instanceID int64) error
	// BulkCancelPlanned cancels and removes the planned occurrences of
	// [from, to] from today on (#3594); opts.DryRun only counts them.
	BulkCancelPlanned(ctx context.Context, from, to calendar.Date, opts BulkCancelOptions, actorAccountID *int64) (*BulkCancelResult, error)
}

// InstancePlanning creates, edits and re-plans single blocks.
type InstancePlanning interface {
	CreateInstance(ctx context.Context, in CreateInstanceInput) (*LifecycleInstance, error)
	UpdatePlanned(ctx context.Context, instanceID int64, in UpdateInstanceInput, actorAccountID *int64) (*LifecycleInstance, error)
	// ReplanWeek deletes the planned template-backed blocks of [from, to],
	// re-materializes the window and reapplies the Vertretungsplan
	// overrides. A non-nil activityGroupID narrows the delete to one
	// template.
	ReplanWeek(ctx context.Context, from, to calendar.Date, activityGroupID, actorAccountID *int64) (*ReplanWeekResult, error)
	// GetPlannedStudentIDsByDate returns which of the children have a
	// planned block on the date (#584).
	GetPlannedStudentIDsByDate(ctx context.Context, studentIDs []int64, date calendar.Date) ([]int64, error)
}

// InstanceStaffing is the lifecycle's staffing writes: the "deliberately
// unstaffed" acknowledgement (#1840) and the staff move (#1884).
type InstanceStaffing interface {
	// SetUnderstaffedAck flips the acknowledgement on a planned or active
	// block; a fully staffed block refuses it.
	SetUnderstaffedAck(ctx context.Context, instanceID int64, ack bool, note *string, actorAccountID *int64) (*LifecycleInstance, error)
	// ClearUnderstaffedAckIfStaffed clears a lingering acknowledgement only
	// when the block is fully staffed now.
	ClearUnderstaffedAckIfStaffed(ctx context.Context, instanceID int64, actorAccountID *int64) error
	// AcknowledgeUnderstaffed is the standalone acknowledgement: it gates
	// past blocks and serializes with same-day staffing saves. The note
	// arrives trimmed and validated.
	AcknowledgeUnderstaffed(ctx context.Context, instanceID int64, ack bool, note *string, actorAccountID *int64) (*LifecycleInstance, error)
	// MoveStaffBetweenBlocks moves (or pool-assigns) one staff member onto
	// the target block atomically, with one staff_moved protocol entry.
	MoveStaffBetweenBlocks(ctx context.Context, targetID int64, in MoveStaffInput) (*MoveStaffResult, error)
	// QueueActivityUpdates announces the touched running blocks after the
	// surrounding tenant transaction commits.
	QueueActivityUpdates(ctx context.Context, touched TouchedActivities)
}

// InstanceLifecycleCapability is the whole lifecycle the composition root
// hands out.
type InstanceLifecycleCapability interface {
	InstanceLifecycle
	InstancePlanning
	InstanceStaffing
}

// InstanceSeriesConversion changes a one-off occurrence into a recurring
// series in one tenant transaction.
type InstanceSeriesConversion interface {
	ConvertInstanceToSeries(ctx context.Context, in ConvertInstanceToSeriesInput) (*ConvertInstanceToSeriesResult, error)
}

// InstanceAutoStart starts the due planned blocks of a tenant that opted
// into timetable.auto_start_planned.
type InstanceAutoStart interface {
	RunForTenant(ctx context.Context, now time.Time) (*AutoStartResult, error)
}

// InstanceAutoEnd completes the due running blocks of a tenant through the
// same lifecycle the manual completion uses.
type InstanceAutoEnd interface {
	RunForTenant(ctx context.Context, now time.Time, grace time.Duration) (*AutoEndResult, error)
}

type lifecycleContextKey int

const (
	lifecycleActorKey lifecycleContextKey = iota
	lifecycleConfirmedStudentsKey
	lifecycleSpontaneousWorkdayGuardKey
)

// WithLifecycleActor names the account a completion is attributed to.
func WithLifecycleActor(ctx context.Context, accountID int64) context.Context {
	return context.WithValue(ctx, lifecycleActorKey, accountID)
}

// LifecycleActor is the account WithLifecycleActor named, zero without one.
func LifecycleActor(ctx context.Context) int64 {
	accountID, _ := ctx.Value(lifecycleActorKey).(int64)
	return accountID
}

// WithCompletionConfirmation requires a completion to see exactly these
// children still checked in; otherwise it fails with
// ErrCompletionConfirmationStale.
func WithCompletionConfirmation(ctx context.Context, studentIDs []int64) context.Context {
	return context.WithValue(ctx, lifecycleConfirmedStudentsKey, slices.Clone(studentIDs))
}

// CompletionConfirmation is the confirmed roster of
// WithCompletionConfirmation; required is false without one.
func CompletionConfirmation(ctx context.Context) (studentIDs []int64, required bool) {
	studentIDs, required = ctx.Value(lifecycleConfirmedStudentsKey).([]int64)
	return studentIDs, required
}

// WithSpontaneousStartWorkdayGuard makes the start of a spontaneous block
// refuse a weekend, as the ad-hoc start flows require.
func WithSpontaneousStartWorkdayGuard(ctx context.Context) context.Context {
	return context.WithValue(ctx, lifecycleSpontaneousWorkdayGuardKey, struct{}{})
}

// SpontaneousStartWorkdayGuarded reports WithSpontaneousStartWorkdayGuard.
func SpontaneousStartWorkdayGuarded(ctx context.Context) bool {
	_, ok := ctx.Value(lifecycleSpontaneousWorkdayGuardKey).(struct{})
	return ok
}
