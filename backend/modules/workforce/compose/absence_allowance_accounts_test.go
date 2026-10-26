package compose

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The staff member's own view (#4971) lists every active type with an
// allowance and shows the numbers the Leitung sees for the same person.
func TestAllowanceAccountsMatchTheSummaryOfEachType(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	january := calendar.NewDate(2027, time.January, 8).BerlinMidnight().Add(9 * time.Hour)
	svc := buildWorkforceAt(t, db, january)
	staff := testpkg.CreateTestStaff(t, db, "Rena", "Konten")
	other := testpkg.CreateTestStaff(t, db, "Olaf", "Andere")
	admin := testpkg.CreateTestStaff(t, db, "Swantje", "Konten")

	carry := carryoverType(t, svc, "Krank-Urlaubstag")
	plain, err := svc.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Regenerationstag", AllowanceEnabled: true})
	require.NoError(t, err)
	_, err = svc.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Ohne Kontingent"})
	require.NoError(t, err)
	retired, err := svc.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Alt", AllowanceEnabled: true})
	require.NoError(t, err)

	for _, claim := range []workforce.SetAbsenceTypeAllowance{
		{StaffID: staff.ID, AbsenceTypeID: carry.ID, Year: 2026, EntitledDays: 2},
		{StaffID: staff.ID, AbsenceTypeID: carry.ID, Year: 2027, EntitledDays: 1},
		{StaffID: staff.ID, AbsenceTypeID: plain.ID, Year: 2027, EntitledDays: 3},
		{StaffID: staff.ID, AbsenceTypeID: retired.ID, Year: 2027, EntitledDays: 5},
		{StaffID: other.ID, AbsenceTypeID: plain.ID, Year: 2027, EntitledDays: 9},
	} {
		claim.Reason, claim.ChangedBy = "Anspruch", admin.ID
		_, err := svc.SetAllowance(ctx, claim)
		require.NoError(t, err)
	}
	isActive := false
	_, err = svc.UpdateAbsenceType(ctx, workforce.UpdateAbsenceType{ID: retired.ID, IsActive: &isActive})
	require.NoError(t, err)

	bookAllowanceDay(t, svc, staff.ID, carry.ID, "2026-11-02", january)
	bookAllowanceDay(t, svc, staff.ID, carry.ID, "2027-01-11", january)
	bookAllowanceDay(t, svc, staff.ID, plain.ID, "2026-12-28", january)
	bookAllowanceDay(t, svc, staff.ID, plain.ID, "2027-01-12", january)
	bookAllowanceDay(t, svc, other.ID, plain.ID, "2027-01-13", january)
	_, err = svc.CreateStaffAbsence(ctx, &workforce.StaffAbsence{
		StaffID: staff.ID, CreatedBy: staff.ID, AbsenceType: workforce.AbsenceTypeOther, AbsenceTypeID: &plain.ID,
		Status: workforce.AbsenceStatusRequested, DateStart: "2027-01-14", DateEnd: "2027-01-14", HalfDay: true, CreatedAt: january,
	})
	require.NoError(t, err)

	accounts, err := svc.AllowanceAccounts(ctx, staff.ID, 2027)
	require.NoError(t, err)
	require.Len(t, accounts, 2, "inactive types and types without an allowance stay out")
	byName := map[string]workforce.AbsenceTypeAllowanceAccount{}
	for _, account := range accounts {
		byName[account.AbsenceType.Name] = account
	}
	for _, typ := range []workforce.StaffAbsenceType{carry, plain} {
		account, ok := byName[typ.Name]
		require.True(t, ok, typ.Name)
		assert.Equal(t, typ.ID, account.AbsenceType.ID)
		want, err := svc.AllowanceSummary(ctx, staff.ID, typ.ID, 2027)
		require.NoError(t, err)
		assert.Equal(t, want, account.Summary, typ.Name)
	}
	assert.Equal(t, 3.0, byName["Regenerationstag"].Summary.EntitledDays)
	assert.Equal(t, 1.0, byName["Regenerationstag"].Summary.TakenDays)
	assert.Equal(t, 0.5, byName["Regenerationstag"].Summary.ReservedDays)
	require.NotNil(t, byName["Krank-Urlaubstag"].Summary.CarriedIn)

	_, err = svc.AllowanceAccounts(ctx, staff.ID, 1999)
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeAllowanceInvalid)

	foreignID, _ := testpkg.CreateTestTenant(t, db)
	foreign, err := svc.AllowanceAccounts(testpkg.TenantContext(foreignID), staff.ID, 2027)
	require.NoError(t, err)
	assert.Empty(t, foreign, "another school sees none of these types")
}
