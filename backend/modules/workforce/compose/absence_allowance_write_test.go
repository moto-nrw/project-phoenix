package compose

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type allowanceAuditRow struct {
	OldEntitledDays *float64
	NewEntitledDays float64
	Reason          string
	ChangedBy       int64
	CreatedAt       time.Time
}

func readAllowanceAudit(t *testing.T, db *bun.DB, staffID, typeID int64) []allowanceAuditRow {
	t.Helper()
	var rows []allowanceAuditRow
	require.NoError(t, db.NewSelect().TableExpr("active.staff_absence_type_allowance_changes").
		Column("old_entitled_days", "new_entitled_days", "reason", "changed_by", "created_at").
		Where("tenant_id = ?", testpkg.Tenant(t)).Where("staff_id = ?", staffID).Where("absence_type_id = ?", typeID).
		Where("year = ?", 2026).OrderExpr("id ASC").Scan(testpkg.Ctx(t), &rows))
	return rows
}

func TestAllowanceWritesAuditAndRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	capability := buildWorkforce(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Claim", "Owner")
	actor := testpkg.CreateTestStaff(t, db, "Claim", "Editor")
	absenceType, err := capability.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Audited", AllowanceEnabled: true})
	require.NoError(t, err)
	input := workforce.SetAbsenceTypeAllowance{StaffID: staff.ID, AbsenceTypeID: absenceType.ID, Year: 2026, Reason: "  Initial claim  ", ChangedBy: actor.ID}
	summary, err := capability.SetAllowance(ctx, input)
	require.NoError(t, err)
	assert.Zero(t, summary.EntitledDays)
	rows := readAllowanceAudit(t, db, staff.ID, absenceType.ID)
	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].OldEntitledDays)
	assert.Equal(t, "Initial claim", rows[0].Reason)
	assert.Equal(t, actor.ID, rows[0].ChangedBy)
	assert.False(t, rows[0].CreatedAt.IsZero())

	input.EntitledDays = 2.5
	_, err = capability.SetAllowance(ctx, input)
	require.NoError(t, err)
	rows = readAllowanceAudit(t, db, staff.ID, absenceType.ID)
	require.Len(t, rows, 2)
	require.NotNil(t, rows[1].OldEntitledDays, "an existing zero claim is not a missing claim")
	assert.Zero(t, *rows[1].OldEntitledDays)
	assert.Equal(t, 2.5, rows[1].NewEntitledDays)

	foreignID, _ := testpkg.CreateTestTenant(t, db)
	foreignActor := testpkg.CreateTestStaffForTenant(t, db, foreignID, "Foreign", "Editor")
	input.EntitledDays, input.ChangedBy = 3.5, foreignActor.ID
	_, err = capability.SetAllowance(ctx, input)
	require.Error(t, err, "the audit actor's tenant FK must reject the second write")
	assert.Contains(t, err.Error(), "audit absence type allowance")
	summary, err = capability.AllowanceSummary(ctx, staff.ID, absenceType.ID, 2026)
	require.NoError(t, err)
	assert.Equal(t, 2.5, summary.EntitledDays, "failed auditing must roll back the claim update")
	assert.Equal(t, rows, readAllowanceAudit(t, db, staff.ID, absenceType.ID))
	_, err = capability.SetAllowance(testpkg.TenantContext(foreignID), input)
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeNotFound)

	input.ChangedBy, input.EntitledDays = actor.ID, 1.25
	_, err = capability.SetAllowance(ctx, input)
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeAllowanceInvalid)
	assert.Equal(t, rows, readAllowanceAudit(t, db, staff.ID, absenceType.ID))
}

func TestCustomAbsenceAllowanceSerializesConcurrentCorrections(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	capability := buildWorkforce(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Concurrent", "Claim")
	actor := testpkg.CreateTestStaff(t, db, "Lea", "Parallel")
	absenceType, err := capability.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Concurrent", AllowanceEnabled: true, OverrunPolicy: workforce.AbsenceTypeOverrunBlock})
	require.NoError(t, err)
	input := workforce.SetAbsenceTypeAllowance{StaffID: staff.ID, AbsenceTypeID: absenceType.ID, Year: 2026, EntitledDays: 1, Reason: "Ausgangswert", ChangedBy: actor.ID}
	_, err = capability.SetAllowance(ctx, input)
	require.NoError(t, err)
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, days := range []float64{2, 3} {
		go func() {
			<-start
			request := input
			request.EntitledDays = days
			request.Reason = "Parallele Korrektur"
			results <- testpkg.WithinTenantContext(t, context.Background(), db, testpkg.Tenant(t), func(txCtx context.Context) error {
				_, err := capability.SetAllowance(txCtx, request)
				return err
			})
		}()
	}
	close(start)
	first, second := <-results, <-results
	require.NoError(t, first)
	require.NoError(t, second)
	rows := readAllowanceAudit(t, db, staff.ID, absenceType.ID)
	require.Len(t, rows, 3)
	require.NotNil(t, rows[1].OldEntitledDays)
	require.NotNil(t, rows[2].OldEntitledDays)
	assert.Equal(t, 1.0, *rows[1].OldEntitledDays)
	assert.Equal(t, rows[1].NewEntitledDays, *rows[2].OldEntitledDays)
}
