package compose_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Presence's own RLS suite covers attendance and visits. This matrix covers
// the other five tables named by #2701 without owner-level tenant predicates.
func TestClassDayInputsEnforceRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var contexts []context.Context
	var tenants []int64
	var fixtures []*mensaFixture
	var rows []map[string]int64
	parent := t
	for _, side := range []string{"own", "foreign"} {
		t.Run(side, func(t *testing.T) {
			testpkg.OwnTenant(t)
			ctx := testpkg.Ctx(t)
			fixture := buildMensaFixtureOn(t, db)
			group := testpkg.CreateTestEducationGroup(t, db, "RLS-"+side)
			entry := testpkg.CreateTestClassListEntryForTenant(parent, db, testpkg.Tenant(t), "RLS", side, "3a")
			var rosterID int64
			require.NoError(t, db.NewSelect().Table("schedule.instance_students").Column("id").Where("instance_id = ? AND student_id = ?", fixture.instanceID, fixture.plannedID).Scan(ctx, &rosterID))
			rows = append(rows, map[string]int64{"users.students": fixture.plannedID, "users.class_list_entries": entry.ID, "education.groups": group.ID, "schedule.activity_instances": fixture.instanceID, "schedule.instance_students": rosterID})
			contexts = append(contexts, ctx)
			tenants = append(tenants, testpkg.Tenant(t))
			fixtures = append(fixtures, fixture)
		})
	}
	for table, ownID := range rows[0] {
		t.Run(table, func(t *testing.T) {
			for side, ctx := range contexts {
				require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenants[side], func(txCtx context.Context, tx bun.Tx) error {
					var bypass bool
					require.NoError(t, tx.NewRaw("SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user").Scan(txCtx, &bypass))
					require.False(t, bypass)
					var visible []int64
					require.NoError(t, tx.NewSelect().Table(table).Column("id").Where("id IN (?, ?)", ownID, rows[1][table]).Scan(txCtx, &visible))
					require.Equal(t, []int64{rows[side][table]}, visible)
					return nil
				}))
			}
		})
	}
	for side, ctx := range contexts {
		result, err := fixtures[side].svc.BuildList(ctx, classday.Params{Date: classday.Date(listDate.String()), Target: classday.TargetSlots, Source: classday.SourceReconciliation})
		require.NoError(t, err)
		require.Len(t, result.Rows, 3, "both tenants have a populated roster and actual visits")
		require.Len(t, result.Slots, 1)
		require.Equal(t, fixtures[side].instanceID, result.Slots[0].InstanceID)
		require.NotNil(t, rowByStudent(result.Rows, fixtures[side].plannedID))
		for _, foreignID := range []int64{fixtures[1-side].plannedID, fixtures[1-side].missingID, fixtures[1-side].walkInID} {
			require.Nil(t, rowByStudent(result.Rows, foreignID))
		}
	}
}
