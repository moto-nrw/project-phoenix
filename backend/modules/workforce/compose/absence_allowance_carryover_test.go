package compose

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stringPtr(value string) *string { return &value }

func carryoverType(t *testing.T, svc workforce.Capability, name string) workforce.StaffAbsenceType {
	t.Helper()
	typ, err := svc.CreateAbsenceType(testpkg.Ctx(t), workforce.CreateAbsenceType{
		Name: name, AllowanceEnabled: true, CarryoverUntil: "03-31",
	})
	require.NoError(t, err)
	require.Equal(t, "03-31", typ.CarryoverUntil)
	return typ
}

func bookAllowanceDay(t *testing.T, svc workforce.Capability, staffID, typeID int64, day string, enteredAt time.Time) {
	t.Helper()
	_, err := svc.CreateStaffAbsence(testpkg.Ctx(t), workforce.StaffAbsence{
		StaffID: staffID, CreatedBy: staffID, AbsenceType: workforce.AbsenceTypeOther, AbsenceTypeID: &typeID,
		Status: workforce.AbsenceStatusReported, DateStart: day, DateEnd: day, CreatedAt: enteredAt,
	})
	require.NoError(t, err)
}

func TestAbsenceTypeCarryoverRule(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := buildWorkforce(t, db)

	typ := carryoverType(t, svc, "Krank-Urlaubstag")
	listed, err := svc.ListStaffAbsenceTypes(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, "03-31", listed[0].CarryoverUntil)

	renamed, err := svc.UpdateAbsenceType(ctx, workforce.UpdateAbsenceType{ID: typ.ID, Name: stringPtr("Krank-Urlaub")})
	require.NoError(t, err)
	assert.Equal(t, "03-31", renamed.CarryoverUntil, "an update without the rule keeps it")

	moved, err := svc.UpdateAbsenceType(ctx, workforce.UpdateAbsenceType{ID: typ.ID, CarryoverUntil: stringPtr("05-31")})
	require.NoError(t, err)
	assert.Equal(t, "05-31", moved.CarryoverUntil)

	ended, err := svc.UpdateAbsenceType(ctx, workforce.UpdateAbsenceType{ID: typ.ID, CarryoverUntil: stringPtr("")})
	require.NoError(t, err)
	assert.Empty(t, ended.CarryoverUntil, "an empty rule lets the rest expire on 31.12.")

	for _, invalid := range []string{"02-29", "3-31", "13-01"} {
		_, err = svc.UpdateAbsenceType(ctx, workforce.UpdateAbsenceType{ID: typ.ID, CarryoverUntil: stringPtr(invalid)})
		require.ErrorIs(t, err, workforce.ErrAbsenceTypeInvalid, invalid)
	}
	_, err = svc.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Falsch", AllowanceEnabled: true, CarryoverUntil: "04-31"})
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeInvalid)
}

// Swantjes Fall (#3257): 10 Krank-Urlaubstage aus 2026 sind bis 31.03.2027
// nutzbar. Eine Buchung im Januar geht zuerst vom alten Rest ab.
func TestCustomAllowanceCarriesTheRestIntoTheNextYear(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	january := timezone.NewDate(2027, time.January, 8).BerlinMidnight().Add(9 * time.Hour)
	svc := buildWorkforceAt(t, db, january)
	staff := testpkg.CreateTestStaff(t, db, "Rena", "Übertrag")
	admin := testpkg.CreateTestStaff(t, db, "Swantje", "Leitung")
	typ := carryoverType(t, svc, "Krank-Urlaubstag")
	for year, days := range map[int]float64{2026: 2, 2027: 1} {
		_, err := svc.SetAllowance(ctx, workforce.SetAbsenceTypeAllowance{
			StaffID: staff.ID, AbsenceTypeID: typ.ID, Year: year, EntitledDays: days, Reason: "Anspruch", ChangedBy: admin.ID,
		})
		require.NoError(t, err)
	}
	bookAllowanceDay(t, svc, staff.ID, typ.ID, "2026-11-02", january)

	// Mo 11.01. bis Mi 13.01.2027: 1 Tag aus 2026, 2 Tage aus 2027 -> zu viel.
	preview, err := svc.PreviewAllowanceBooking(ctx, staff.ID, typ.ID, "2027-01-11", "2027-01-13", false)
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeAllowanceExceeded)
	require.Len(t, preview, 2)
	assert.Equal(t, 2026, preview[0].Year)
	assert.Equal(t, 1.0, preview[0].BookingDays)
	assert.Zero(t, preview[0].RemainingDays)
	assert.Equal(t, "2027-03-31", preview[0].ExpiresOn)
	assert.Equal(t, 2027, preview[1].Year)
	assert.Equal(t, 2.0, preview[1].BookingDays)
	assert.Equal(t, -1.0, preview[1].RemainingDays)

	// Mo 11.01. bis Di 12.01.2027 passt genau.
	preview, err = svc.PreviewAllowanceBooking(ctx, staff.ID, typ.ID, "2027-01-11", "2027-01-12", false)
	require.NoError(t, err)
	require.Len(t, preview, 2)
	assert.Equal(t, 1.0, preview[0].BookingDays)
	assert.Equal(t, 1.0, preview[1].BookingDays)
	bookAllowanceDay(t, svc, staff.ID, typ.ID, "2027-01-11", january)

	old, err := svc.AllowanceSummary(ctx, staff.ID, typ.ID, 2026)
	require.NoError(t, err)
	assert.Equal(t, workforce.AbsenceTypeAllowanceSummary{
		StaffID: staff.ID, AbsenceTypeID: typ.ID, Year: 2026,
		EntitledDays: 2, TakenDays: 2, RemainingDays: 0, ExpiresOn: "2027-03-31",
	}, old)
	current, err := svc.AllowanceSummary(ctx, staff.ID, typ.ID, 2027)
	require.NoError(t, err)
	assert.Zero(t, current.TakenDays)
	assert.Equal(t, 1.0, current.RemainingDays)
	require.NotNil(t, current.CarriedIn)
	assert.Equal(t, workforce.AbsenceTypeAllowanceCarry{Year: 2026, ExpiresOn: "2027-03-31"}, *current.CarriedIn)

	// Den Anspruch 2026 zu senken würde den Januartag auf 2027 schieben, und
	// dort ist nur noch Platz für diesen einen Tag.
	bookAllowanceDay(t, svc, staff.ID, typ.ID, "2027-02-01", january)
	_, err = svc.SetAllowance(ctx, workforce.SetAbsenceTypeAllowance{
		StaffID: staff.ID, AbsenceTypeID: typ.ID, Year: 2026, EntitledDays: 1, Reason: "Korrektur", ChangedBy: admin.ID,
	})
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeAllowanceInvalid)
	assert.Contains(t, err.Error(), "31.03.2027")
	assert.Contains(t, err.Error(), "für 2027")
}

func TestCustomAllowanceRestExpires(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	april := timezone.NewDate(2027, time.April, 6).BerlinMidnight().Add(9 * time.Hour)
	svc := buildWorkforceAt(t, db, april)
	staff := testpkg.CreateTestStaff(t, db, "Rena", "Verfall")
	admin := testpkg.CreateTestStaff(t, db, "Swantje", "Verfall")
	typ := carryoverType(t, svc, "Krank-Urlaubstag")
	_, err := svc.SetAllowance(ctx, workforce.SetAbsenceTypeAllowance{
		StaffID: staff.ID, AbsenceTypeID: typ.ID, Year: 2026, EntitledDays: 10, Reason: "Sommerferien krank", ChangedBy: admin.ID,
	})
	require.NoError(t, err)
	march := timezone.NewDate(2027, time.March, 1).BerlinMidnight()
	bookAllowanceDay(t, svc, staff.ID, typ.ID, "2027-03-05", march)

	old, err := svc.AllowanceSummary(ctx, staff.ID, typ.ID, 2026)
	require.NoError(t, err)
	assert.Equal(t, 1.0, old.TakenDays)
	assert.Zero(t, old.RemainingDays)
	assert.Equal(t, 9.0, old.ExpiredDays, "the rest is shown as expired")

	// Nach dem 31.03. ist der alte Rest nicht mehr buchbar, auch nicht
	// rückwirkend für einen Tag davor.
	preview, err := svc.PreviewAllowanceBooking(ctx, staff.ID, typ.ID, "2027-03-08", "2027-03-08", false)
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeAllowanceExceeded)
	require.Len(t, preview, 1)
	assert.Equal(t, 2027, preview[0].Year)
	assert.Equal(t, 1.0, preview[0].BookingDays)
	assert.Equal(t, -1.0, preview[0].RemainingDays)
}
