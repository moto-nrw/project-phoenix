package audit

import (
	"context"
	"errors"
	"time"
)

// Class-list entry audit actions (#2382). Part of the audit wire format.
const (
	ClassListEntryActionCreated  = "created"
	ClassListEntryActionUpdated  = "updated"
	ClassListEntryActionDeleted  = "deleted"
	ClassListEntryActionAssigned = "assigned"
)

// ClassListEntryChange is an append-only audit row for one change to a
// class-list-only entry (#2382). Entries carry children's names, so every
// create / update / delete / assign must leave a trace — same contract as
// audit.staff_master_data_changes, written in the same tenant transaction as
// the change itself. Old/new carry the display form "Vorname Nachname
// (Klasse)"; MatchedStudentID records which student an "assigned" resolution
// attached the entry to before deleting it.
type ClassListEntryChange struct {
	ID int64 `bun:"id,pk,autoincrement" json:"id"`
	TenantModel
	EntryID          int64     `bun:"entry_id,notnull" json:"entry_id"`
	Action           string    `bun:"action,notnull" json:"action"`
	OldValue         string    `bun:"old_value,notnull" json:"old_value"`
	NewValue         string    `bun:"new_value,notnull" json:"new_value"`
	MatchedStudentID *int64    `bun:"matched_student_id" json:"matched_student_id,omitempty"`
	ChangedBy        int64     `bun:"changed_by,notnull" json:"changed_by"`
	OccurredAt       time.Time `bun:"occurred_at,nullzero,notnull,default:now()" json:"occurred_at"`
}

// Validate ensures the audit row is complete.
func (c *ClassListEntryChange) Validate() error {
	if c.EntryID <= 0 {
		return errors.New("entry_id is required")
	}
	if c.ChangedBy <= 0 {
		return errors.New("changed_by is required")
	}
	switch c.Action {
	case ClassListEntryActionCreated, ClassListEntryActionUpdated,
		ClassListEntryActionDeleted, ClassListEntryActionAssigned:
		// valid
	default:
		return errors.New("unknown class list entry action")
	}
	return nil
}

// ClassListEntryChangeRepository is append-only; there is no application-side
// update or delete on the trail.
type ClassListEntryChangeRepository interface {
	Create(ctx context.Context, change *ClassListEntryChange) error
	// ListByEntryID returns the trail of one entry, newest first.
	ListByEntryID(ctx context.Context, entryID int64) ([]*ClassListEntryChange, error)
}
