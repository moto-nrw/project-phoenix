package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #3256: a claim cannot shrink below the days already booked or requested,
// otherwise the Kontingent would show a negative rest.
func TestCustomAbsenceAllowanceCannotDropBelowUsedDays(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	svc := buildWorkforce(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Rena", "Untergrenze")
	admin := testpkg.CreateTestStaff(t, db, "Lea", "Untergrenze")
	typ, err := svc.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Regenerationstag", AllowanceEnabled: true})
	require.NoError(t, err)
	claim := workforce.SetAbsenceTypeAllowance{
		StaffID: staff.ID, AbsenceTypeID: typ.ID, Year: 2026,
		EntitledDays: 2, Reason: "Tarif", ChangedBy: admin.ID,
	}
	_, err = svc.SetAllowance(ctx, claim)
	require.NoError(t, err)
	_, err = svc.CreateStaffAbsence(ctx, workforce.StaffAbsence{
		StaffID: staff.ID, CreatedBy: admin.ID, AbsenceType: workforce.AbsenceTypeOther, AbsenceTypeID: &typ.ID,
		Status: workforce.AbsenceStatusReported, DateStart: "2026-09-07", DateEnd: "2026-09-07",
	})
	require.NoError(t, err)
	_, err = svc.CreateStaffAbsence(ctx, workforce.StaffAbsence{
		StaffID: staff.ID, CreatedBy: staff.ID, AbsenceType: workforce.AbsenceTypeOther, AbsenceTypeID: &typ.ID,
		Status: workforce.AbsenceStatusRequested, DateStart: "2026-09-08", DateEnd: "2026-09-08", HalfDay: true,
		StartHalfDay: true, EndHalfDay: true,
	})
	require.NoError(t, err)

	claim.EntitledDays = 1
	claim.Reason = "Teilzeit"
	_, err = svc.SetAllowance(ctx, claim)
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeAllowanceInvalid)
	assert.Contains(t, err.Error(), "1,5 Tage")

	claim.EntitledDays = 1.5
	summary, err := svc.SetAllowance(ctx, claim)
	require.NoError(t, err, "exactly the used days is allowed")
	assert.Zero(t, summary.RemainingDays)
}
