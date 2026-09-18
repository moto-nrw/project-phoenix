package domain

import (
	"errors"
	"strings"
)

// The audited class-list administration (#2382): the three refusals that
// only exist once the entry is checked against the regular students of the
// same class. They are separate from the row-level errors above because a
// list entry never references a student — the relation is the shared name
// and class, nothing more.
var (
	// ErrClassListEntryStudentExists: a regular student already carries the
	// name and class, so the child is already on the class list.
	ErrClassListEntryStudentExists = errors.New("a student already carries this name and class")
	// ErrClassListEntryStudentNotFound: the assign target does not exist or
	// has graduated.
	ErrClassListEntryStudentNotFound = errors.New("assign target student not found")
	// ErrClassListEntryAssignMismatch: the assign target exists but does not
	// carry the entry's name and class.
	ErrClassListEntryAssignMismatch = errors.New("assign target student does not carry the entry name and class")
)

// Class-list entry audit actions (#2382). Part of the audit wire format.
const (
	ClassListEntryActionCreated  = "created"
	ClassListEntryActionUpdated  = "updated"
	ClassListEntryActionDeleted  = "deleted"
	ClassListEntryActionAssigned = "assigned"
)

// ClassListEntryChange is one row of the append-only change trail. Old and
// new carry the display form; MatchedStudentID records which student an
// "assigned" resolution attached the entry to before deleting it.
type ClassListEntryChange struct {
	EntryID          int64
	Action           string
	OldValue         string
	NewValue         string
	MatchedStudentID *int64
	ChangedBy        int64
}

// ClassListEntryDisplayValue renders the audit-trail representation of an
// entry: "Vorname Nachname (Klasse)".
func ClassListEntryDisplayValue(fields ClassListEntryFields) string {
	return strings.TrimSpace(fields.FirstName) + " " + strings.TrimSpace(fields.LastName) +
		" (" + strings.TrimSpace(fields.SchoolClass) + ")"
}

// Fields returns the writable part of an entry.
func (e ClassListEntry) Fields() ClassListEntryFields {
	return ClassListEntryFields{FirstName: e.FirstName, LastName: e.LastName, SchoolClass: e.SchoolClass}
}
