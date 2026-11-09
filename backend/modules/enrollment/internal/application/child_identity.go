package application

import (
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/textnorm"
)

// childNameKey is the form in which two child names count as the same name:
// Unicode NFC (textnorm.Name), without surrounding spaces and without regard
// to case, so a decomposed "José" is the composed one.
//
// The SQL predicates of the duplicate check (request_duplicates.go) and of
// the hosted health read (holiday_host_participants.go) compare
// lower(btrim(...)) without NFC. Request children are stored through
// textnorm.Name, so the duplicate check agrees with this key; holiday
// registrations store names only trimmed, so a decomposed spelling there
// does not match the composed one of the school's record.
func childNameKey(name string) string {
	return strings.ToLower(textnorm.Name(name))
}

// guardianEmailKey is the form in which two guardian e-mail addresses count
// as the same address: without surrounding spaces and without regard to case.
func guardianEmailKey(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func requestChildIdentityKey(firstName, lastName, dateOfBirth string) string {
	return childNameKey(firstName) + "\x00" + childNameKey(lastName) + "\x00" + strings.TrimSpace(dateOfBirth)
}

// sameSubmittedIdentity reports whether an edited child still names the same
// person as the stored child it was paired to; a changed identity drops a
// stale existing-student pin (#1663).
func sameSubmittedIdentity(existing *enrollment.RequestChild, submitted SubmitChild) bool {
	if existing == nil {
		return false
	}
	return requestChildIdentityKey(existing.FirstName, existing.LastName, existing.DateOfBirth.String()) ==
		requestChildIdentityKey(submitted.FirstName, submitted.LastName, submitted.DateOfBirth.String())
}
