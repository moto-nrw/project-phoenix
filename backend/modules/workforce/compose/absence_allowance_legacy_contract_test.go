package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomAbsenceAllowanceSummarizesOnlyItsOwnType(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Rena", "Generation")
	admin := testpkg.CreateTestStaff(t, db, "Lea", "Leitung")

	svc := buildWorkforce(t, db)

	absenceType, err := svc.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Regenerationstag"})
	require.NoError(t, err)
	enabled := true
	_, err = svc.UpdateAbsenceType(ctx, workforce.UpdateAbsenceType{ID: absenceType.ID, AllowanceEnabled: &enabled})
	require.NoError(t, err)

	summary, err := svc.SetAllowance(ctx, workforce.SetAbsenceTypeAllowance{
		StaffID:       staff.ID,
		AbsenceTypeID: absenceType.ID,
		Year:          2026,
		EntitledDays:  2,
		Reason:        "Tariflicher Anspruch",
		ChangedBy:     admin.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, 2.0, summary.EntitledDays)
	assert.Zero(t, summary.TakenDays)
	assert.Zero(t, summary.ReservedDays)
	assert.Equal(t, 2.0, summary.RemainingDays)

	summary, err = svc.SetAllowance(ctx, workforce.SetAbsenceTypeAllowance{
		StaffID: staff.ID, AbsenceTypeID: absenceType.ID, Year: 2026,
		EntitledDays: 2.5, Reason: "Tarifliche Korrektur", ChangedBy: admin.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, 2.5, summary.EntitledDays)
	changes := readAllowanceAudit(t, db, staff.ID, absenceType.ID)
	var correction *allowanceAuditRow
	for _, change := range changes {
		if change.Reason == "Tarifliche Korrektur" {
			correction = &change
			break
		}
	}
	require.NotNil(t, correction)
	require.NotNil(t, correction.OldEntitledDays)
	assert.Equal(t, 2.0, *correction.OldEntitledDays)
	assert.Equal(t, 2.5, correction.NewEntitledDays)

	typeID := absenceType.ID
	for _, absence := range []workforce.StaffAbsence{
		{
			StaffID: staff.ID, AbsenceType: workforce.AbsenceTypeOther,
			AbsenceTypeID: &typeID, Status: workforce.AbsenceStatusReported,
			DateStart: timezone.NewDate(2026, 9, 7).String(), DateEnd: timezone.NewDate(2026, 9, 7).String(),
			HalfDay: true, CreatedBy: admin.ID,
		},
		{
			StaffID: staff.ID, AbsenceType: workforce.AbsenceTypeOther,
			AbsenceTypeID: &typeID, Status: workforce.AbsenceStatusRequested,
			DateStart: timezone.NewDate(2026, 10, 5).String(), DateEnd: timezone.NewDate(2026, 10, 5).String(),
			CreatedBy: staff.ID,
		},
		{
			StaffID: staff.ID, AbsenceType: workforce.AbsenceTypeVacation,
			Status:    workforce.AbsenceStatusApproved,
			DateStart: timezone.NewDate(2026, 11, 2).String(), DateEnd: timezone.NewDate(2026, 11, 2).String(),
			CreatedBy: staff.ID,
		},
	} {
		_, err := svc.CreateStaffAbsence(ctx, absence)
		require.NoError(t, err)
	}

	summary, err = svc.AllowanceSummary(ctx, staff.ID, absenceType.ID, 2026)
	require.NoError(t, err)
	assert.Equal(t, 0.5, summary.TakenDays)
	assert.Equal(t, 1.0, summary.ReservedDays)
	assert.Equal(t, 1.0, summary.RemainingDays)
}
