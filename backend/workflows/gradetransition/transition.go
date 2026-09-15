// Package gradetransition coordinates the school-year rollover across owner
// capabilities (#2711): the draft and its ledgers belong to School
// Structure, the children and their bracelets to People Directory, the
// class-teacher assignments and class-list entries to School Membership, the
// materialized rosters to Timetable & Activities, and the check-in guard to
// Student Presence. The workflow owns no table and contains no SQL: it
// authorizes, locks in the blocker-defined order, resolves the cohort under
// those locks, compares it with the confirmed preview, and runs every owner
// command inside one tenant UnitOfWork. Nothing it does is irreversible;
// graduation is a soft delete and every rewrite is ledgered for the revert.
package gradetransition

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// Operations the workflow authorizes. The composition maps each one to the
// tenant permission the HTTP surface already enforces.
const (
	OperationRead   = "read"
	OperationCreate = "create"
	OperationUpdate = "update"
	OperationDelete = "delete"
	OperationApply  = "apply"
)

// Sentinel errors. The texts are part of the HTTP contract: the adapter maps
// the sentinels to status codes and codes and renders the wrapped detail.
var (
	ErrUnauthorized       = errors.New("grade transitions require an authorized tenant principal")
	ErrTransitionNotFound = errors.New("grade transition not found")
	// ErrInvalidTransitionData wraps client-correctable validation failures
	// in a submitted transition or its mappings.
	ErrInvalidTransitionData = errors.New("invalid transition data")
	// ErrTransitionNotDraft is returned when the targeted transition is no
	// longer a draft: another admin applied or reverted it in between.
	ErrTransitionNotDraft = errors.New("must be in draft status")
	// ErrTransitionNotApplied is returned by Revert when the transition is
	// still a draft or was already reverted.
	ErrTransitionNotApplied = errors.New("must be in applied status")
	// ErrNotLatestApplied is returned by Revert when the caller targets an
	// applied transition that is not the most recently applied one: reverts
	// unwind in strict reverse order.
	ErrNotLatestApplied = errors.New("only the most recently applied transition can be reverted")
	// ErrPreviewStale is returned by Apply when the cohort resolved under the
	// locks is no longer the one the admin confirmed, or when a child entered
	// a mapped class after the cohort was locked.
	ErrPreviewStale = errors.New("the preview is out of date: the affected classes or children have changed")
	// ErrGraduatesCheckedIn is returned by Apply when a graduating child is
	// still checked in; the admin must check the children out first.
	ErrGraduatesCheckedIn = errors.New("graduating students are still checked in")
)

// Graduate lifecycle states reported alongside a transition's ledger. They
// describe what is left of a graduated child TODAY, which decides which
// actions the Abgänge view may offer.
const (
	GraduateStateAlumnus  = "alumnus"
	GraduateStateRestored = "restored"
	GraduateStatePurged   = "purged"
)

// PurgedStudentName is the ledger name of a child whose person row no
// longer exists at apply time; it matches the School Structure placeholder.
const PurgedStudentName = schoolstructure.PurgedStudentPlaceholder

// The transition rows the workflow returns are School Structure's public
// types; the aliases and constants let an HTTP adapter depend on the
// workflow alone.
type (
	Transition                  = schoolstructure.Transition
	TransitionMapping           = schoolstructure.TransitionMapping
	TransitionHistoryEntry      = schoolstructure.TransitionHistoryEntry
	TransitionClassTeacherEntry = schoolstructure.TransitionClassTeacherEntry
	TransitionClassListEntry    = schoolstructure.TransitionClassListEntry
	ReleasedTag                 = peopledirectory.ReleasedTag
)

const (
	StatusDraft    = schoolstructure.TransitionStatusDraft
	StatusApplied  = schoolstructure.TransitionStatusApplied
	StatusReverted = schoolstructure.TransitionStatusReverted

	ActionPromoted  = schoolstructure.TransitionActionPromoted
	ActionGraduated = schoolstructure.TransitionActionGraduated
)

// Actor is the authenticated principal an operation is attributed to.
type Actor struct {
	TenantID  int64
	AccountID int64
}

// Mapping is one class rename of a draft; a nil target graduates the class.
type Mapping struct {
	FromClass string
	ToClass   *string
}

// Draft is what an admin submits to create a transition.
type Draft struct {
	AcademicYear string
	Notes        *string
	Mappings     []Mapping
}

// DraftPatch updates a draft. A nil Mappings slice keeps the stored
// mappings; an empty one clears them.
type DraftPatch struct {
	AcademicYear *string
	Notes        *string
	Mappings     []Mapping
}

// ListFilter narrows a listing. AfterID switches to an ascending id window
// and resets the page to the first one.
type ListFilter struct {
	Status       string
	AcademicYear string
	AfterID      int64
	Page         int
	PageSize     int
}

// Preview describes what an apply would do. Fingerprint identifies the exact
// cohort: the mappings plus every affected child and the class they are in.
// Apply accepts it back and refuses with ErrPreviewStale when the cohort
// resolved under the locks differs, so an admin can never confirm one set of
// children and graduate another.
type Preview struct {
	TransitionID    int64
	AcademicYear    string
	TotalStudents   int
	ToPromote       int
	ToGraduate      int
	ByMapping       []MappingPreview
	UnmappedClasses []UnmappedClass
	Warnings        []string
	Fingerprint     string
}

type MappingPreview struct {
	FromClass    string
	ToClass      *string
	StudentCount int
	Action       string
}

type UnmappedClass struct {
	ClassName    string
	StudentCount int
}

// Result reports what one committed apply or revert changed.
type Result struct {
	TransitionID      int64
	Status            string
	StudentsPromoted  int
	StudentsGraduated int
	CanRevert         bool
	Warnings          []string
}

// SuggestedMapping is an auto-suggested class mapping. Ambiguous marks a
// class whose name does not match the grade pattern; the editor must not
// preselect a graduation for those.
type SuggestedMapping struct {
	FromClass    string
	ToClass      *string
	StudentCount int
	IsGraduating bool
	Ambiguous    bool
}

// HistoryEntry is one ledger row together with the child's state today.
type HistoryEntry struct {
	schoolstructure.TransitionHistoryEntry
	StudentState string
}

// ClassListEntryAudit is one audit row of a class-list rewrite.
type ClassListEntryAudit struct {
	EntryID  int64
	Action   string
	OldValue string
	NewValue string
}

// Structure is the School Structure port: the draft, its mappings and the
// three ledgers.
type Structure interface {
	schoolstructure.TransitionCapability
}

// Directory is the People Directory port: cohort reads, row locks, the
// class and status writes, and the bracelet commands.
type Directory interface {
	ListStudentsByID(context.Context, []int64) ([]peopledirectory.Student, error)
	ListStudentsByClasses(context.Context, []string) ([]peopledirectory.Student, error)
	ListSchoolClasses(context.Context) ([]string, error)
	ListStudentNamesByID(context.Context, []int64) ([]peopledirectory.StudentName, error)
	ReadEnrollmentStudent(context.Context, int64, string) (peopledirectory.EnrollmentRecord, error)
	// LockEnrollmentClassWritesExclusive is the tenant-wide class-writes gate,
	// the FIRST lock of an apply or revert.
	LockEnrollmentClassWritesExclusive(context.Context) error
	PromoteStudents(ctx context.Context, ids []int64, fromClass, toClass string) (int64, error)
	RevertStudentClass(ctx context.Context, id int64, fromClass, toClass string) (int64, error)
	GraduateStudents(context.Context, []int64) (int64, error)
	ReactivateStudents(ctx context.Context, ids []int64, status string) ([]int64, error)
	ReleaseTags(context.Context, []int64) ([]peopledirectory.ReleasedTag, error)
	RestoreTag(context.Context, int64, string) (bool, error)
}

// Membership is the School Membership port: class-teacher assignments,
// class-list entries and the staff liveness the revert consults.
type Membership interface {
	ListClassAssignments(context.Context, schoolmembership.ClassAssignmentFilter) ([]schoolmembership.ClassAssignment, error)
	CreateClassAssignment(context.Context, schoolmembership.CreateClassAssignment) (schoolmembership.ClassAssignment, error)
	DeleteClassAssignment(context.Context, int64) error
	ListStaff(context.Context, schoolmembership.StaffFilter) ([]schoolmembership.Staff, error)
	ListClassListEntries(context.Context, schoolmembership.ClassListEntryFilter) ([]schoolmembership.ClassListEntry, error)
	CreateClassListEntry(context.Context, schoolmembership.CreateClassListEntry) (schoolmembership.ClassListEntry, error)
	DeleteClassListEntry(context.Context, int64) error
}

// Presence is the Student Presence port behind the check-in guard.
type Presence interface {
	ListVisits(context.Context, studentpresence.VisitFilter) ([]studentpresence.Visit, error)
	ListAttendance(context.Context, studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error)
}

// Rosters is the Timetable & Activities port over the already-materialized
// rosters: the apply archives the graduates' still-planned rows and records
// the baseline marker, the revert replays the archive and fills the instances
// materialized while the children were alumni.
type Rosters interface {
	RemoveStudentsFromFutureRosters(ctx context.Context, transitionID int64, studentIDs []int64) error
	RestoreStudentsToFutureRosters(ctx context.Context, transitionID int64, studentIDs []int64, baselineInstanceID *int64) error
	CurrentRosterBaseline(ctx context.Context) (int64, error)
}

// Dependencies are consumer-owned ports. UnitOfWork must join an ambient
// tenant transaction or open one. Authorize resolves the tenant principal
// for one operation. LockRecurrenceWrites is the tenant-wide recurrence gate
// every timetable writer takes; ResyncOfferingRosters re-reconciles the
// offering-sourced templates after the class rewrite (effectiveFrom is a
// YYYY-MM-DD calendar day).
type Dependencies struct {
	UnitOfWork func(context.Context, func(context.Context) error) error
	Authorize  func(context.Context, string) (Actor, error)
	Today      func() string
	Now        func() time.Time
	Structure  Structure
	Directory  Directory
	Membership Membership
	Presence   Presence
	Rosters    Rosters

	LockRecurrenceWrites      func(context.Context) error
	ResyncOfferingRosters     func(ctx context.Context, effectiveFrom string) error
	AppendClassListEntryAudit func(context.Context, Actor, ClassListEntryAudit) error
	Observe                   func(Observation)
}

type Observation struct {
	Operation    string
	Duration     time.Duration
	TransitionID int64
	Err          error
}

type Workflow struct{ deps Dependencies }

func New(deps Dependencies) (*Workflow, error) {
	if deps.UnitOfWork == nil || deps.Authorize == nil || deps.Today == nil || deps.Now == nil ||
		deps.Structure == nil || deps.Directory == nil || deps.Membership == nil || deps.Presence == nil || deps.Rosters == nil ||
		deps.LockRecurrenceWrites == nil || deps.ResyncOfferingRosters == nil || deps.AppendClassListEntryAudit == nil || deps.Observe == nil {
		return nil, errors.New("grade transition: all dependencies are required")
	}
	return &Workflow{deps: deps}, nil
}

func (w *Workflow) run(ctx context.Context, operation, permission string, transitionID int64, fn func(context.Context, Actor) error) (err error) {
	started := time.Now()
	defer func() {
		w.deps.Observe(Observation{Operation: operation, Duration: time.Since(started), TransitionID: transitionID, Err: err})
	}()
	return w.deps.UnitOfWork(ctx, func(txCtx context.Context) error {
		actor, err := w.deps.Authorize(txCtx, permission)
		if err != nil {
			return err
		}
		if actor.TenantID <= 0 || actor.AccountID <= 0 {
			return ErrUnauthorized
		}
		return fn(txCtx, actor)
	})
}

// CreateDraft stores a new draft with its mappings.
func (w *Workflow) CreateDraft(ctx context.Context, draft Draft) (result schoolstructure.Transition, err error) {
	if draft.AcademicYear == "" {
		return schoolstructure.Transition{}, fmt.Errorf("%w: academic_year is required", ErrInvalidTransitionData)
	}
	err = w.run(ctx, "create", OperationCreate, 0, func(txCtx context.Context, actor Actor) error {
		created, err := w.deps.Structure.CreateTransition(txCtx, schoolstructure.TransitionDraft{
			AcademicYear: draft.AcademicYear, Notes: draft.Notes, CreatedBy: actor.AccountID, Mappings: mappingInputs(draft.Mappings),
		})
		if err != nil {
			return translateStructureError(err)
		}
		result = created
		return nil
	})
	if err != nil {
		return schoolstructure.Transition{}, err
	}
	return result, nil
}

// UpdateDraft edits a draft. It takes the tenant transition gate BEFORE
// reading the row: an apply validates the confirmed fingerprint against the
// mappings and then writes history from that same set without locking the
// mapping rows, so a concurrent mapping replacement must wait for it (or be
// seen by it). Only the transition gate is taken here, keeping the
// acquisition order acyclic against apply and revert, which take the
// recurrence gate first.
func (w *Workflow) UpdateDraft(ctx context.Context, id int64, patch DraftPatch) (result schoolstructure.Transition, err error) {
	err = w.run(ctx, "update", OperationUpdate, id, func(txCtx context.Context, _ Actor) error {
		if err := w.deps.Structure.LockTransitions(txCtx); err != nil {
			return err
		}
		current, err := w.deps.Structure.FindTransition(txCtx, id)
		if err != nil {
			return translateStructureError(err)
		}
		if !current.IsDraft() {
			return fmt.Errorf("cannot modify transition: %w", ErrTransitionNotDraft)
		}
		updated, err := w.deps.Structure.UpdateTransition(txCtx, schoolstructure.TransitionUpdate{
			ID: id, AcademicYear: patch.AcademicYear, Notes: patch.Notes, Mappings: mappingInputs(patch.Mappings),
		})
		if err != nil {
			return translateStructureError(err)
		}
		result = updated
		return nil
	})
	if err != nil {
		return schoolstructure.Transition{}, err
	}
	return result, nil
}

// DeleteDraft removes a draft under the same gate as UpdateDraft: an apply
// that already read the mappings must not find the row gone underneath it.
func (w *Workflow) DeleteDraft(ctx context.Context, id int64) error {
	return w.run(ctx, "delete", OperationDelete, id, func(txCtx context.Context, _ Actor) error {
		if err := w.deps.Structure.LockTransitions(txCtx); err != nil {
			return err
		}
		current, err := w.deps.Structure.FindTransition(txCtx, id)
		if err != nil {
			return translateStructureError(err)
		}
		if !current.IsDraft() {
			return fmt.Errorf("cannot delete transition: %w", ErrTransitionNotDraft)
		}
		return translateStructureError(w.deps.Structure.DeleteTransition(txCtx, id))
	})
}

// FindTransition returns one transition with its mappings.
func (w *Workflow) FindTransition(ctx context.Context, id int64) (result schoolstructure.Transition, err error) {
	err = w.run(ctx, "find", OperationRead, id, func(txCtx context.Context, _ Actor) error {
		found, err := w.deps.Structure.FindTransition(txCtx, id)
		if err != nil {
			return translateStructureError(err)
		}
		result = found
		return nil
	})
	if err != nil {
		return schoolstructure.Transition{}, err
	}
	return result, nil
}

// ListTransitions returns a page of transitions with their mappings and the
// total.
func (w *Workflow) ListTransitions(ctx context.Context, filter ListFilter) (result []schoolstructure.Transition, total int, err error) {
	err = w.run(ctx, "list", OperationRead, 0, func(txCtx context.Context, _ Actor) error {
		page, pageSize := filter.Page, filter.PageSize
		if page < 1 {
			page = 1
		}
		if pageSize < 1 {
			pageSize = 20
		}
		if filter.AfterID > 0 {
			page = 1
		}
		transitions, count, err := w.deps.Structure.ListTransitions(txCtx, schoolstructure.TransitionFilter{
			Status: filter.Status, AcademicYear: filter.AcademicYear, AfterID: filter.AfterID,
			Limit: pageSize, Offset: (page - 1) * pageSize,
		})
		if err != nil {
			return translateStructureError(err)
		}
		result, total = transitions, count
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return result, total, nil
}

// ListClasses returns the distinct classes of the tenant's enrolled
// children.
func (w *Workflow) ListClasses(ctx context.Context) (result []string, err error) {
	err = w.run(ctx, "list_classes", OperationRead, 0, func(txCtx context.Context, _ Actor) error {
		classes, err := w.deps.Directory.ListSchoolClasses(txCtx)
		if err != nil {
			return err
		}
		result = classes
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// History returns a transition's ledger together with the CURRENT state of
// every child it names. The ledger alone cannot drive the Abgänge view: it is
// append-only, so a child who has since been reverted or purged still reads
// as graduated in it.
func (w *Workflow) History(ctx context.Context, id int64) (result []HistoryEntry, err error) {
	err = w.run(ctx, "history", OperationRead, id, func(txCtx context.Context, _ Actor) error {
		// A draft legitimately has no history; a nonexistent id must be a
		// not-found error, not an empty list.
		if _, err := w.deps.Structure.FindTransition(txCtx, id); err != nil {
			return translateStructureError(err)
		}
		entries, err := w.deps.Structure.ListTransitionHistory(txCtx, id)
		if err != nil {
			return translateStructureError(err)
		}
		ids := make([]int64, 0, len(entries))
		for _, entry := range entries {
			ids = append(ids, entry.StudentID)
		}
		states := make(map[int64]string, len(ids))
		if len(ids) > 0 {
			students, err := w.deps.Directory.ListStudentsByID(txCtx, ids)
			if err != nil {
				return err
			}
			for _, student := range students {
				state := GraduateStateRestored
				if student.IsAlumnus() {
					state = GraduateStateAlumnus
				}
				states[student.ID] = state
			}
		}
		result = make([]HistoryEntry, 0, len(entries))
		for _, entry := range entries {
			state, exists := states[entry.StudentID]
			if !exists {
				state = GraduateStatePurged
			}
			result = append(result, HistoryEntry{TransitionHistoryEntry: entry, StudentState: state})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func mappingInputs(mappings []Mapping) []schoolstructure.TransitionMappingInput {
	if mappings == nil {
		return nil
	}
	result := make([]schoolstructure.TransitionMappingInput, 0, len(mappings))
	for _, mapping := range mappings {
		result = append(result, schoolstructure.TransitionMappingInput{FromClass: mapping.FromClass, ToClass: mapping.ToClass})
	}
	return result
}

// translateStructureError maps the owner's outcomes onto the workflow's
// sentinels; every other error is a fault and passes through.
func translateStructureError(err error) error {
	var invalid *schoolstructure.InvalidTransitionError
	switch {
	case err == nil:
		return nil
	case errors.Is(err, schoolstructure.ErrTransitionNotFound):
		return ErrTransitionNotFound
	case errors.Is(err, schoolstructure.ErrTransitionStateConflict):
		// The owner refused a draft-only command on a row that left the draft
		// status behind the workflow's own check.
		return fmt.Errorf("cannot modify transition: %w", ErrTransitionNotDraft)
	case errors.As(err, &invalid):
		return fmt.Errorf("%w: %s", ErrInvalidTransitionData, invalid.Reason)
	}
	return err
}
