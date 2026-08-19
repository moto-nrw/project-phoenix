package education

import (
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// GradeTransitionClassListEntry is one ledger entry of what a grade-transition
// apply did to users.class_list_entries (#2382): 'removed' rows it deleted
// (with their original display form), 'created' rows it inserted. The revert
// replays these entries exactly instead of reversing the class mappings — a
// reverse rename cannot distinguish an entry the apply renamed into a class
// from a pre-existing entry of that class it never touched. Mirrors
// GradeTransitionClassTeacher; the action values are shared.
type GradeTransitionClassListEntry struct {
	base.Model `bun:"schema:education,table:grade_transition_class_list_entries"`
	base.TenantModel
	TransitionID int64 `bun:"transition_id,notnull" json:"transition_id"`
	// EntryID is the row the apply inserted ('created' rows only; nil on
	// 'removed' rows, whose row is gone). The revert resolves created rows by
	// this ID instead of by name and class: an admin edit that keeps the
	// normalized identity ("2a" → "2A") is invisible to an identity lookup and
	// would be silently overwritten.
	EntryID     *int64 `bun:"entry_id" json:"entry_id,omitempty"`
	FirstName   string `bun:"first_name,notnull" json:"first_name"`
	LastName    string `bun:"last_name,notnull" json:"last_name"`
	SchoolClass string `bun:"school_class,notnull" json:"school_class"`
	Action      string `bun:"action,notnull" json:"action"`
}

// Validate ensures ledger entry data is valid
func (h *GradeTransitionClassListEntry) Validate() error {
	if h.TransitionID <= 0 {
		return errors.New("transition ID is required")
	}
	if strings.TrimSpace(h.FirstName) == "" {
		return errors.New("first name is required")
	}
	if strings.TrimSpace(h.LastName) == "" {
		return errors.New("last name is required")
	}
	if strings.TrimSpace(h.SchoolClass) == "" {
		return errors.New("school class is required")
	}
	if h.Action != ClassTeacherActionRemoved && h.Action != ClassTeacherActionCreated {
		return errors.New("action must be removed or created")
	}
	if h.EntryID != nil && *h.EntryID <= 0 {
		return errors.New("entry ID must be positive")
	}
	return nil
}
