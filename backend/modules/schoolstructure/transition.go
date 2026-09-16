package schoolstructure

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Grade transitions are School Structure's record of a school-year rollover:
// the draft (mappings from one class name to the next, or to a graduation),
// the per-child history the apply writes, and the two ledgers that let a
// revert replay what the apply did to class-teacher assignments and
// class-list entries. The owner persists and validates these rows; the
// lifecycle (locking, cohort resolution, the apply and the revert) is the
// grade-transition workflow's (#2711).
const (
	TransitionStatusDraft    = "draft"
	TransitionStatusApplied  = "applied"
	TransitionStatusReverted = "reverted"

	TransitionActionPromoted  = "promoted"
	TransitionActionGraduated = "graduated"

	// LedgerActionRemoved marks a row the apply deleted, preserving its
	// original display form for the revert; LedgerActionCreated marks a row
	// the apply inserted, which the revert deletes again if it still exists.
	LedgerActionRemoved = "removed"
	LedgerActionCreated = "created"
)

var (
	ErrTransitionNotFound = errors.New("grade transition not found")
	// ErrTransitionStateConflict reports a status guard that matched no
	// row: the transition left the status the command expected.
	ErrTransitionStateConflict = errors.New("grade transition is not in the expected status")
	ErrInvalidTransition       = errors.New("invalid grade transition")
)

// InvalidTransitionError carries the validation reason; it unwraps to
// ErrInvalidTransition so callers classify it with errors.Is.
type InvalidTransitionError struct{ Reason string }

func (e *InvalidTransitionError) Error() string { return e.Reason }
func (e *InvalidTransitionError) Unwrap() error { return ErrInvalidTransition }

type Transition struct {
	ID                       int64      `json:"id"`
	TenantID                 int64      `json:"tenant_id"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
	AcademicYear             string     `json:"academic_year"`
	Status                   string     `json:"status"`
	AppliedAt                *time.Time `json:"applied_at,omitempty"`
	AppliedBy                *int64     `json:"applied_by,omitempty"`
	RevertedAt               *time.Time `json:"reverted_at,omitempty"`
	RevertedBy               *int64     `json:"reverted_by,omitempty"`
	CreatedBy                int64      `json:"created_by"`
	Notes                    *string    `json:"notes,omitempty"`
	RosterBaselineInstanceID *int64     `json:"roster_baseline_instance_id,omitempty"`
	Mappings                 []TransitionMapping
}

func (t Transition) IsDraft() bool    { return t.Status == TransitionStatusDraft }
func (t Transition) IsApplied() bool  { return t.Status == TransitionStatusApplied }
func (t Transition) IsReverted() bool { return t.Status == TransitionStatusReverted }

// CanApply reports whether the draft carries at least one mapping.
func (t Transition) CanApply() bool { return t.IsDraft() && len(t.Mappings) > 0 }

type TransitionMapping struct {
	ID           int64   `json:"id"`
	TenantID     int64   `json:"tenant_id"`
	TransitionID int64   `json:"transition_id"`
	FromClass    string  `json:"from_class"`
	ToClass      *string `json:"to_class,omitempty"`
}

// IsGraduating reports a mapping without a target class.
func (m TransitionMapping) IsGraduating() bool { return m.ToClass == nil }

// TransitionMappingInput is one class rename of a draft; a nil or blank
// target graduates the class.
type TransitionMappingInput struct {
	FromClass string
	ToClass   *string
}

type TransitionDraft struct {
	AcademicYear string
	Notes        *string
	CreatedBy    int64
	Mappings     []TransitionMappingInput
}

// TransitionUpdate patches a draft. A nil Mappings slice keeps the stored
// mappings; an empty one clears them.
type TransitionUpdate struct {
	ID           int64
	AcademicYear *string
	Notes        *string
	Mappings     []TransitionMappingInput
}

// TransitionFilter narrows a listing. A non-nil AfterID (including 0) switches
// to an ascending id window starting after that cursor; otherwise the newest
// transition comes first. The admin list client always starts with after_id=0.
type TransitionFilter struct {
	Status       string
	AcademicYear string
	AfterID      *int64
	Limit        int
	Offset       int
}

// TransitionHistoryEntry is one child's row in the apply's history. The
// person name is a snapshot for the audit trail; FromStatus lets the revert
// restore the exact lifecycle status; RFIDTag is the bracelet graduation
// released, so the revert can hand it back.
type TransitionHistoryEntry struct {
	ID           int64     `json:"id"`
	TenantID     int64     `json:"tenant_id"`
	CreatedAt    time.Time `json:"created_at"`
	TransitionID int64     `json:"transition_id"`
	StudentID    int64     `json:"student_id"`
	PersonName   string    `json:"person_name"`
	FromClass    string    `json:"from_class"`
	ToClass      *string   `json:"to_class,omitempty"`
	Action       string    `json:"action"`
	FromStatus   *string   `json:"from_status,omitempty"`
	RFIDTag      *string   `json:"rfid_tag,omitempty"`
}

func (e TransitionHistoryEntry) WasPromoted() bool  { return e.Action == TransitionActionPromoted }
func (e TransitionHistoryEntry) WasGraduated() bool { return e.Action == TransitionActionGraduated }

type TransitionClassTeacherEntry struct {
	ID           int64  `json:"id"`
	TransitionID int64  `json:"transition_id"`
	StaffID      int64  `json:"staff_id"`
	SchoolClass  string `json:"school_class"`
	Action       string `json:"action"`
}

// TransitionClassListEntry records what the apply did to one class-list
// entry. EntryID is the row the apply inserted (created rows only).
type TransitionClassListEntry struct {
	ID           int64  `json:"id"`
	TransitionID int64  `json:"transition_id"`
	EntryID      *int64 `json:"entry_id,omitempty"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	SchoolClass  string `json:"school_class"`
	Action       string `json:"action"`
}

// TransitionQuery reads the transition drafts and ledgers of the tenant in
// context.
type TransitionQuery interface {
	// FindTransition returns the transition with its mappings.
	FindTransition(context.Context, int64) (Transition, error)
	// ListTransitions returns a page of transitions with their mappings and
	// the total count matching the filter.
	ListTransitions(context.Context, TransitionFilter) ([]Transition, int, error)
	ListTransitionHistory(context.Context, int64) ([]TransitionHistoryEntry, error)
	ListTransitionClassTeacherLedger(context.Context, int64) ([]TransitionClassTeacherEntry, error)
	ListTransitionClassListLedger(context.Context, int64) ([]TransitionClassListEntry, error)
}

// TransitionCommand changes the transition rows inside the caller's tenant
// transaction. Status guards refuse a row that is not in the expected status
// with ErrTransitionStateConflict: UpdateTransition and DeleteTransition act
// on drafts only (the delete cascades the history and ledgers a revert
// needs), MarkTransitionApplied on a draft, MarkTransitionReverted on an
// applied transition.
type TransitionCommand interface {
	CreateTransition(context.Context, TransitionDraft) (Transition, error)
	UpdateTransition(context.Context, TransitionUpdate) (Transition, error)
	DeleteTransition(context.Context, int64) error
	// LockTransitions takes the tenant-wide transaction-scoped transition
	// gate shared with the timetable materializer. It requires a tenant
	// transaction; the lock releases at COMMIT/ROLLBACK.
	LockTransitions(context.Context) error
	// LockTransitionForMutation reads the transition with its mappings and
	// holds the row FOR UPDATE for the caller's transaction.
	LockTransitionForMutation(context.Context, int64) (Transition, error)
	// LockLatestAppliedTransition returns the most recently applied
	// transition FOR UPDATE; found is false when none is applied.
	LockLatestAppliedTransition(context.Context) (Transition, bool, error)
	MarkTransitionApplied(ctx context.Context, id, accountID int64, at time.Time, rosterBaselineInstanceID *int64) error
	MarkTransitionReverted(ctx context.Context, id, accountID int64, at time.Time) error
	AppendTransitionHistory(context.Context, []TransitionHistoryEntry) error
	AppendTransitionClassTeacherLedger(context.Context, []TransitionClassTeacherEntry) error
	AppendTransitionClassListLedger(context.Context, []TransitionClassListEntry) error
}

// TransitionCapability is the School Structure half of a grade transition.
type TransitionCapability interface {
	TransitionQuery
	TransitionCommand
}

type transitionEngine interface {
	FindTransition(context.Context, int64, string) (Transition, error)
	ListTransitions(context.Context, TransitionFilter) ([]Transition, int, error)
	ListTransitionHistory(context.Context, int64) ([]TransitionHistoryEntry, error)
	ListTransitionClassTeacherLedger(context.Context, int64) ([]TransitionClassTeacherEntry, error)
	ListTransitionClassListLedger(context.Context, int64) ([]TransitionClassListEntry, error)
	CreateTransition(context.Context, TransitionDraft) (Transition, error)
	UpdateTransition(context.Context, TransitionUpdate) (Transition, error)
	DeleteTransition(context.Context, int64) error
	LockTransitions(context.Context) error
	LockLatestAppliedTransition(context.Context) (Transition, bool, error)
	MarkTransitionApplied(ctx context.Context, id, accountID int64, at time.Time, rosterBaselineInstanceID *int64) error
	MarkTransitionReverted(ctx context.Context, id, accountID int64, at time.Time) error
	AppendTransitionHistory(context.Context, []TransitionHistoryEntry) error
	AppendTransitionClassTeacherLedger(context.Context, []TransitionClassTeacherEntry) error
	AppendTransitionClassListLedger(context.Context, []TransitionClassListEntry) error
}

func invalidTransition(reason string) error { return &InvalidTransitionError{Reason: reason} }

func (m *Module) FindTransition(ctx context.Context, id int64) (Transition, error) {
	if id <= 0 {
		return Transition{}, invalidTransition("transition ID is required")
	}
	return m.engine.FindTransition(ctx, id, "")
}

func (m *Module) ListTransitions(ctx context.Context, filter TransitionFilter) ([]Transition, int, error) {
	if filter.Limit < 0 || filter.Offset < 0 || (filter.AfterID != nil && *filter.AfterID < 0) {
		return nil, 0, invalidTransition("invalid transition listing window")
	}
	filter.Status = strings.TrimSpace(filter.Status)
	filter.AcademicYear = strings.TrimSpace(filter.AcademicYear)
	return m.engine.ListTransitions(ctx, filter)
}

func (m *Module) ListTransitionHistory(ctx context.Context, transitionID int64) ([]TransitionHistoryEntry, error) {
	if transitionID <= 0 {
		return nil, invalidTransition("transition ID is required")
	}
	return m.engine.ListTransitionHistory(ctx, transitionID)
}

func (m *Module) ListTransitionClassTeacherLedger(ctx context.Context, transitionID int64) ([]TransitionClassTeacherEntry, error) {
	if transitionID <= 0 {
		return nil, invalidTransition("transition ID is required")
	}
	return m.engine.ListTransitionClassTeacherLedger(ctx, transitionID)
}

func (m *Module) ListTransitionClassListLedger(ctx context.Context, transitionID int64) ([]TransitionClassListEntry, error) {
	if transitionID <= 0 {
		return nil, invalidTransition("transition ID is required")
	}
	return m.engine.ListTransitionClassListLedger(ctx, transitionID)
}

func (m *Module) CreateTransition(ctx context.Context, draft TransitionDraft) (Transition, error) {
	if draft.CreatedBy <= 0 {
		return Transition{}, invalidTransition("created_by is required")
	}
	return m.engine.CreateTransition(ctx, draft)
}

func (m *Module) UpdateTransition(ctx context.Context, update TransitionUpdate) (Transition, error) {
	if update.ID <= 0 {
		return Transition{}, invalidTransition("transition ID is required")
	}
	return m.engine.UpdateTransition(ctx, update)
}

func (m *Module) DeleteTransition(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidTransition("transition ID is required")
	}
	return m.engine.DeleteTransition(ctx, id)
}

func (m *Module) LockTransitions(ctx context.Context) error {
	return m.engine.LockTransitions(ctx)
}

func (m *Module) LockTransitionForMutation(ctx context.Context, id int64) (Transition, error) {
	if id <= 0 {
		return Transition{}, invalidTransition("transition ID is required")
	}
	return m.engine.FindTransition(ctx, id, "update")
}

func (m *Module) LockLatestAppliedTransition(ctx context.Context) (Transition, bool, error) {
	return m.engine.LockLatestAppliedTransition(ctx)
}

func (m *Module) MarkTransitionApplied(ctx context.Context, id, accountID int64, at time.Time, rosterBaselineInstanceID *int64) error {
	if id <= 0 || accountID <= 0 || at.IsZero() {
		return invalidTransition("transition ID, account and instant are required")
	}
	return m.engine.MarkTransitionApplied(ctx, id, accountID, at, rosterBaselineInstanceID)
}

func (m *Module) MarkTransitionReverted(ctx context.Context, id, accountID int64, at time.Time) error {
	if id <= 0 || accountID <= 0 || at.IsZero() {
		return invalidTransition("transition ID, account and instant are required")
	}
	return m.engine.MarkTransitionReverted(ctx, id, accountID, at)
}

func (m *Module) AppendTransitionHistory(ctx context.Context, entries []TransitionHistoryEntry) error {
	if len(entries) == 0 {
		return nil
	}
	return m.engine.AppendTransitionHistory(ctx, entries)
}

func (m *Module) AppendTransitionClassTeacherLedger(ctx context.Context, entries []TransitionClassTeacherEntry) error {
	if len(entries) == 0 {
		return nil
	}
	return m.engine.AppendTransitionClassTeacherLedger(ctx, entries)
}

func (m *Module) AppendTransitionClassListLedger(ctx context.Context, entries []TransitionClassListEntry) error {
	if len(entries) == 0 {
		return nil
	}
	return m.engine.AppendTransitionClassListLedger(ctx, entries)
}
