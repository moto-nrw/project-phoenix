package compose

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type storedGuardianChange struct {
	TenantID           int64     `bun:"tenant_id"`
	StudentID          int64     `bun:"student_id"`
	GuardianProfileID  int64     `bun:"guardian_profile_id"`
	ActorAccountID     *int64    `bun:"actor_account_id"`
	ActorNameSnapshot  *string   `bun:"actor_name_snapshot"`
	ActorEmailSnapshot *string   `bun:"actor_email_snapshot"`
	ChangeType         string    `bun:"change_type"`
	FieldName          string    `bun:"field_name"`
	OldValue           *string   `bun:"old_value"`
	NewValue           *string   `bun:"new_value"`
	ChangedAt          time.Time `bun:"changed_at"`
}

func guardianChangesFor(t *testing.T, db bun.IDB, ctx context.Context, studentID int64) []storedGuardianChange {
	t.Helper()
	var rows []storedGuardianChange
	err := db.NewSelect().Model(&rows).
		ModelTableExpr("audit.guardian_changes AS change").
		ColumnExpr("change.tenant_id, change.student_id, change.guardian_profile_id, change.actor_account_id, change.actor_name_snapshot, change.actor_email_snapshot, change.change_type, change.field_name, change.old_value, change.new_value, change.changed_at").
		Where("change.student_id = ?", studentID).
		OrderExpr("change.id").
		Scan(ctx)
	require.NoError(t, err)
	return rows
}

func stringPointer(value string) *string { return &value }

func TestGuardianChangeLogRecordsRowsInAmbientTenantTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudent(t, db, "Audit", "Guardian", "1a")
	guardian := testpkg.CreateTestGuardianProfile(t, db, "guardian-change")
	actor := testpkg.CreateTestAccount(t, db, "guardian-change-actor")
	actorID := actor.ID
	changes := []auditlog.GuardianChange{
		{
			StudentID: student.ID, GuardianProfileID: guardian.ID, ActorAccountID: &actorID,
			ActorNameSnapshot: stringPointer("Erika Muster"), ActorEmailSnapshot: stringPointer("erika@example.test"),
			ChangeType: auditlog.GuardianChangeTypePickup, FieldName: auditlog.GuardianFieldCanPickup,
			OldValue: stringPointer("false"), NewValue: stringPointer("true"),
		},
		{
			StudentID: student.ID, GuardianProfileID: guardian.ID,
			ChangeType: auditlog.GuardianChangeTypeContact, FieldName: auditlog.GuardianFieldPhones,
		},
	}
	log := NewGuardianChangeLog()
	before := time.Now().Add(-time.Minute)

	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, tenantID, func(ctx context.Context) error {
		return log.RecordGuardianChanges(ctx, changes)
	}))

	rows := guardianChangesFor(t, db, context.Background(), student.ID)
	require.Len(t, rows, 2)
	require.Equal(t, tenantID, rows[0].TenantID)
	require.Equal(t, guardian.ID, rows[0].GuardianProfileID)
	require.Equal(t, &actorID, rows[0].ActorAccountID)
	require.Equal(t, "Erika Muster", *rows[0].ActorNameSnapshot)
	require.Equal(t, "erika@example.test", *rows[0].ActorEmailSnapshot)
	require.Equal(t, "pickup", rows[0].ChangeType)
	require.Equal(t, "can_pickup", rows[0].FieldName)
	require.Equal(t, "false", *rows[0].OldValue)
	require.Equal(t, "true", *rows[0].NewValue)
	require.True(t, rows[0].ChangedAt.After(before), "changed_at comes from the database default")
	require.Equal(t, tenantID, rows[1].TenantID)
	require.Nil(t, rows[1].ActorAccountID)
	require.Nil(t, rows[1].ActorNameSnapshot)
	require.Nil(t, rows[1].ActorEmailSnapshot)
	require.Equal(t, "contact", rows[1].ChangeType)
	require.Equal(t, "phones", rows[1].FieldName)
	require.Nil(t, rows[1].OldValue)
	require.Nil(t, rows[1].NewValue)
}

func TestGuardianChangeLogEmptySliceIsNoOp(t *testing.T) {
	t.Parallel()
	log := NewGuardianChangeLog()
	require.NoError(t, log.RecordGuardianChanges(context.Background(), nil))
	require.NoError(t, log.RecordGuardianChanges(context.Background(), []auditlog.GuardianChange{}))
}

func TestGuardianChangeLogRefusesWithoutTenantTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Audit", "NoTx", "1a")
	guardian := testpkg.CreateTestGuardianProfile(t, db, "guardian-change-notx")
	changes := []auditlog.GuardianChange{{
		StudentID: student.ID, GuardianProfileID: guardian.ID,
		ChangeType: auditlog.GuardianChangeTypeContact, FieldName: auditlog.GuardianFieldEmail,
	}}
	log := NewGuardianChangeLog()

	require.Error(t, log.RecordGuardianChanges(testpkg.Ctx(t), changes), "tenant without transaction")
	require.Error(t, log.RecordGuardianChanges(context.Background(), changes), "neither tenant nor transaction")
	require.Empty(t, guardianChangesFor(t, db, context.Background(), student.ID))
}

func TestGuardianChangeLogRowsStayInsideTheirTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.Tenant(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantB)
	student := testpkg.CreateTestStudent(t, db, "Audit", "Tenant", "1a")
	guardian := testpkg.CreateTestGuardianProfile(t, db, "guardian-change-tenant")
	log := NewGuardianChangeLog()

	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, tenantA, func(ctx context.Context) error {
		return log.RecordGuardianChanges(ctx, []auditlog.GuardianChange{{
			StudentID: student.ID, GuardianProfileID: guardian.ID,
			ChangeType: auditlog.GuardianChangeTypePickup, FieldName: auditlog.GuardianFieldEmergencyContact,
			OldValue: stringPointer("true"), NewValue: stringPointer("false"),
		}})
	}))

	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, tenantB, func(ctx context.Context, tx bun.Tx) error {
		require.Empty(t, guardianChangesFor(t, tx, ctx, student.ID), "tenant B must not see tenant A's row")
		return nil
	}))
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, tenantA, func(ctx context.Context, tx bun.Tx) error {
		rows := guardianChangesFor(t, tx, ctx, student.ID)
		require.Len(t, rows, 1)
		require.Equal(t, tenantA, rows[0].TenantID)
		return nil
	}))
}
