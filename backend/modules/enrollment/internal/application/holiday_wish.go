package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The holiday care wish in the holiday registration (#4703); the rules are
// on enrollment.HolidayWishProofs. The prefill seals what the school knows
// of a child's wish with the field cipher that seals the health
// information, under a context of its own that names the Träger and the
// period: a statement opens only for the link it was made for, and a
// parent cannot change it. The submission opens it and keeps the wish only
// for the child the statement names, at the school it names.

// holidayWishProofVersion is the version of the sealed statement.
const holidayWishProofVersion = 1

// errHolidayWishModesRequired reports a registration built without the
// Träger's setting of the wish mode.
var errHolidayWishModesRequired = errors.New("enrollment: the holiday registration needs the Träger's holiday wish mode")

// HolidayCareWishStore reads the holiday care wish of an approved child in
// the tenant transaction of its school.
type HolidayCareWishStore interface {
	// ApprovedHolidayCareWish is the answer of the newest approved
	// enrollment of the student in a school-year phase whose service window
	// holds on; nil when there is none or the form did not ask.
	ApprovedHolidayCareWish(ctx context.Context, studentID int64, on calendar.Date) (*bool, error)
}

// holidayWishProof is the content of the sealed statement.
type holidayWishProof struct {
	Version        int           `json:"v"`
	OrganizationID int64         `json:"o"`
	PeriodID       int64         `json:"p"`
	SchoolID       int64         `json:"s"`
	FirstName      string        `json:"f"`
	LastName       string        `json:"l"`
	BirthDate      calendar.Date `json:"b"`
	Wish           string        `json:"w"`
	ExpiresAt      int64         `json:"e"`
}

func holidayWishProofContext(organizationID, periodID int64) []byte {
	return fmt.Appendf(nil, "enrollment.holiday_wish_proof:v%d:%d:%d", holidayWishProofVersion, organizationID, periodID)
}

// sealHolidayWishProof seals the statement of claim with wish.
func sealHolidayWishProof(cipher enrollment.HolidayHealthCipher, claim enrollment.HolidayWishClaim, wish string, now time.Time) (string, error) {
	plaintext, err := json.Marshal(holidayWishProof{
		Version: holidayWishProofVersion, OrganizationID: claim.OrganizationID, PeriodID: claim.PeriodID, SchoolID: claim.SchoolID,
		FirstName: claim.FirstName, LastName: claim.LastName, BirthDate: claim.BirthDate, Wish: wish,
		ExpiresAt: now.Add(enrollment.HolidayWishProofLifetime).Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("enrollment: encode holiday wish proof: %w", err)
	}
	sealed, err := cipher.Seal(plaintext, holidayWishProofContext(claim.OrganizationID, claim.PeriodID))
	if err != nil {
		return "", fmt.Errorf("enrollment: seal holiday wish proof: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// openHolidayWishProof opens a statement for the link's period. Anything
// that does not open, belongs to another link or expired is no statement:
// the child then comes in with an unknown wish, like a child without one.
func openHolidayWishProof(cipher enrollment.HolidayHealthCipher, organizationID, periodID int64, proof string, now time.Time) (holidayWishProof, bool) {
	sealed, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(proof))
	if err != nil || len(sealed) == 0 {
		return holidayWishProof{}, false
	}
	plaintext, err := cipher.Open(sealed, holidayWishProofContext(organizationID, periodID))
	if err != nil {
		return holidayWishProof{}, false
	}
	var statement holidayWishProof
	if err := json.Unmarshal(plaintext, &statement); err != nil || statement.Version != holidayWishProofVersion ||
		statement.OrganizationID != organizationID || statement.PeriodID != periodID || now.Unix() > statement.ExpiresAt ||
		(statement.Wish != enrollment.HolidayWishYes && statement.Wish != enrollment.HolidayWishNo) {
		return holidayWishProof{}, false
	}
	return statement, true
}

// holidayWishNamesChild reports whether a statement names the child as the
// parent sent it: the same school, name and date of birth. A parent who
// corrects the name keeps the registration but loses the statement; the
// office then checks the wish.
func holidayWishNamesChild(statement holidayWishProof, child enrollment.HolidayChildInput) bool {
	return child.HomeSchoolID == statement.SchoolID && child.BirthDate == statement.BirthDate &&
		sameHolidayName(child.FirstName, statement.FirstName) && sameHolidayName(child.LastName, statement.LastName)
}

func sameHolidayName(left, right string) bool {
	return strings.EqualFold(strings.Join(strings.Fields(left), " "), strings.Join(strings.Fields(right), " "))
}

// holidayWishMode reads the Träger's wish mode and refuses an unknown value.
func holidayWishMode(ctx context.Context, modes enrollment.HolidayWishModes, organizationID int64) (string, error) {
	if modes == nil {
		return "", errHolidayWishModesRequired
	}
	mode, err := modes.HolidayWishMode(ctx, organizationID)
	if err != nil {
		return "", fmt.Errorf("enrollment: read holiday wish mode: %w", err)
	}
	switch mode {
	case enrollment.HolidayWishModePriority, enrollment.HolidayWishModeWishOnly:
		return mode, nil
	default:
		return "", fmt.Errorf("enrollment: unknown holiday wish mode %q", mode)
	}
}

// offerHolidayWishMode is the rule the offer names: off, or the Träger's
// mode when the period takes the wish into account.
func (h *HolidayRegistrations) offerHolidayWishMode(ctx context.Context, organizationID int64, period enrollment.HolidayPeriod) (string, error) {
	if !period.HolidayWishRule {
		return enrollment.HolidayWishModeOff, nil
	}
	return holidayWishMode(ctx, h.deps.WishModes, organizationID)
}

// resolveHolidayWishes drops every wish a caller put in and keeps only what
// an opened statement says of the child it names. With the mode wish_only a
// child whose statement says no refuses the registration; only the parent
// who holds the statement can trigger that, so the answer never tells
// anyone else about a child of a school.
func (h *HolidayRegistrations) resolveHolidayWishes(ctx context.Context, link enrollment.HolidayRegistrationLink, period enrollment.HolidayPeriod,
	input enrollment.HolidayRegistrationInput) (enrollment.HolidayRegistrationInput, error) {
	declined := -1
	for i := range input.Children {
		child := &input.Children[i]
		proof := child.WishProof
		child.WishProof, child.HolidayWish, child.HolidayWishSource = "", "", ""
		if !period.HolidayWishRule || child.HomeSchoolID == 0 || proof == "" {
			continue
		}
		statement, ok := openHolidayWishProof(h.deps.Health, link.OrganizationID, link.PeriodID, proof, h.deps.Now())
		if !ok || !holidayWishNamesChild(statement, *child) {
			continue
		}
		child.HolidayWish, child.HolidayWishSource = statement.Wish, enrollment.HolidayWishSourceAccount
		if statement.Wish == enrollment.HolidayWishNo && declined < 0 {
			declined = i
		}
	}
	if declined < 0 {
		return input, nil
	}
	mode, err := holidayWishMode(ctx, h.deps.WishModes, link.OrganizationID)
	if err != nil {
		return input, err
	}
	if mode == enrollment.HolidayWishModeWishOnly {
		return input, fmt.Errorf("child %d: %w", declined, enrollment.ErrHolidayWishMissing)
	}
	return input, nil
}

// HolidayWishProofs serves enrollment.HolidayWishProofs.
type HolidayWishProofs struct {
	store  HolidayCareWishStore
	cipher enrollment.HolidayHealthCipher
	now    func() time.Time
}

var _ enrollment.HolidayWishProofs = (*HolidayWishProofs)(nil)

// NewHolidayWishProofs builds the statements of the prefill; a nil now uses
// the wall clock.
func NewHolidayWishProofs(store HolidayCareWishStore, cipher enrollment.HolidayHealthCipher, now func() time.Time) *HolidayWishProofs {
	if store == nil || cipher == nil {
		panic("enrollment: the holiday wish proofs need the care wish store and the cipher")
	}
	if now == nil {
		now = time.Now
	}
	return &HolidayWishProofs{store: store, cipher: cipher, now: now}
}

// HolidayWishProof reads the approved wish of the student for the school
// year of claim.On and seals it for the claim's link. An unknown wish has
// no statement.
func (p *HolidayWishProofs) HolidayWishProof(ctx context.Context, claim enrollment.HolidayWishClaim) (enrollment.HolidayWishStatement, error) {
	if claim.OrganizationID <= 0 || claim.PeriodID <= 0 || claim.SchoolID <= 0 || claim.StudentID <= 0 || claim.On.IsZero() {
		return enrollment.HolidayWishStatement{}, nil
	}
	answer, err := p.store.ApprovedHolidayCareWish(ctx, claim.StudentID, claim.On)
	if err != nil {
		return enrollment.HolidayWishStatement{}, fmt.Errorf("enrollment: read holiday care wish: %w", err)
	}
	if answer == nil {
		return enrollment.HolidayWishStatement{}, nil
	}
	wish := enrollment.HolidayWishNo
	if *answer {
		wish = enrollment.HolidayWishYes
	}
	proof, err := sealHolidayWishProof(p.cipher, claim, wish, p.now())
	if err != nil {
		return enrollment.HolidayWishStatement{}, err
	}
	return enrollment.HolidayWishStatement{Wish: wish, Proof: proof}, nil
}
