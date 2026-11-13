package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

type fixedWishMode string

func (m fixedWishMode) HolidayWishMode(context.Context, int64) (string, error) { return string(m), nil }

type approvedWish struct{ wish *bool }

func (a approvedWish) ApprovedHolidayCareWish(context.Context, int64, calendar.Date) (*bool, error) {
	return a.wish, nil
}

var wishNow = time.Date(2027, 3, 1, 9, 0, 0, 0, time.UTC)

func wishRegistrations(mode string) *HolidayRegistrations {
	return &HolidayRegistrations{deps: HolidayRegistrationDependencies{
		Health: boundCipher{}, WishModes: fixedWishMode(mode), Now: func() time.Time { return wishNow },
	}}
}

var (
	wishLink   = enrollment.HolidayRegistrationLink{OrganizationID: 7, PeriodID: 70}
	wishPeriod = enrollment.HolidayPeriod{ID: 70, HolidayWishRule: true}
	wishClaim  = enrollment.HolidayWishClaim{
		OrganizationID: 7, PeriodID: 70, SchoolID: 21, StudentID: 11,
		FirstName: "Felix", LastName: "Schneider", BirthDate: calendar.NewDate(2018, 3, 1), On: calendar.NewDate(2027, 7, 5),
	}
)

// statement seals the prefill's statement of wishClaim with answer.
func statement(t *testing.T, claim enrollment.HolidayWishClaim, answer bool) string {
	t.Helper()
	proofs := NewHolidayWishProofs(approvedWish{wish: &answer}, boundCipher{}, func() time.Time { return wishNow })
	stated, err := proofs.HolidayWishProof(context.Background(), claim)
	require.NoError(t, err)
	require.NotEmpty(t, stated.Proof)
	return stated.Proof
}

func wishChild(proof string) enrollment.HolidayChildInput {
	return enrollment.HolidayChildInput{
		FirstName: "Felix", LastName: "Schneider", BirthDate: calendar.NewDate(2018, 3, 1), HomeSchoolID: 21, WishProof: proof,
	}
}

func resolveOne(t *testing.T, mode string, period enrollment.HolidayPeriod, child enrollment.HolidayChildInput) (enrollment.HolidayChildInput, error) {
	t.Helper()
	resolved, err := wishRegistrations(mode).resolveHolidayWishes(context.Background(), wishLink, period,
		enrollment.HolidayRegistrationInput{Children: []enrollment.HolidayChildInput{child}})
	return resolved.Children[0], err
}

// A wish counts only from a statement the prefill sealed for this link and
// this child (#4703). A caller cannot set it, and a statement of another
// period, school, child or one that expired is no statement: the child
// comes in with an unknown wish. Each case goes red when its check is gone.
func TestResolveHolidayWishesKeepsOnlyTheStatedWishOfTheNamedChild(t *testing.T) {
	t.Parallel()
	stated, err := resolveOne(t, enrollment.HolidayWishModePriority, wishPeriod, wishChild(statement(t, wishClaim, true)))
	require.NoError(t, err)
	assert.Equal(t, enrollment.HolidayWishYes, stated.HolidayWish)
	assert.Equal(t, enrollment.HolidayWishSourceAccount, stated.HolidayWishSource)
	assert.Empty(t, stated.WishProof, "the statement is not stored")

	forged := wishChild("")
	forged.HolidayWish, forged.HolidayWishSource = enrollment.HolidayWishYes, enrollment.HolidayWishSourceOffice
	got, err := resolveOne(t, enrollment.HolidayWishModePriority, wishPeriod, forged)
	require.NoError(t, err)
	assert.Empty(t, got.HolidayWish, "a wish the caller sets is dropped")
	assert.Empty(t, got.HolidayWishSource)

	otherPeriod := wishClaim
	otherPeriod.PeriodID = 71
	otherSchool := wishClaim
	otherSchool.SchoolID = 22
	otherChild := wishClaim
	otherChild.FirstName = "Mia"
	cases := map[string]enrollment.HolidayChildInput{
		"another period": wishChild(statement(t, otherPeriod, true)),
		"another school": wishChild(statement(t, otherSchool, true)),
		"another child":  wishChild(statement(t, otherChild, true)),
		"garbage":        wishChild("not-a-statement"),
	}
	for name, child := range cases {
		got, err := resolveOne(t, enrollment.HolidayWishModePriority, wishPeriod, child)
		require.NoError(t, err, name)
		assert.Empty(t, got.HolidayWish, name)
	}

	expired := wishRegistrations(enrollment.HolidayWishModePriority)
	expired.deps.Now = func() time.Time { return wishNow.Add(enrollment.HolidayWishProofLifetime + time.Minute) }
	resolved, err := expired.resolveHolidayWishes(context.Background(), wishLink, wishPeriod,
		enrollment.HolidayRegistrationInput{Children: []enrollment.HolidayChildInput{wishChild(statement(t, wishClaim, true))}})
	require.NoError(t, err)
	assert.Empty(t, resolved.Children[0].HolidayWish, "an expired statement counts no more")

	off := wishPeriod
	off.HolidayWishRule = false
	got, err = resolveOne(t, enrollment.HolidayWishModeWishOnly, off, wishChild(statement(t, wishClaim, false)))
	require.NoError(t, err, "a period without the rule refuses nobody")
	assert.Empty(t, got.HolidayWish, "and keeps no wish")

	guest := wishChild(statement(t, wishClaim, true))
	guest.HomeSchoolID = 0
	got, err = resolveOne(t, enrollment.HolidayWishModePriority, wishPeriod, guest)
	require.NoError(t, err)
	assert.Empty(t, got.HolidayWish, "a holiday guest has no wish")
}

// "Nur mit Ferienwunsch" refuses only the parent who brings a statement that
// says no (#4703). The same child without a statement is accepted with an
// unknown wish: the answer never depends on what a school knows about a
// child whose parent did not prove the relationship (ADR 0110 point 4).
func TestResolveHolidayWishesRefusesOnlyAStatedNo(t *testing.T) {
	t.Parallel()
	_, err := resolveOne(t, enrollment.HolidayWishModeWishOnly, wishPeriod, wishChild(statement(t, wishClaim, false)))
	require.True(t, errors.Is(err, enrollment.ErrHolidayWishMissing), "a stated no is refused: %v", err)

	got, err := resolveOne(t, enrollment.HolidayWishModeWishOnly, wishPeriod, wishChild(""))
	require.NoError(t, err, "without a statement nothing is looked up and nothing refused")
	assert.Empty(t, got.HolidayWish)

	got, err = resolveOne(t, enrollment.HolidayWishModePriority, wishPeriod, wishChild(statement(t, wishClaim, false)))
	require.NoError(t, err, "with precedence a stated no only waits behind")
	assert.Equal(t, enrollment.HolidayWishNo, got.HolidayWish)
}

// An unknown or unasked wish has no statement, and an unknown mode is an
// error, never a silent default.
func TestHolidayWishProofsAndModes(t *testing.T) {
	t.Parallel()
	proofs := NewHolidayWishProofs(approvedWish{}, boundCipher{}, nil)
	stated, err := proofs.HolidayWishProof(context.Background(), wishClaim)
	require.NoError(t, err)
	assert.Equal(t, enrollment.HolidayWishStatement{}, stated)

	_, err = holidayWishMode(context.Background(), fixedWishMode("sometimes"), 7)
	require.Error(t, err)
	mode, err := wishRegistrations(enrollment.HolidayWishModeWishOnly).offerHolidayWishMode(context.Background(), 7, enrollment.HolidayPeriod{})
	require.NoError(t, err)
	assert.Equal(t, enrollment.HolidayWishModeOff, mode, "a period without the rule names no mode")
}
