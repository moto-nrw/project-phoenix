package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllowanceSummaryNativeContract(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	capability := buildWorkforce(t, db)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Allowance", "Owner")
	absenceType, err := capability.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Custom", AllowanceEnabled: true})
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO active.staff_absence_type_allowances (tenant_id, staff_id, absence_type_id, year, entitled_days) VALUES (?, ?, ?, ?, ?)`, testpkg.Tenant(t), staff.ID, absenceType.ID, 2026, 4.5).Exec(ctx)
	require.NoError(t, err)
	for _, row := range []workforce.StaffAbsence{
		{DateStart: "2025-12-31", DateEnd: "2026-01-02", StartHalfDay: true, EndHalfDay: true, Status: workforce.AbsenceStatusApproved},
		{DateStart: "2026-01-05", DateEnd: "2026-01-05", HalfDay: true, Status: workforce.AbsenceStatusRequested},
		{DateStart: "2026-01-06", DateEnd: "2026-01-06", Status: workforce.AbsenceStatusCanceled},
		{DateStart: "2026-01-07", DateEnd: "2026-01-07", Status: workforce.AbsenceStatusDeclined},
	} {
		row.StaffID, row.CreatedBy, row.AbsenceTypeID = staff.ID, staff.ID, &absenceType.ID
		row.AbsenceType = workforce.AbsenceTypeOther
		_, err := capability.CreateStaffAbsence(ctx, row)
		require.NoError(t, err)
	}
	summary, err := capability.AllowanceSummary(ctx, staff.ID, absenceType.ID, 2026)
	require.NoError(t, err)
	assert.Equal(t, workforce.AbsenceTypeAllowanceSummary{
		StaffID: staff.ID, AbsenceTypeID: absenceType.ID, Year: 2026,
		EntitledDays: 4.5, TakenDays: 1.5, ReservedDays: 0.5, RemainingDays: 2.5,
	}, summary)
	previous, err := capability.AllowanceSummary(ctx, staff.ID, absenceType.ID, 2025)
	require.NoError(t, err)
	assert.Zero(t, previous.EntitledDays, "a missing claim means zero")
	assert.Equal(t, 0.5, previous.TakenDays)
	assert.Equal(t, -0.5, previous.RemainingDays)
	_, err = capability.AllowanceSummary(ctx, staff.ID, absenceType.ID, 1999)
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeAllowanceInvalid)
	foreignID, _ := testpkg.CreateTestTenant(t, db)
	_, err = capability.AllowanceSummary(testpkg.TenantContext(foreignID), staff.ID, absenceType.ID, 2026)
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeNotFound)
}
