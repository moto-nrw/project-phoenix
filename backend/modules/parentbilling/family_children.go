package parentbilling

import (
	"context"
	"errors"
	"time"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The children of a family for the sibling rule (#5918, ADR 0088 section 4
// addendum 2026-11-08, E2 option A). A family states how many children it
// has, siblings without a care contract included, and the Abrechnung of the
// Träger confirms or changes the number with a validity. A sibling rule
// that counts the family (ContributionRule.CountsFamilyChildren) ranks a
// family's children among max(confirmed number valid in the month, children
// with a contract the rule counts). The siblings without a contract come
// first in the order, as they pay nothing: a child with a contract of a
// family of three is the 2nd or 3rd child.
//
// Invariants:
//   - The family is the address of the invoice's recipient, compared without
//     case, like the families of the rule chain (ADR 0164 point 4).
//   - Only the number is stored, never a name or a date of birth of a child
//     without a contract, and no contract or registration is made up.
//   - A number the parents stated counts only once the Abrechnung confirmed
//     it: until then the rule counts the children with a contract. A newer
//     statement leaves the confirmed number in force and asks for a check.
//   - Validity is read for the month a run bills, not for today.
//   - The parents read and state only the number of their own family: a
//     contract sent to their address whose child their relationship lets
//     them act for (parent_portal.enrollment.submit).
//   - Every read and change is recorded per contract, with the value, the
//     validity, the account and whether a parent or the Abrechnung acted.

// MaxFamilyChildren is the largest number of children a family can state.
const MaxFamilyChildren = 20

// FamilyChildrenExpiryNotice is how many days before its end a confirmed
// number asks the Abrechnung for a check.
const FamilyChildrenExpiryNotice = 30

// States of the number of a family as the Abrechnung sees it today.
const (
	// FamilyChildrenMissing: nobody stated a number.
	FamilyChildrenMissing = "missing"
	// FamilyChildrenStated: the parents stated a number that waits for the
	// Abrechnung.
	FamilyChildrenStated = "stated"
	// FamilyChildrenChanged: the parents stated another number after the
	// confirmation; the confirmed one stays in force.
	FamilyChildrenChanged = "changed"
	// FamilyChildrenConfirmed: confirmed and valid today.
	FamilyChildrenConfirmed = "confirmed"
	// FamilyChildrenExpiring: confirmed, ends within the notice.
	FamilyChildrenExpiring = "expiring"
	// FamilyChildrenExpired: confirmed, but no longer valid.
	FamilyChildrenExpired = "expired"
	// FamilyChildrenUpcoming: confirmed from a later day on.
	FamilyChildrenUpcoming = "upcoming"
)

var (
	// ErrFamilyChildrenNotFound reports a family the Träger has no number
	// or no contract for.
	ErrFamilyChildrenNotFound = errors.New("no number of children for this family")
	// ErrFamilyWithoutAddress reports a contract whose invoice goes to no
	// address: it is a family of its own and has no number.
	ErrFamilyWithoutAddress = errors.New("the contract's invoice goes to no address")
)

// FamilyChildCount is the number of a family: what the parents stated and
// what the Abrechnung confirmed with its validity.
type FamilyChildCount struct {
	ID int64 `json:"id"`
	// ContractID is the contract the number was last stated or confirmed
	// for; the number goes with it.
	ContractID        int64          `json:"contract_id"`
	StatedChildren    *int           `json:"stated_children"`
	StatedAt          *time.Time     `json:"stated_at"`
	ConfirmedChildren *int           `json:"confirmed_children"`
	ValidFrom         *calendar.Date `json:"valid_from"`
	ValidUntil        *calendar.Date `json:"valid_until"`
	ConfirmedAt       *time.Time     `json:"confirmed_at"`
	UpdatedAt         time.Time      `json:"updated_at"`

	// FamilyEmail is the key of the family; the wire names the recipient
	// instead.
	FamilyEmail string `json:"-"`
	// Confirmations retains earlier confirmations, newest first, for billing months.
	Confirmations []FamilyChildConfirmation `json:"-"`
}

// InForceOn is the confirmation that decides a day: the newest one that
// started by then, as in ConfirmedIn, whether or not it still holds.
// started is false when no confirmation had started by that day.
func (c FamilyChildCount) InForceOn(day calendar.Date) (confirmation FamilyChildConfirmation, started bool) {
	confirmations := append([]FamilyChildConfirmation{{Children: c.ConfirmedChildren, ValidFrom: c.ValidFrom, ValidUntil: c.ValidUntil}}, c.Confirmations...)
	for _, candidate := range confirmations {
		if candidate.Children == nil || candidate.ValidFrom == nil || candidate.ValidFrom.After(day) {
			continue
		}
		return candidate, true
	}
	return FamilyChildConfirmation{}, false
}

// ConfirmedIn answers the confirmed number of the month that starts on
// first, like an evidence (ADR 0164 point 6): the one in force on the first
// day, else the first that starts within the month. A newer confirmation
// replaces older ones from its start onward, including after its expiry.
func (c FamilyChildCount) ConfirmedIn(first calendar.Date) (int, bool) {
	end := calendar.NewDate(first.Year(), first.Month()+1, 1).AddDays(-1)
	for day := first; !day.After(end); day = day.AddDays(1) {
		confirmation, started := c.InForceOn(day)
		if started && (confirmation.ValidUntil == nil || !confirmation.ValidUntil.Before(day)) {
			return *confirmation.Children, true
		}
	}
	return 0, false
}

// FamilyChildConfirmation is a retained confirmation with its effective dates.
type FamilyChildConfirmation struct {
	Children   *int           `json:"children"`
	ValidFrom  *calendar.Date `json:"valid_from"`
	ValidUntil *calendar.Date `json:"valid_until"`
}

// FamilyContractChild is a child of a family with a contract in force.
type FamilyContractChild struct {
	ContractID     int64  `json:"contract_id"`
	SchoolName     string `json:"school_name"`
	ChildFirstName string `json:"child_first_name"`
	ChildLastName  string `json:"child_last_name"`
}

// FamilyChildren is one family as the Abrechnung checks it: the recipient
// of its invoices, its children with a contract, its number and the state
// of that number today.
type FamilyChildren struct {
	RecipientFirstName string                `json:"recipient_first_name"`
	RecipientLastName  string                `json:"recipient_last_name"`
	RecipientEmail     string                `json:"recipient_email"`
	Children           []FamilyContractChild `json:"children"`
	Count              *FamilyChildCount     `json:"count"`
	Status             string                `json:"status"`
}

// FamilyChildrenSet is what the page shows: every family with a contract in
// force from three months back to a year ahead, and the sibling rules that
// count the family's number.
type FamilyChildrenSet struct {
	Families []FamilyChildren   `json:"families"`
	Rules    []ContributionRule `json:"rules"`
}

// FamilyChildrenInput is the number the Abrechnung confirms for the family
// of a contract, from ValidFrom on; a nil ValidUntil is open.
type FamilyChildrenInput struct {
	ContractID int64          `json:"contract_id"`
	Children   int            `json:"children"`
	ValidFrom  calendar.Date  `json:"valid_from"`
	ValidUntil *calendar.Date `json:"valid_until"`
}

// FamilyChildrenOffice lets the Abrechnung of a Träger check the numbers of
// its families. Every operation names the person of the session and runs in
// the Träger transaction of its Träger, which the caller opens.
type FamilyChildrenOffice interface {
	FamilyChildren(ctx context.Context, reviewer HolidayReviewer) (FamilyChildrenSet, error)
	// ConfirmFamilyChildren confirms or changes the number of the family of
	// the input's contract.
	ConfirmFamilyChildren(ctx context.Context, reviewer HolidayReviewer, input FamilyChildrenInput) (FamilyChildrenSet, error)
	// RemoveFamilyChildren removes the number of the family of a contract.
	RemoveFamilyChildren(ctx context.Context, reviewer HolidayReviewer, contractID int64) (FamilyChildrenSet, error)
}

// Actions and actors of the trail of a family's number.
const (
	FamilyChildEventRead    = "read"
	FamilyChildEventState   = "state"
	FamilyChildEventConfirm = "confirm"
	FamilyChildEventDelete  = "delete"

	FamilyChildActorParent = "parent"
	FamilyChildActorOffice = "office"
)

// FamilyChildEvent is one action of the trail: who did what, with the
// value and validity of a change.
type FamilyChildEvent struct {
	Action string
	Actor  string
	// Channel tells where a parents' read or statement came from
	// (FamilyChannel*); the Abrechnung has none.
	Channel    string
	AccountID  int64
	Children   *int
	ValidFrom  *calendar.Date
	ValidUntil *calendar.Date
}

// FamilyChildTarget is the contract an entry of the trail goes with and
// the number it names, if there is one.
type FamilyChildTarget struct {
	ContractID int64
	CountID    *int64
}

// ParentFamilyReader is the parents' account that reads or states the
// number on a contract the parents portal checked: sent to the account's
// address, with a child the relationship lets it act for. Email is the
// billing recipient resolved after authorization, not the mailing address.
type ParentFamilyReader struct {
	AccountID  int64
	Email      string
	ContractID int64
}

// ParentFamilyChildren is the number of the family as the parents see it.
type ParentFamilyChildren struct {
	ConfirmedAt       *time.Time     `json:"confirmed_at"`
	StatedChildren    *int           `json:"stated_children"`
	StatedAt          *time.Time     `json:"stated_at"`
	ConfirmedChildren *int           `json:"confirmed_children"`
	ValidFrom         *calendar.Date `json:"valid_from"`
	ValidUntil        *calendar.Date `json:"valid_until"`
}

// ParentFamilyChildrenStatement serves the parents portal for one Träger.
// Both operations run in the Träger transaction the caller opens, record the
// read or the statement, and key the family by the resolved billing address.
type ParentFamilyChildrenStatement interface {
	ParentFamilyChildren(ctx context.Context, organizationID int64, reader ParentFamilyReader) (ParentFamilyChildren, error)
	StateFamilyChildren(ctx context.Context, organizationID int64, reader ParentFamilyReader, children int) (ParentFamilyChildren, error)
}

// Channels of a parents' read or statement (#6335).
const (
	// FamilyChannelPortal: the contract in the parents portal (#5918).
	FamilyChannelPortal = "portal"
	// FamilyChannelLink: the one-time link of a contract mail, without an
	// account.
	FamilyChannelLink = "link"
	// FamilyChannelEnrollment: the status page of the registration, taken
	// over when the contract of an accepted child is made.
	FamilyChannelEnrollment = "enrollment"
)

// ContractFamily is the family of a contract the Betreuungsvertrag checked
// (#6335): the contract of an open one-time link, or a contract it just
// made from an accepted registration. Email is the contract's billing
// recipient, the key of the family. AccountID is the parents' account if
// one is known, else 0.
type ContractFamily struct {
	ContractID int64
	Email      string
	AccountID  int64
	Channel    string
}

// ContractFamilyChildren serves the Betreuungsvertrag's link and the
// takeover of a registration's number (#6335). Both operations run in the
// Träger transaction the caller opens, which already checked the contract,
// and record the read or the statement with its channel.
type ContractFamilyChildren interface {
	ContractFamilyChildren(ctx context.Context, organizationID int64, family ContractFamily) (ParentFamilyChildren, error)
	// StateContractFamilyChildren stores the parents' number. A statedAt
	// before the family's newest statement changes nothing; nil is now.
	StateContractFamilyChildren(ctx context.Context, organizationID int64, family ContractFamily, children int, statedAt *time.Time) (ParentFamilyChildren, error)
	// CountsFamilyChildrenAt tells whether a care sibling rule of the Träger
	// that applies to the school and has not ended by day counts the
	// family's children, so a registration there asks for them.
	CountsFamilyChildrenAt(ctx context.Context, organizationID, schoolID int64, day calendar.Date) (bool, error)
}
