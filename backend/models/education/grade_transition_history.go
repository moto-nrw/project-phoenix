package education

import (
	"github.com/moto-nrw/project-phoenix/models/base"
)

// Action constants for grade transition history
const (
	ActionPromoted  = "promoted"
	ActionGraduated = "graduated"
	ActionUnchanged = "unchanged"
)

// GradeTransitionHistory is the persistence shape of one child's row in a
// transition's history. The owner capability lives in
// modules/schoolstructure (#2711); this struct remains for row fixtures.
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
	// RFIDTag snapshots the bracelet the graduate was holding when the
	// transition released it. Graduation clears users.persons.tag_id so the
	// physical tag can be handed to a current child; this ledger entry is what
	// lets a revert give it back. NULL means the child held no tag (or the row
	// predates the column) and the revert re-links nothing.
	RFIDTag *string `bun:"rfid_tag" json:"rfid_tag,omitempty"`
}
