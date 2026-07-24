package education

import (
	"context"
	"errors"
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
	UpdateStudentClasses(ctx context.Context, transitionID int64) (int64, error)
	// RevertStudentClass moves a promoted student back to fromClass, but ONLY
	// while their current class still equals toClass (the class this transition
	// assigned). A student manually moved to another class since — or promoted
	// again by a later transition — is left untouched (0 rows affected) so the
	// revert never clobbers a newer correction.
	RevertStudentClass(ctx context.Context, studentID int64, fromClass, toClass string) (int64, error)
	GraduateStudentsByClasses(ctx context.Context, classes []string) (int64, error)
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
