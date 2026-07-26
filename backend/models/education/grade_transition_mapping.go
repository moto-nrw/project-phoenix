package education

import (
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// GradeTransitionMapping represents a class-to-class mapping for a grade transition
type GradeTransitionMapping struct {
	base.Model `bun:"schema:education,table:grade_transition_mappings"`
	base.TenantModel
	TransitionID int64   `bun:"transition_id,notnull" json:"transition_id"`
	FromClass    string  `bun:"from_class,notnull" json:"from_class"`
	ToClass      *string `bun:"to_class" json:"to_class,omitempty"` // NULL = graduate/delete
}

// Validate ensures mapping data is valid
func (m *GradeTransitionMapping) Validate() error {
	m.FromClass = strings.TrimSpace(m.FromClass)

	if m.TransitionID <= 0 {
		return errors.New("transition_id is required")
	}

	if m.FromClass == "" {
		return errors.New("from_class is required")
	}

	// Trim to_class if provided
	if m.ToClass != nil {
		trimmed := strings.TrimSpace(*m.ToClass)
		if trimmed == "" {
			m.ToClass = nil // Empty string becomes nil (graduate)
		} else {
			m.ToClass = &trimmed
		}
	}

	// from_class and to_class cannot be the same
	if m.ToClass != nil && m.FromClass == *m.ToClass {
		return errors.New("from_class and to_class cannot be the same")
	}

	return nil
}

// IsGraduating returns true if this mapping means students will graduate (be deleted)
func (m *GradeTransitionMapping) IsGraduating() bool {
	return m.ToClass == nil
}

// GetAction returns the action type for this mapping
func (m *GradeTransitionMapping) GetAction() string {
	if m.IsGraduating() {
		return ActionGraduated
	}
	return ActionPromoted
}
