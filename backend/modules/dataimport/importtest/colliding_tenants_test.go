package integration

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestImportUpdateAndUpsertKeepCollidingNaturalKeysInTheirTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	type school struct {
		ctx                          context.Context
		tenantID, studentID, staffID int64
		city                         string
	}
	var schools []school
	for _, name := range []string{"school A", "school B"} {
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			student := testpkg.CreateTestStudent(t, db, "Nora", "Collision", "1A")
			actor := newImporter(t, db)
			schools = append(schools, school{ctx: testpkg.Ctx(t), tenantID: testpkg.Tenant(t), studentID: student.ID, staffID: actor.staffID})
		})
	}
	require.NotEqual(t, schools[0].tenantID, schools[1].tenantID)
	require.NotEqual(t, schools[0].studentID, schools[1].studentID)
	for _, mode := range []importModels.ImportMode{importModels.ImportModeUpdate, importModels.ImportModeUpsert} {
		for side := range schools {
			current := &schools[side]
			current.city = string(mode) + " " + []string{"Köln", "Bonn"}[side]
			require.NoError(t, testpkg.WithTenantTx(t, current.ctx, db, current.tenantID, func(ctx context.Context, tx bun.Tx) error {
				var bypass bool
				require.NoError(t, tx.NewRaw(`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(ctx, &bypass))
				require.False(t, bypass, "the import must use the app role, not a superuser")
				result, err := module.Import.Import(ctx, importModels.ImportRequest[importModels.StudentImportRow]{
					Rows: []importModels.StudentImportRow{{FirstName: "Nora", LastName: "Collision", SchoolClass: "1A", AddressCity: current.city, DataRetentionDays: 30}},
					Mode: mode, UserID: current.staffID, SkipInvalidRows: true,
				})
				require.NoError(t, err)
				requireNoRowErrors(t, result)
				require.Equal(t, 1, result.UpdatedCount)
				require.Zero(t, result.CreatedCount)
				var visible []int64
				require.NoError(t, tx.NewSelect().Table("users.student_profiles").Column("id").Where("id IN (?, ?)", schools[0].studentID, schools[1].studentID).Scan(ctx, &visible))
				require.Equal(t, []int64{current.studentID}, visible, "RLS isolates colliding rows without an explicit tenant predicate")
				count, err := tx.NewSelect().Table("users.student_profiles").Count(ctx)
				require.NoError(t, err)
				require.EqualValues(t, 1, count, "upsert must not duplicate a colliding natural key")
				return nil
			}))
			for _, school := range schools {
				var city string
				require.NoError(t, db.NewRaw(`SELECT COALESCE(address_city, '') FROM users.student_profiles WHERE tenant_id = ? AND id = ?`, school.tenantID, school.studentID).Scan(school.ctx, &city))
				require.Equal(t, school.city, city, "updating a natural-key collision must leave the other school's row unchanged")
			}
		}
	}
}
