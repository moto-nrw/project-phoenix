package test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// StaffAbsenceTypeFixture is test-owned setup data for a school-defined
// absence type row (active.staff_absence_types).
type StaffAbsenceTypeFixture struct {
	ID               int64 `bun:"id,pk,autoincrement"`
	TenantID         int64
	Name             string
	BaseType         string
	IsActive         bool `bun:"is_active,notnull"`
	AllowanceEnabled bool `bun:"allowance_enabled,notnull"`
}

// StaffAbsenceFixture is test-owned setup data for a staff absence row
// (active.staff_absences).
type StaffAbsenceFixture struct {
	ID            int64 `bun:"id,pk,autoincrement"`
	TenantID      int64
	StaffID       int64
	AbsenceType   string
	AbsenceTypeID *int64
	DateStart     timezone.Date `bun:"date_start,notnull,type:date"`
	DateEnd       timezone.Date `bun:"date_end,notnull,type:date"`
	Status        string
	CreatedBy     int64
}

// CreateTestStaffAbsenceType creates an active school-defined absence type
// (Abwesenheitsart, #2403) with the given wording in the test's tenant.
func CreateTestStaffAbsenceType(tb testing.TB, db *bun.DB, name string) *StaffAbsenceTypeFixture {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	absenceType := &StaffAbsenceTypeFixture{TenantID: fixtureTenantID(tb), Name: name, BaseType: workforce.AbsenceTypeOther, IsActive: true}
	err := db.NewInsert().Model(absenceType).ModelTableExpr(`active.staff_absence_types`).Scan(ctx)
	require.NoError(tb, err, "Failed to create test staff absence type")
	return absenceType
}

// CreateTestStaffAbsenceToday creates an approved one-day absence of the
// given school-defined type for a staff member on the current Berlin
// calendar day (today's absence label is what the staff directory shows).
func CreateTestStaffAbsenceToday(tb testing.TB, db *bun.DB, staffID int64, absenceTypeID int64) *StaffAbsenceFixture {
	tb.Helper()
	day := timezone.TodayDate()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	absence := &StaffAbsenceFixture{
		TenantID:      fixtureTenantID(tb),
		StaffID:       staffID,
		AbsenceType:   workforce.AbsenceTypeOther,
		AbsenceTypeID: &absenceTypeID,
		DateStart:     day,
		DateEnd:       day,
		Status:        workforce.AbsenceStatusApproved,
		CreatedBy:     staffID,
	}
	err := db.NewInsert().Model(absence).ModelTableExpr(`active.staff_absences`).Scan(ctx)
	require.NoError(tb, err, "Failed to create test staff absence")
	return absence
}
