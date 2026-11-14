package enrollment

import (
	"context"
	"time"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The holiday registration knows the parent account (ADR 0216 points 1 to 4
// and 9). A registration names at most one account, in its own row:
//
//   - A parent signed in to the parents portal gets a sealed proof of the
//     account with the prefill (ADR 0113). The public submission opens it
//     and links the registration at once. The account never travels as a
//     number in the request.
//   - Without a proof the parent answers at the end of the form whether they
//     want an account. On yes the registration mails an offer in the same
//     Träger transaction: the link to set up an account and the way in for
//     everyone who already has one. The answer of the route does not differ.
//   - Every other registration is claimed by the account whose address the
//     parent typed, at its next sign-in to the parents portal.
//
// The link names only the account. It unlocks no child of a school
// (ADR 0110 point 4).

// HolidayParentAccountProofLifetime is how long a sealed proof of the
// prefill counts, like the statement of the holiday care wish.
const HolidayParentAccountProofLifetime = 24 * time.Hour

// MailKindHolidayParentAccountOffer is the Delivery outbox kind of the offer
// of a parent account; the root registers its renderer under the same kind.
const MailKindHolidayParentAccountOffer = "holiday_parent_account_offer"

// HolidayParentAccountProofs seals the proof of a signed-in account for the
// prefill of a link: the Träger and the period of the link and the account.
// It reads nothing.
type HolidayParentAccountProofs interface {
	HolidayParentAccountProof(organizationID, periodID, accountID int64) (string, error)
}

// HolidayParentAccountLink is the way into a parent account the offer mail
// names: SetupURL sets up the account at the Träger, LoginURL is the sign-in
// of the parents portal for an address that already has one.
type HolidayParentAccountLink struct {
	SetupURL string
	LoginURL string
}

// HolidayParentAccountLinks issues the setup link of a parent account at a
// Träger for an address. It reads nothing, so it may run in the Träger
// transaction of the registration; the root binds it to Identity & Access.
type HolidayParentAccountLinks interface {
	HolidayParentAccountLink(email string, organizationID int64) (HolidayParentAccountLink, error)
}

// HolidayParentAccountOfferMail is the payload of the offer mail. It names
// the parent, the Träger and the period; no child and no link. The setup
// link is a credential for 30 days, so the outbox never holds it: the
// renderer decides at sending whether the address has a parent account and
// issues the setup link only for one without (ADR 0216 point 4).
type HolidayParentAccountOfferMail struct {
	Recipient      string `json:"recipient_email"`
	GuardianName   string `json:"guardian_name"`
	TraegerName    string `json:"traeger_name"`
	PeriodName     string `json:"period_name"`
	OrganizationID int64  `json:"organization_id"`
}

// HolidayParentRegistrationWeek is a booked week of a child of the account's
// registration: its days, its time model and its placement
// (HolidayPlacementPlaced, HolidayPlacementWaitlisted or
// HolidayPlacementPending).
type HolidayParentRegistrationWeek struct {
	FirstDay      calendar.Date
	LastDay       calendar.Date
	TimeModelName string
	Placement     string
	// Lunch says the child eats lunch in the week (#5631).
	Lunch bool
}

// HolidayParentRegistrationChild is a child of the account's registration by
// its first name, with its weeks in the order of the period.
type HolidayParentRegistrationChild struct {
	FirstName string
	Weeks     []HolidayParentRegistrationWeek
}

// HolidayParentRegistration is a registration linked to the account: the
// Träger, the period with its first and last care day (zero without weeks),
// when it came in, its state and per child the first name and the weeks. It
// carries no health information, no address, no contact, no last name and
// no date of birth.
type HolidayParentRegistration struct {
	ID             int64
	OrganizationID int64
	PeriodName     string
	FirstDay       calendar.Date
	LastDay        calendar.Date
	SubmittedAt    time.Time
	Status         string
	Children       []HolidayParentRegistrationChild
}

// HolidayParentRegistrations are the registrations of a parent account
// across every Träger. Each operation reads or writes across Träger and runs
// only in the caller's administrative transaction, with the account in its
// filter and a statement count independent of the rows.
type HolidayParentRegistrations interface {
	// ClaimHolidayRegistrations links every registration without an account
	// whose guardian address equals email, without regard to case, to the
	// account (ADR 0216 point 3). The caller vouches that email is the
	// account's confirmed address. It returns the number of registrations
	// claimed.
	ClaimHolidayRegistrations(ctx context.Context, accountID int64, email string) (int, error)
	// HolidayParentRegistrations reads the registrations linked to the
	// account, newest first.
	HolidayParentRegistrations(ctx context.Context, accountID int64) ([]HolidayParentRegistration, error)
	// HolidayParentRegistrationTraeger names the Träger at which the account
	// has a linked registration, ascending.
	HolidayParentRegistrationTraeger(ctx context.Context, accountID int64) ([]int64, error)
}
