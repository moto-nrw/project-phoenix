package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func allowanceDay(id int64, day, entered string) AllowanceUse {
	return AllowanceUse{AbsenceID: id, Day: day, Days: 1, EnteredOn: entered}
}

func TestValidateCarryoverUntil(t *testing.T) {
	t.Parallel()
	for _, valid := range []string{"", "03-31", "01-01", "12-31", "02-28"} {
		assert.NoError(t, ValidateCarryoverUntil(valid), valid)
	}
	for _, invalid := range []string{"3-31", "03/31", "00-10", "13-01", "04-31", "02-29", "03-00", "ab-cd", "03-31x"} {
		assert.ErrorIs(t, ValidateCarryoverUntil(invalid), ErrAbsenceTypeInvalid, invalid)
	}
}

func TestAllowanceExpiresOn(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "2026-12-31", AllowanceExpiresOn(2026, ""))
	assert.Equal(t, "2027-03-31", AllowanceExpiresOn(2026, "03-31"))
}

// Swantjes Fall (#3257): Krank-Urlaubstage aus 2026 gelten bis 31.03.2027.
// Tage im Januar bis März gehen zuerst vom alten Rest ab.
func TestAllowanceLedgerUsesTheOlderRestFirst(t *testing.T) {
	t.Parallel()
	entitlements := map[int]float64{2026: 2, 2027: 5}
	uses := []AllowanceUse{
		allowanceDay(3, "2027-01-13", "2027-01-05"),
		allowanceDay(1, "2026-11-02", "2026-10-01"),
		allowanceDay(2, "2027-01-12", "2027-01-05"),
		allowanceDay(4, "2027-04-01", "2027-01-05"),
	}
	ledger := BuildAllowanceLedger(entitlements, "03-31", "2027-02-01", uses)

	old := ledger.Summary(1, 9, 2026)
	assert.Equal(t, 2.0, old.TakenDays, "November and the first January day use 2026")
	assert.Zero(t, old.RemainingDays)
	assert.Equal(t, "2027-03-31", old.ExpiresOn)
	assert.Equal(t, 1.0, ledger.Booked(2, 2026))
	assert.Zero(t, ledger.Booked(3, 2026), "the older rest is used up in calendar order")

	current := ledger.Summary(1, 9, 2027)
	assert.Equal(t, 2.0, current.TakenDays, "13 January and 1 April use 2027")
	assert.Equal(t, 3.0, current.RemainingDays)
	require.NotNil(t, current.CarriedIn)
	assert.Equal(t, AbsenceTypeAllowanceCarry{Year: 2026, ExpiresOn: "2027-03-31"}, *current.CarriedIn)
}

func TestAllowanceLedgerSplitsOneBookingOverTwoYears(t *testing.T) {
	t.Parallel()
	uses := []AllowanceUse{
		{AbsenceID: 5, Day: "2027-01-11", Days: 0.5, EnteredOn: "2027-01-04"},
		allowanceDay(5, "2027-01-12", "2027-01-04"),
		allowanceDay(5, "2027-01-13", "2027-01-04"),
	}
	ledger := BuildAllowanceLedger(map[int]float64{2026: 1, 2027: 3}, "03-31", "2027-01-04", uses)
	assert.Equal(t, []int{2026, 2027}, ledger.BookedYears(5))
	assert.Equal(t, 1.0, ledger.Booked(5, 2026))
	assert.Equal(t, 1.5, ledger.Booked(5, 2027))
}

func TestAllowanceLedgerExpiresTheRest(t *testing.T) {
	t.Parallel()
	entitlements := map[int]float64{2026: 10, 2027: 2}
	uses := []AllowanceUse{
		allowanceDay(1, "2027-03-31", "2027-03-01"),
		// Entered after the expiry: the old rest is gone, even for a past day.
		allowanceDay(2, "2027-03-30", "2027-04-02"),
		// After the expiry every day belongs to the new year.
		allowanceDay(3, "2027-04-01", "2027-03-01"),
	}

	before := BuildAllowanceLedger(entitlements, "03-31", "2027-03-31", uses)
	assert.Equal(t, 9.0, before.Summary(1, 9, 2026).RemainingDays, "the expiry day itself is still usable")
	assert.Zero(t, before.Summary(1, 9, 2026).ExpiredDays)

	after := BuildAllowanceLedger(entitlements, "03-31", "2027-04-02", uses)
	old := after.Summary(1, 9, 2026)
	assert.Zero(t, old.RemainingDays)
	assert.Equal(t, 9.0, old.ExpiredDays, "the rest is reported as expired, not dropped")
	assert.Equal(t, 1.0, old.TakenDays)
	current := after.Summary(1, 9, 2027)
	assert.Equal(t, 2.0, current.TakenDays)
	assert.Zero(t, current.RemainingDays)
	require.NotNil(t, current.CarriedIn)
	assert.Equal(t, 9.0, current.CarriedIn.ExpiredDays)
}

func TestAllowanceLedgerWithoutCarryover(t *testing.T) {
	t.Parallel()
	uses := []AllowanceUse{allowanceDay(1, "2027-01-12", "2027-01-05")}
	ledger := BuildAllowanceLedger(map[int]float64{2026: 3, 2027: 0}, "", "2027-01-12", uses)
	old := ledger.Summary(1, 9, 2026)
	assert.Equal(t, "2026-12-31", old.ExpiresOn)
	assert.Zero(t, old.RemainingDays)
	assert.Equal(t, 3.0, old.ExpiredDays)
	current := ledger.Summary(1, 9, 2027)
	assert.Equal(t, -1.0, current.RemainingDays, "a negative account stays visible")
	assert.Nil(t, current.CarriedIn)
}

func TestAllowanceLedgerCountsRequestsAsReserved(t *testing.T) {
	t.Parallel()
	uses := []AllowanceUse{
		{AbsenceID: 2, Day: "2027-01-12", Days: 1, Pending: true, EnteredOn: "2027-01-05"},
		allowanceDay(1, "2027-01-12", "2027-01-05"),
	}
	ledger := BuildAllowanceLedger(map[int]float64{2026: 1}, "03-31", "2027-01-05", uses)
	old := ledger.Summary(1, 9, 2026)
	assert.Equal(t, 1.0, old.TakenDays, "on the same day the booked entry comes first")
	assert.Zero(t, old.ReservedDays)
	assert.Equal(t, 1.0, ledger.Summary(1, 9, 2027).ReservedDays)
}

func TestAllowanceLedgerOverdrawn(t *testing.T) {
	t.Parallel()
	existing := []AllowanceUse{allowanceDay(1, "2027-02-01", "2027-01-05")}
	before := BuildAllowanceLedger(map[int]float64{2026: 1, 2027: 0}, "03-31", "2027-01-05", existing)
	assert.Empty(t, before.Overdrawn(before))

	// An earlier day now uses the old rest, so February falls to 2027.
	candidate := allowanceDay(0, "2027-01-11", "2027-01-05")
	after := BuildAllowanceLedger(map[int]float64{2026: 1, 2027: 0}, "03-31", "2027-01-05", append(existing, candidate))
	assert.Equal(t, []int{2027}, after.Overdrawn(before))

	// An account that was negative already does not block unrelated changes.
	negative := BuildAllowanceLedger(map[int]float64{2027: 0}, "", "2027-01-05", existing)
	assert.Empty(t, negative.Overdrawn(negative))
}
