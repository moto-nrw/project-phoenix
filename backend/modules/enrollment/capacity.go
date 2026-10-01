package enrollment

import (
	"context"
	"encoding/base64"
	"hash/fnv"
	"strings"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// OfferingClaim is one capacity claim a child brings to the capacity gate:
// an offering plus the selection's validity interval (ValidUntil exclusive,
// matching RequestChildOffering). Submit-time selections span the whole
// phase (nil bounds); the restore path passes the surviving dated intervals
// so a claim is only checked against occupancy inside its own window.
type OfferingClaim struct {
	OfferingID int64
	ValidFrom  *calendar.Date
	ValidUntil *calendar.Date
}

// CapacityCheck is one run of the capacity gate: the claims per child in
// submission order, the slots a replaced request already held per offering,
// and the request children whose own claims must not count against them.
type CapacityCheck struct {
	Phase                   *Phase
	Claims                  [][]OfferingClaim
	PreservedClaims         map[int64]int
	ReplacedRequestChildIDs []int64
}

// OfferingCapacity is the shared capacity gate behind every path that
// (re-)creates active offering claims: a submission and its edit/replace
// variants, and the admin restore of a withdrawn request. It locks the
// selected offerings by ascending id, counts active claims in the phase's
// remaining capacity window, and flags over-capacity children per the
// phase's overflow mode (waitlist/reject/allow). The result maps a child's
// position to the status it must take instead of submitted.
type OfferingCapacity interface {
	ApplyCapacityOverflow(ctx context.Context, check CapacityCheck) (map[int]string, error)
}

// CareBookingInput is the effective-care data an enrollment workflow sends
// to Care Plan. Submitted days and notes are written separately through
// Enrollment.
type CareBookingInput struct {
	CareOfferingID        int64
	ManualSelectedDays    []string
	AutomaticSelectedDays []string
	ValidFrom             *calendar.Date
	ValidUntil            *calendar.Date
}

// CareBookingCommands records the effective bookings of a new request child
// in Care Plan.
type CareBookingCommands interface {
	RecordCareBookings(context.Context, int64, []CareBookingInput) error
}

// SubmissionDedupLockKey derives the advisory-lock key that serializes the
// submissions of one guardian email in a phase: a 64-bit FNV-1a hash of the
// lower-cased, trimmed address the caller passes.
func SubmissionDedupLockKey(normalizedEmail string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(normalizedEmail))
	return h.Sum64()
}

// NewStatusToken returns the public status token of a request: 32 random
// bytes from fill, URL-safe base64 without padding.
func NewStatusToken(fill func([]byte) error) (string, error) {
	b := make([]byte, 32)
	if err := fill(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// NormalizedSubmissionSource maps a stored submission source onto the known
// values; anything unknown is a public submission.
func NormalizedSubmissionSource(source string) string {
	switch strings.TrimSpace(source) {
	case RequestSourceLateInvite:
		return RequestSourceLateInvite
	case RequestSourceAdminManual:
		return RequestSourceAdminManual
	default:
		return RequestSourcePublic
	}
}
