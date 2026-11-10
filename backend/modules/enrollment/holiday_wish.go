package enrollment

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Holiday care wish in the holiday registration (#4703, decision of
// 09.11.): the Träger-Büro switches on per period that the wish the parents
// gave in the online registration of the school counts. A parent with an
// account and the relationship right brings the wish of the child from the
// prefill (ADR 0113) as a sealed statement; moto checks it when the form is
// sent. Every other child of a school of the Träger comes in with an unknown
// wish, which the office checks. The public form never looks a child up by
// name or date of birth (ADR 0110 point 4), so the answer to a submission
// without a statement never depends on a child of a school.
//
// The Träger setting holiday_registration.wish_mode decides what a missing
// wish means: with "priority" (the default) a child whose wish is "no"
// waits behind every other booking of its week; with "wish_only" a parent
// who brings a "no" cannot send the form. A holiday guest has no place at a
// school of the Träger and is never affected. The precedence only orders
// the waiting list: a free place still goes to whoever registers first, and
// no place is ever taken away (#4699).

// Values of holiday_registration.wish_mode.
const (
	HolidayWishModePriority = "priority"
	HolidayWishModeWishOnly = "wish_only"
	// HolidayWishModeOff is what the offer names for a period that does not
	// take the wish into account.
	HolidayWishModeOff = "off"
)

// What is known about the wish of a registered child, and where from.
const (
	HolidayWishYes = "yes"
	HolidayWishNo  = "no"
	// HolidayWishSourceAccount is the wish a parent with the relationship
	// right brought from the prefill.
	HolidayWishSourceAccount = "account"
	// HolidayWishSourceOffice is the decision of the Träger-Büro.
	HolidayWishSourceOffice = "office"
)

// Values of the review filter on the wish.
const (
	HolidayWishFilterCheck = "check"
	HolidayWishFilterYes   = HolidayWishYes
	HolidayWishFilterNo    = HolidayWishNo
)

// HolidayAuditWish records the office's decision on a child's wish.
const HolidayAuditWish = "wish"

// HolidayWishProofLifetime is how long a sealed statement of the prefill
// counts: long enough to fill in the form, short enough that a copied
// statement soon stops counting.
const HolidayWishProofLifetime = 24 * time.Hour

// CodeHolidayWishMissing refuses a child of a school of the Träger whose
// parents said no holiday care in the online registration, in a period
// that admits only children with the wish.
const CodeHolidayWishMissing = "enrollment.holiday_wish_missing"

var (
	// ErrHolidayWishMissing is the refusal of CodeHolidayWishMissing.
	ErrHolidayWishMissing = errors.New("enrollment: the period admits only children with a holiday care wish")
	// ErrHolidayWishInvalid reports an office decision that is neither yes
	// nor no, or one on a holiday guest.
	ErrHolidayWishInvalid = errors.New("enrollment: invalid holiday care wish")
)

// HolidayWishModes reads the Träger's holiday_registration.wish_mode in the
// caller's Träger transaction. The root binds the Träger level of the
// settings system.
type HolidayWishModes interface {
	HolidayWishMode(ctx context.Context, organizationID int64) (string, error)
}

// HolidayWishClaim names a child of a school of the Träger for which the
// prefill states the wish: the period of the link, the school and the
// child as the school knows it. On is the first day of the period: the
// wish of the school year that day lies in counts.
type HolidayWishClaim struct {
	OrganizationID int64
	PeriodID       int64
	SchoolID       int64
	StudentID      int64
	FirstName      string
	LastName       string
	BirthDate      calendar.Date
	On             calendar.Date
}

// HolidayWishStatement is the wish of a child as the prefill states it.
// Wish is HolidayWishYes, HolidayWishNo or empty when the school does not
// know it; Proof is the sealed statement the form sends back with the
// child, empty when the wish is unknown.
type HolidayWishStatement struct {
	Wish  string
	Proof string
}

// HolidayWishProofs states the wish of a child for the prefill. It runs in
// the tenant transaction of the child's school, after the caller checked
// the relationship right; the caller records the read in the school's data
// access log.
type HolidayWishProofs interface {
	HolidayWishProof(ctx context.Context, claim HolidayWishClaim) (HolidayWishStatement, error)
}

// HolidayWishReview lets the Träger-Büro decide the wish of a registered
// child of a school of the Träger: HolidayWishYes or HolidayWishNo. It runs
// in the Träger transaction of the reviewer's Träger and records the
// decision in the registration's trail. It is the office's half of the
// check: a child without a known wish waits for this decision.
type HolidayWishReview interface {
	SetHolidayWish(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID, childID int64, wish string) (HolidayRegistrationDetail, error)
}
