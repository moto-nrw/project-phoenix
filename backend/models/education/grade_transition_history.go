package education

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Transition action constants
const (
	ActionPromoted   = "promoted"
	ActionGraduated  = "graduated"
	ActionUnchanged  = "unchanged"
)

// GradeTransitionHistory records individual student changes during a grade transition
type GradeTransitionHistory struct {
	base.Model   `bun:"schema:education,table:grade_transition_history"`
	TransitionID int64   `bun:"transition_id,notnull" json:"transition_id"`
	StudentID    int64   `bun:"student_id,notnull" json:"student_id"` // Keep even if student deleted
	PersonName   string  `bun:"person_name,notnull" json:"person_name"` // Snapshot for audit trail
	FromClass    string  `bun:"from_class,notnull" json:"from_class"`
	ToClass      *string `bun:"to_class" json:"to_class,omitempty"` // NULL = graduated/deleted
	Action       string  `bun:"action,notnull" json:"action"` // 'promoted', 'graduated', 'unchanged'
}

// BeforeAppendModel sets the correct table expression
func (h *GradeTransitionHistory) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(`education.grade_transition_history AS "history"`)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`education.grade_transition_history AS "history"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`education.grade_transition_history AS "history"`)
	}
	return nil
}

// TableName returns the database table name
func (h *GradeTransitionHistory) TableName() string {
	return "education.grade_transition_history"
}

// Validate ensures history data is valid
func (h *GradeTransitionHistory) Validate() error {
	h.PersonName = strings.TrimSpace(h.PersonName)
	h.FromClass = strings.TrimSpace(h.FromClass)
	h.Action = strings.TrimSpace(h.Action)

	if h.TransitionID <= 0 {
		return errors.New("transition_id is required")
	}

	if h.StudentID <= 0 {
		return errors.New("student_id is required")
	}

	if h.PersonName == "" {
		return errors.New("person_name is required")
	}

	if h.FromClass == "" {
		return errors.New("from_class is required")
	}

	if h.Action == "" {
		return errors.New("action is required")
	}

	if h.Action != ActionPromoted && h.Action != ActionGraduated && h.Action != ActionUnchanged {
		return errors.New("invalid action: must be promoted, graduated, or unchanged")
	}

	// Trim to_class if provided
	if h.ToClass != nil {
		trimmed := strings.TrimSpace(*h.ToClass)
		if trimmed == "" {
			h.ToClass = nil
		} else {
			h.ToClass = &trimmed
		}
	}

	return nil
}

// WasGraduated returns true if this record represents a graduating student
func (h *GradeTransitionHistory) WasGraduated() bool {
	return h.Action == ActionGraduated
}

// WasPromoted returns true if this record represents a promoted student
func (h *GradeTransitionHistory) WasPromoted() bool {
	return h.Action == ActionPromoted
}

// GetID returns the entity's ID
func (h *GradeTransitionHistory) GetID() interface{} {
	return h.ID
}

// GetCreatedAt returns the creation timestamp
func (h *GradeTransitionHistory) GetCreatedAt() time.Time {
	return h.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (h *GradeTransitionHistory) GetUpdatedAt() time.Time {
	return h.UpdatedAt
}
