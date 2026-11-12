package carecontract

import (
	"context"
	"time"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The children of the family for the sibling rule of the Elternabrechnung
// (#5918, #6335). The parents state the number, siblings without a contract
// included, through the one-time link of a contract mail, and on the status
// page of their registration before any contract; the contract an accepted
// child of that registration gets takes the registration's number over.
// The Elternabrechnung keeps the number, its confirmation and its trail;
// the Betreuungsvertrag checks the contract and names its family.
//
// Invariants:
//   - A link reads and states only the number of the family of the
//     contract it names, while it is open: unused, unexpired, its contract
//     still waits for the signature with the PDF it was sent with. Any other
//     link is ErrSigningLinkInvalid, as for the signature.
//   - The family is the contract's billing recipient (BillingRecipients).
//   - A registration's number is taken over only with its contract, in the
//     Träger transaction that makes the contract, and never replaces a
//     newer statement of the family.

// Channels of the parents' number.
const (
	FamilyChannelLink       = "link"
	FamilyChannelEnrollment = "enrollment"
)

// FamilyChildren is the number of the family of a contract as the parents
// see it.
type FamilyChildren struct {
	ConfirmedAt       *time.Time     `json:"confirmed_at"`
	StatedChildren    *int           `json:"stated_children"`
	StatedAt          *time.Time     `json:"stated_at"`
	ConfirmedChildren *int           `json:"confirmed_children"`
	ValidFrom         *calendar.Date `json:"valid_from"`
	ValidUntil        *calendar.Date `json:"valid_until"`
	// CanState tells that the parents may state a number here.
	CanState bool `json:"can_state"`
}

// FamilyStatement is the number the parents stated on their registration,
// with the time they stated it.
type FamilyStatement struct {
	Children int
	StatedAt time.Time
}

// FamilyContract names the family of a contract the Betreuungsvertrag
// checked: the contract, its billing recipient's address, the parents'
// account if known (else 0) and the channel.
type FamilyContract struct {
	ContractID int64
	Email      string
	AccountID  int64
	Channel    string
}

// FamilyNumbers is the root-bound port to the Elternabrechnung's numbers of
// the families. Both operations run in the caller's Träger transaction. A
// number outside 1 to 20 is a *ValidationError on "children".
type FamilyNumbers interface {
	FamilyChildren(ctx context.Context, organizationID int64, family FamilyContract) (FamilyChildren, error)
	// StateFamilyChildren stores the parents' number; a statedAt before
	// the family's newest statement changes nothing, nil is now.
	StateFamilyChildren(ctx context.Context, organizationID int64, family FamilyContract, children int, statedAt *time.Time) (FamilyChildren, error)
}

// LinkFamilyChildren serves the number of the family on the one-time link
// of a contract mail (#6335). Each operation opens the Träger transaction of
// the link and leaves the link open.
type LinkFamilyChildren interface {
	LinkFamilyChildren(ctx context.Context, link string) (FamilyChildren, error)
	StateLinkFamilyChildren(ctx context.Context, link string, children int) (FamilyChildren, error)
}
