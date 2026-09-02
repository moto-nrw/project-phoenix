package test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// CreateTestStaffAbsenceType creates an active school-defined absence type
// (Abwesenheitsart, #2403) with the given wording in the test's tenant.
func CreateTestStaffAbsenceType(tb testing.TB, db *bun.DB, name string) *active.StaffAbsenceType {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	absenceType := &active.StaffAbsenceType{Name: name, BaseType: active.AbsenceTypeOther, IsActive: true, OverrunPolicy: active.AbsenceTypeOverrunWarn}
	absenceType.SetTenantID(fixtureTenantID(tb))
	err := db.NewInsert().Model(absenceType).ModelTableExpr(`active.staff_absence_types`).Scan(ctx)
	require.NoError(tb, err, "Failed to create test staff absence type")
	return absenceType
}

// CreateTestStaffAbsenceToday creates an approved one-day absence of the
// given school-defined type for a staff member on the current Berlin
// calendar day (today's absence label is what the staff directory shows).
func CreateTestStaffAbsenceToday(tb testing.TB, db *bun.DB, staffID int64, absenceTypeID int64) *active.StaffAbsence {
	tb.Helper()
	day := timezone.TodayDate()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	absence := &active.StaffAbsence{
		StaffID:       staffID,
		AbsenceType:   active.AbsenceTypeOther,
		AbsenceTypeID: &absenceTypeID,
		DateStart:     day,
		DateEnd:       day,
		Status:        active.AbsenceStatusApproved,
		CreatedBy:     staffID,
	}
	absence.SetTenantID(fixtureTenantID(tb))
	err := db.NewInsert().Model(absence).ModelTableExpr(`active.staff_absences`).Scan(ctx)
	require.NoError(tb, err, "Failed to create test staff absence")
	return absence
}
