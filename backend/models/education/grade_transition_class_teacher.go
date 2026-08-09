package education

import (
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Actions recorded in the class-teacher transition ledger.
const (
	// ClassTeacherActionRemoved marks a class_teachers row the apply deleted,
	// preserving its original display form for the revert.
	ClassTeacherActionRemoved = "removed"
	// ClassTeacherActionCreated marks a class_teachers row the apply
	// inserted; the revert deletes it again if it still exists.
	ClassTeacherActionCreated = "created"
)

// GradeTransitionClassTeacher is one ledger entry of what a grade-transition
// apply did to education.class_teachers (#1772). The revert replays these
// entries exactly instead of reversing the class mappings — a reverse rename
// cannot distinguish a row the apply renamed into a class from a pre-existing
// assignment of that class it never touched. Mirrors GradeTransitionHistory
// on the student side.
type GradeTransitionClassTeacher struct {
	base.Model `bun:"schema:education,table:grade_transition_class_teachers"`
	base.TenantModel
	TransitionID int64  `bun:"transition_id,notnull" json:"transition_id"`
	StaffID      int64  `bun:"staff_id,notnull" json:"staff_id"`
	SchoolClass  string `bun:"school_class,notnull" json:"school_class"`
	Action       string `bun:"action,notnull" json:"action"`
}

// Validate ensures ledger entry data is valid
func (h *GradeTransitionClassTeacher) Validate() error {
	if h.TransitionID <= 0 {
		return errors.New("transition ID is required")
	}
	if h.StaffID <= 0 {
		return errors.New("staff ID is required")
	}
	if strings.TrimSpace(h.SchoolClass) == "" {
		return errors.New("school class is required")
	}
	if h.Action != ClassTeacherActionRemoved && h.Action != ClassTeacherActionCreated {
		return errors.New("action must be removed or created")
	}
	return nil
}
