package education

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// GradeTransitionMapping represents a class-to-class mapping for a grade transition
type GradeTransitionMapping struct {
	base.Model   `bun:"schema:education,table:grade_transition_mappings"`
	TransitionID int64   `bun:"transition_id,notnull" json:"transition_id"`
	FromClass    string  `bun:"from_class,notnull" json:"from_class"`
	ToClass      *string `bun:"to_class" json:"to_class,omitempty"` // NULL = graduate/delete
}

// BeforeAppendModel sets the correct table expression
func (m *GradeTransitionMapping) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(`education.grade_transition_mappings AS "mapping"`)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`education.grade_transition_mappings AS "mapping"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`education.grade_transition_mappings AS "mapping"`)
	}
	return nil
}

// TableName returns the database table name
func (m *GradeTransitionMapping) TableName() string {
	return "education.grade_transition_mappings"
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

// GetID returns the entity's ID
func (m *GradeTransitionMapping) GetID() interface{} {
	return m.ID
}

// GetCreatedAt returns the creation timestamp
func (m *GradeTransitionMapping) GetCreatedAt() time.Time {
	return m.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (m *GradeTransitionMapping) GetUpdatedAt() time.Time {
	return m.UpdatedAt
}
