package peopledirectory

import (
	"context"
	"encoding/json"
)

// NewStudentGuardianLink is one new guardian of a child the caller's
// transaction created: the profile, its phone numbers and the relationship
// with its pickup permission and portal access.
type NewStudentGuardianLink struct {
	StudentID int64
	Guardian  NewStudentGuardian
}

// NewGuardianBatches creates the new guardians of many new children at once
// (#5552, ADR 0067 addendum of 2026-11-08, point 3: CreateGuardians,
// AddGuardianPhones and LinkGuardiansToStudents with Care Plan's pickup
// permissions and Identity & Access's portal access). It is a leaf command
// outside Capability and Owner; the data import binds it on its own.
type NewGuardianBatches interface {
	// CreateNewStudentGuardians writes, in the caller's transaction and all or
	// none, what CreateGuardian, AddGuardianPhone and LinkGuardianToStudent
	// write for a guardian without e-mail that is new to a new child: one
	// statement per table. It refuses an entry naming an existing profile or
	// an e-mail, a child with more than one primary guardian and a later
	// phone that would take over the primary flag; those keep the single
	// commands.
	CreateNewStudentGuardians(context.Context, []NewStudentGuardianLink) error
}

// NewGuardianLinkHalves are the Care Plan pickup half and the Identity &
// Access portal half of one new link the batch writes; the root binds them
// to those owners' batch commands.
type NewGuardianLinkHalves struct {
	RelationshipID int64
	CanPickup      bool
	PickupNotes    *string
	AccountID      *int64
	Permissions    json.RawMessage
}
