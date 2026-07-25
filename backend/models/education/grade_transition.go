package education

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Transition status constants
const (
	TransitionStatusDraft    = "draft"
	TransitionStatusApplied  = "applied"
	TransitionStatusReverted = "reverted"
)

// TenantTransitionsLockKey is the advisory-lock key that serializes everything
// which may not interleave with applying or reverting a grade transition, per
// tenant. It lives on the model because it has TWO independent holders that
// must agree on the exact string:
//
//   - GradeTransitionRepository.LockTenantTransitions — taken by Apply and
//     Revert before reading any class or lifecycle state.
//   - The timetable materializer (services/schedule) — taken for the whole
//     materialization pass.
//
// The second holder is what closes the graduation race: the materializer copies
// enrollments using the student status it read when the pass started, so
// without a shared gate an apply could commit a graduation AND finish its
// roster archive pass in between, leaving the materializer to insert an
// upcoming roster row for a child who is now an alumnus — a row nothing would
// ever remove. Re-reading the status before each insert does not close it
// (the graduation can commit in the gap); only mutual exclusion does (#405
// review).
//
// LOCK ORDER: holders that also take the tenant recurrence lock
// (`template-recurrence:<tenant>`) must take THAT one first. Apply/Revert take
// only this key, so the order stays acyclic and the two can never deadlock.
func TenantTransitionsLockKey(tenantID int64) string {
	return fmt.Sprintf("education.grade_transitions:%d", tenantID)
}

// GradeTransition represents a bulk grade level change operation
type GradeTransition struct {
	base.Model `bun:"schema:education,table:grade_transitions"`
	base.TenantModel

	AcademicYear string     `bun:"academic_year,notnull" json:"academic_year"`
	Status       string     `bun:"status,notnull,default:'draft'" json:"status"`
	AppliedAt    *time.Time `bun:"applied_at" json:"applied_at,omitempty"`
	AppliedBy    *int64     `bun:"applied_by" json:"applied_by,omitempty"`
	RevertedAt   *time.Time `bun:"reverted_at" json:"reverted_at,omitempty"`
	RevertedBy   *int64     `bun:"reverted_by" json:"reverted_by,omitempty"`
	CreatedBy    int64      `bun:"created_by,notnull" json:"created_by"`
	Notes        *string    `bun:"notes" json:"notes,omitempty"`
	Metadata     JSONMap    `bun:"metadata,type:jsonb,default:'{}'" json:"metadata,omitempty"`

	// Relations
	Mappings []*GradeTransitionMapping `bun:"rel:has-many,join:id=transition_id" json:"mappings,omitempty"`
	History  []*GradeTransitionHistory `bun:"rel:has-many,join:id=transition_id" json:"history,omitempty"`
}

// JSONMap is a helper type for JSONB columns
type JSONMap map[string]interface{}

// academicYearPattern validates the academic year format (e.g., "2025-2026")
var academicYearPattern = regexp.MustCompile(`^\d{4}-\d{4}$`)

// Validate ensures grade transition data is valid
func (t *GradeTransition) Validate() error {
	t.AcademicYear = strings.TrimSpace(t.AcademicYear)

	if t.AcademicYear == "" {
		return errors.New("academic year is required")
	}

	if !academicYearPattern.MatchString(t.AcademicYear) {
		return errors.New("academic year must be in format YYYY-YYYY (e.g., 2025-2026)")
	}

	if t.Status == "" {
		t.Status = TransitionStatusDraft
	}

	if t.Status != TransitionStatusDraft &&
		t.Status != TransitionStatusApplied &&
		t.Status != TransitionStatusReverted {
		return errors.New("invalid status: must be draft, applied, or reverted")
	}

	if t.CreatedBy <= 0 {
		return errors.New("created_by is required")
	}

	return nil
}

// IsDraft returns true if the transition is in draft status
func (t *GradeTransition) IsDraft() bool {
	return t.Status == TransitionStatusDraft
}

// IsApplied returns true if the transition has been applied
func (t *GradeTransition) IsApplied() bool {
	return t.Status == TransitionStatusApplied
}

// IsReverted returns true if the transition has been reverted
func (t *GradeTransition) IsReverted() bool {
	return t.Status == TransitionStatusReverted
}

// CanModify returns true if the transition can be modified (only drafts)
func (t *GradeTransition) CanModify() bool {
	return t.IsDraft()
}

// CanApply returns true if the transition can be applied
func (t *GradeTransition) CanApply() bool {
	return t.IsDraft() && len(t.Mappings) > 0
}

// CanRevert returns true if the transition can be reverted
func (t *GradeTransition) CanRevert() bool {
	return t.IsApplied()
}

// GradeTransitionRepository defines the interface for grade transition data access
type GradeTransitionRepository interface {
	// Transition CRUD
	Create(ctx context.Context, t *GradeTransition) error
	FindByID(ctx context.Context, id int64) (*GradeTransition, error)
	FindByIDWithMappings(ctx context.Context, id int64) (*GradeTransition, error)
	Update(ctx context.Context, t *GradeTransition) error
	Delete(ctx context.Context, id int64) error

	// Query methods
	List(ctx context.Context, options *base.QueryOptions) ([]*GradeTransition, int, error)
	FindByAcademicYear(ctx context.Context, year string) ([]*GradeTransition, error)
	FindByStatus(ctx context.Context, status string) ([]*GradeTransition, error)
	// LockLatestApplied returns the most recently applied transition (highest
	// applied_at, id as tiebreaker) with a FOR UPDATE lock, or nil when none is
	// applied. Reverts call it inside their transaction to enforce a strict
	// reverse-order unwind: only the latest applied transition may be reverted,
	// and the lock serializes concurrent reverts of the same row.
	LockLatestApplied(ctx context.Context) (*GradeTransition, error)
	// LockTenantTransitions takes a tenant-wide transaction-scoped advisory lock
	// that BOTH Apply and Revert acquire before doing any work. It is the shared
	// resource that serializes the two operations against each other: without it
	// an apply of a new draft can interleave with a revert of the current latest
	// transition, snapshotting pre-revert classes, losing its guarded class
	// update after the revert changes them, and still marking itself applied with
	// history describing changes it never made (#405 review). Requires the
	// caller's tenant transaction in context; the lock releases at COMMIT/ROLLBACK.
	LockTenantTransitions(ctx context.Context) error

	// Mapping operations
	CreateMapping(ctx context.Context, m *GradeTransitionMapping) error
	CreateMappings(ctx context.Context, mappings []*GradeTransitionMapping) error
	DeleteMappings(ctx context.Context, transitionID int64) error
	GetMappings(ctx context.Context, transitionID int64) ([]*GradeTransitionMapping, error)
	GetMappingsByTransitionIDs(ctx context.Context, transitionIDs []int64) (map[int64][]*GradeTransitionMapping, error)

	// History operations
	CreateHistory(ctx context.Context, h *GradeTransitionHistory) error
	CreateHistoryBatch(ctx context.Context, history []*GradeTransitionHistory) error
	GetHistory(ctx context.Context, transitionID int64) ([]*GradeTransitionHistory, error)

	// Bulk operations
	GetDistinctClasses(ctx context.Context) ([]string, error)
	GetStudentCountByClass(ctx context.Context, className string) (int, error)
	GetStudentsByClasses(ctx context.Context, classes []string) ([]*StudentClassInfo, error)
	// UpdateStudentClasses promotes every student currently sitting in a mapped
	// from-class. Apply does NOT use it: a class-wide update re-evaluates
	// membership at write time and therefore promotes students the history
	// snapshot never captured (unrevertable) while missing students it did
	// capture. Use PromoteStudentsByIDs for transition writes (#405 review).
	UpdateStudentClasses(ctx context.Context, transitionID int64) (int64, error)
	// PromoteStudentsByIDs moves exactly the given students from fromClass to
	// toClass, guarded on them still being in fromClass and not alumni. Apply
	// promotes the same FOR UPDATE-locked rows it recorded in history, so every
	// promotion is revertable and every history row describes a real one.
	PromoteStudentsByIDs(ctx context.Context, studentIDs []int64, fromClass, toClass string) (int64, error)
	// RevertStudentClass moves a promoted student back to fromClass, but ONLY
	// while their current class still equals toClass (the class this transition
	// assigned). A student manually moved to another class since — or promoted
	// again by a later transition — is left untouched (0 rows affected) so the
	// revert never clobbers a newer correction.
	RevertStudentClass(ctx context.Context, studentID int64, fromClass, toClass string) (int64, error)
	GraduateStudentsByClasses(ctx context.Context, classes []string) (int64, error)
	// GraduateStudentsByIDs soft-deletes exactly the given student IDs (status
	// flips to alumnus) provided they are not already alumni. Apply graduates the
	// same locked rows it checked for open check-ins and recorded in history, so
	// a student inserted or moved into a graduating class after the initial lookup
	// is never silently graduated without those guards (#405 review).
	GraduateStudentsByIDs(ctx context.Context, studentIDs []int64) (int64, error)
	ReactivateStudentsByIDs(ctx context.Context, studentIDs []int64) (int64, error)
	ReactivateStudentsToStatus(ctx context.Context, studentIDs []int64, targetStatus string) (int64, error)
}

// StudentClassInfo contains basic student information for class transitions
type StudentClassInfo struct {
	StudentID   int64  `bun:"student_id"`
	PersonID    int64  `bun:"person_id"`
	PersonName  string `bun:"person_name"`
	SchoolClass string `bun:"school_class"`
	Status      string `bun:"status"`
}
