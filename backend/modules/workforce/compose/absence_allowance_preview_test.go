package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomAbsenceAllowanceOverrunPolicy(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := buildWorkforce(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Preview", "Owner")
	typ, err := svc.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Preview", AllowanceEnabled: true, OverrunPolicy: workforce.AbsenceTypeOverrunBlock})
	require.NoError(t, err)
	_, err = svc.SetAllowance(ctx, workforce.SetAbsenceTypeAllowance{StaffID: staff.ID, AbsenceTypeID: typ.ID, Year: 2026, EntitledDays: 0.5, Reason: "Initial claim", ChangedBy: staff.ID})
	require.NoError(t, err)
	preview, err := svc.PreviewAllowanceBooking(ctx, staff.ID, typ.ID, "2026-09-07", "2026-09-07", false)
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeAllowanceExceeded)
	require.Len(t, preview, 1)
	assert.Equal(t, -0.5, preview[0].RemainingDays)
	warn := workforce.AbsenceTypeOverrunWarn
	_, err = svc.UpdateAbsenceType(ctx, workforce.UpdateAbsenceType{ID: typ.ID, OverrunPolicy: &warn})
	require.NoError(t, err)
	preview, err = svc.PreviewAllowanceBooking(ctx, staff.ID, typ.ID, "2026-09-07", "2026-09-07", false)
	require.NoError(t, err)
	require.Len(t, preview, 1)
	assert.Equal(t, -0.5, preview[0].RemainingDays)
	block := workforce.AbsenceTypeOverrunBlock
	_, err = svc.UpdateAbsenceType(ctx, workforce.UpdateAbsenceType{ID: typ.ID, OverrunPolicy: &block})
	require.NoError(t, err)
	_, err = svc.CreateStaffAbsence(ctx, workforce.StaffAbsence{StaffID: staff.ID, CreatedBy: staff.ID, AbsenceType: workforce.AbsenceTypeOther, AbsenceTypeID: &typ.ID, Status: workforce.AbsenceStatusReported, DateStart: "2026-09-07", DateEnd: "2026-09-07", HalfDay: true})
	require.NoError(t, err)
	preview, err = svc.PreviewAllowanceBooking(ctx, staff.ID, typ.ID, "2026-09-07", "2026-09-07", true)
	require.NoError(t, err)
	require.Len(t, preview, 1)
	assert.Zero(t, preview[0].RemainingDays, "a repeated half-day adds no coverage")
	preview, err = svc.PreviewAllowanceBooking(ctx, staff.ID, typ.ID, "2026-09-07", "2026-09-07", false)
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeAllowanceExceeded)
	require.Len(t, preview, 1)
	assert.Equal(t, -0.5, preview[0].RemainingDays, "a full-day correction adds only half a day")
	for _, dates := range [][2]string{{"", "2026-09-07"}, {"2026-02-30", "2026-09-07"}, {"2026-09-08", "2026-09-07"}} {
		preview, err = svc.PreviewAllowanceBooking(ctx, staff.ID, typ.ID, dates[0], dates[1], false)
		require.ErrorIs(t, err, workforce.ErrAbsenceTypeAllowanceInvalid)
		assert.Nil(t, preview)
	}
}

func TestCustomAbsenceAllowanceDoesNotCarryAcrossYears(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := buildWorkforce(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Rena", "Jahreswechsel")
	admin := testpkg.CreateTestStaff(t, db, "Lea", "Jahreswechsel")
	typ, err := svc.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Gesundheitstag", AllowanceEnabled: true, OverrunPolicy: workforce.AbsenceTypeOverrunBlock})
	require.NoError(t, err)
	for _, year := range []int{2026, 2027} {
		_, err = svc.SetAllowance(ctx, workforce.SetAbsenceTypeAllowance{
			StaffID: staff.ID, AbsenceTypeID: typ.ID, Year: year,
			EntitledDays: 1, Reason: "Jahresanspruch", ChangedBy: admin.ID,
		})
		require.NoError(t, err)
	}
	preview, err := svc.PreviewAllowanceBooking(ctx, staff.ID, typ.ID, "2026-12-31", "2027-01-01", false)
	require.NoError(t, err)
	require.Len(t, preview, 2)
	assert.Equal(t, 2026, preview[0].Year)
	assert.Zero(t, preview[0].RemainingDays)
	assert.Equal(t, 2027, preview[1].Year)
	assert.Zero(t, preview[1].RemainingDays)
}

func TestAllowancePreviewDisabledAndForeignTypes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := buildWorkforce(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Preview", "Tenant")
	typ, err := svc.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "No allowance"})
	require.NoError(t, err)
	preview, err := svc.PreviewAllowanceBooking(ctx, staff.ID, typ.ID, "2026-12-31", "2027-01-01", false)
	require.NoError(t, err)
	assert.Nil(t, preview, "disabled allowances impose no booking limit")
	foreignID, _ := testpkg.CreateTestTenant(t, db)
	preview, err = svc.PreviewAllowanceBooking(testpkg.TenantContext(foreignID), staff.ID, typ.ID, "2026-12-31", "2027-01-01", false)
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeNotFound)
	assert.Nil(t, preview)
	enabled, block := true, workforce.AbsenceTypeOverrunBlock
	_, err = svc.UpdateAbsenceType(ctx, workforce.UpdateAbsenceType{ID: typ.ID, AllowanceEnabled: &enabled, OverrunPolicy: &block})
	require.NoError(t, err)
	preview, err = svc.PreviewAllowanceBooking(ctx, staff.ID, typ.ID, "2026-12-31", "2027-01-01", false)
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeAllowanceExceeded)
	require.Len(t, preview, 2, "a blocked year must not suppress later yearly previews")
	assert.Equal(t, -1.0, preview[0].RemainingDays)
	assert.Equal(t, -1.0, preview[1].RemainingDays)
}
