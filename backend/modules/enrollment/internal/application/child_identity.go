package application

import (
	"strings"

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
