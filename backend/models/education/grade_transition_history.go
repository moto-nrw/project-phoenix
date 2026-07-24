package education

import (
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Transition action constants
const (
	ActionPromoted  = "promoted"
	ActionGraduated = "graduated"
	ActionUnchanged = "unchanged"
)

// GradeTransitionHistory records individual student changes during a grade transition
type GradeTransitionHistory struct {
	base.Model `bun:"schema:education,table:grade_transition_history"`
	base.TenantModel
	TransitionID int64   `bun:"transition_id,notnull" json:"transition_id"`
	StudentID    int64   `bun:"student_id,notnull" json:"student_id"`   // Keep even if student deleted
	PersonName   string  `bun:"person_name,notnull" json:"person_name"` // Snapshot for audit trail
	FromClass    string  `bun:"from_class,notnull" json:"from_class"`
	ToClass      *string `bun:"to_class" json:"to_class,omitempty"` // NULL = graduated/deleted
	Action       string  `bun:"action,notnull" json:"action"`       // 'promoted', 'graduated', 'unchanged'
	// FromStatus snapshots the student's lifecycle status at apply time so a
	// revert restores graduates to exactly what they were (pending / inactive /
	// active) instead of blanket-activating them. Nullable for rows written
	// before the from_status column existed (revert falls back to 'active').
	FromStatus *string `bun:"from_status" json:"from_status,omitempty"`
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
