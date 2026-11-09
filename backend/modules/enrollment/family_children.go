package enrollment

import (
	"context"
	"errors"
	"time"
)

// The children of the family on the status page of a registration (#6335).
// Before any contract the parents state how many children their family has,
// siblings without a care contract included, for the sibling rule of the
// Träger's parents' contributions. The number belongs to the registration
// of the status link, only the number is stored, and the contract of an
// accepted child takes it over into the Elternabrechnung
// (ContractApplication.FamilyChildren).
//
// Invariants:
//   - The status link reads and writes only its own registration, while it
//     is valid (unexpired, school still books the Online-Anmeldung).
//   - The page asks only where the number has a use: the school booked the
//     contracts and the parents' contributions.
//   - The parents can state and correct the number until a child of the
//     registration is accepted; from then on the contract carries it.

// MaxFamilyChildren is the largest number of children a family can state.
const MaxFamilyChildren = 20

var (
	// ErrFamilyChildrenClosed refuses a statement the registration no
	// longer takes: not asked, withdrawn, a child accepted or none open.
	ErrFamilyChildrenClosed = errors.New("enrollment: the registration takes no number of children")
	// ErrFamilyChildrenInvalid refuses a number outside 1 to 20.
	ErrFamilyChildrenInvalid = errors.New("enrollment: the number of children must be between 1 and 20")
)

// RegistrationFamilyChildren is the number of the family on a registration.
type RegistrationFamilyChildren struct {
	Children *int       `json:"children"`
	StatedAt *time.Time `json:"stated_at"`
	// Asked: the school asks for the number at all.
	Asked bool `json:"asked"`
	// CanState: the parents may state or correct it now.
	CanState bool `json:"can_state"`
}

// IntakeFamilyChildren serves the number on the status page. Each operation
// resolves the status token itself and opens the school's transaction.
type IntakeFamilyChildren interface {
	FamilyChildrenByStatusToken(ctx context.Context, token string) (RegistrationFamilyChildren, error)
	StateFamilyChildrenByStatusToken(ctx context.Context, token string, children int) (RegistrationFamilyChildren, error)
}
