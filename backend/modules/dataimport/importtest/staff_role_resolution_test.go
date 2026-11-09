package integration

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Role resolution of the staff validation (#5887). The expectations below
// are what the single read per row (FindSchoolRoleByName, FindRolePermissions)
// produced before the batch: this test ran green on that head unchanged.

// staffRoleOutcome is what one row of a staff upload observably got: the
// blocking error codes with their suggestions, or the role of its invitation.
type staffRoleOutcome struct {
	Errors []string
	RoleID int64
}

// namedTenantRole creates a role of the test's school and gives it exactly
// name; the fixture appends a suffix that the scenarios cannot use.
func namedTenantRole(t *testing.T, db *bun.DB, name string) int64 {
	t.Helper()
	role := testpkg.CreateTestRoleForTenant(t, db, "rolle", testpkg.Tenant(t))
	_, err := db.NewRaw(`UPDATE auth.roles SET name = ? WHERE id = ?`, name, role.ID).Exec(context.Background())
	require.NoError(t, err)
	return role.ID
}

func systemRoleID(t *testing.T, db *bun.DB, name string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`SELECT id FROM auth.roles WHERE tenant_id IS NULL AND name = ?`, name).Scan(context.Background(), &id))
	return id
}

// runStaffRoleUpload imports one row per role name with the importer's
// permissions and returns each row's outcome, keyed by the role cell.
func runStaffRoleUpload(t *testing.T, db *bun.DB, module api.ImportTestModule, actor importer, prefix string, permissions []string, roleNames []string) map[string]staffRoleOutcome {
	t.Helper()
	rows := make([]importModels.StaffImportRow, len(roleNames))
	for i, name := range roleNames {
		rows[i] = importModels.StaffImportRow{
			FirstName: fmt.Sprintf("%s%d", prefix, i), LastName: "Rolle", RoleName: name,
			Email: fmt.Sprintf("%s%d.rolle@example.test", strings.ToLower(prefix), i), PersonnelNumber: fmt.Sprintf("%s-%d", prefix, i),
		}
	}
	ctx := tenant.WithUnitOfWork(testpkg.Ctx(t), testpkg.TenantRuntime(t, db))
	ctx = importModels.ContextWithImporterPermissions(ctx, permissions)
	result, err := module.StaffImport.ImportBatches(ctx, importModels.ImportRequest[importModels.StaffImportRow]{
		Rows: rows, Mode: importModels.ImportModeCreate, UserID: actor.accountID, SkipInvalidRows: true,
	}, importModels.BatchAudit{EntityType: "staff", Filename: prefix + ".csv", AccountID: actor.accountID})
	require.NoError(t, err)

	outcomes := make(map[string]staffRoleOutcome, len(rows))
	for _, rowErrors := range result.Errors {
		var codes []string
		for _, e := range rowErrors.Errors {
			if e.Severity == importModels.ErrorSeverityError {
				codes = append(codes, e.Code+":"+strings.Join(e.Suggestions, "|"))
			}
		}
		if len(codes) > 0 {
			outcomes[roleNames[rowErrors.RowNumber-2]] = staffRoleOutcome{Errors: codes}
		}
	}
	var invitations []struct {
		Email  string `bun:"email"`
		RoleID int64  `bun:"role_id"`
	}
	require.NoError(t, db.NewRaw(`SELECT email, role_id FROM auth.invitation_tokens WHERE email LIKE ?`, strings.ToLower(prefix)+"%").Scan(context.Background(), &invitations))
	for _, invitation := range invitations {
		var index int
		_, err := fmt.Sscanf(strings.TrimPrefix(invitation.Email, strings.ToLower(prefix)), "%d", &index)
		require.NoError(t, err)
		outcomes[roleNames[index]] = staffRoleOutcome{RoleID: invitation.RoleID}
	}
	require.Len(t, outcomes, len(rows), "every row either fails or invites")
	return outcomes
}

// TestStaffImportResolvesRolesLikeTheSingleRoleRead pins name priority, alias
// fallback, assignability, grant checks and suggestions of the staff role
// validation. A tie of equal rank is decided by the single read's smallest ID.
func TestStaffImportResolvesRolesLikeTheSingleRoleRead(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewImportTestModuleWithAuth(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.ImportTestOptions{}, api.WithAuthTestMailer(testpkg.NewCapturingMailer()))
	require.NoError(t, err)
	actor := newImporter(t, db)

	care := namedTenantRole(t, db, "Betreuungskraft")
	admin := namedTenantRole(t, db, "Admin") // same key as the system role admin
	gast := namedTenantRole(t, db, "Gast")   // exact name before the alias Gast → guest
	tieLower := namedTenantRole(t, db, "Team")
	tieUpper := namedTenantRole(t, db, "TEAM")
	require.Less(t, tieLower, tieUpper)
	_, err = db.NewRaw(`UPDATE auth.roles SET description = description WHERE id = ?`, tieLower).Exec(t.Context())
	require.NoError(t, err)
	permission := testpkg.CreateTestPermission(t, db, "import-role-grant", "groups", "delete")
	_, err = db.NewRaw(`INSERT INTO auth.role_permissions (role_id, permission_id) VALUES (?, ?)`, care, permission.ID).Exec(context.Background())
	require.NoError(t, err)

	outcomes := runStaffRoleUpload(t, db, module, actor, "Resolve", []string{"admin:*"}, []string{
		"  betreuungskraft ", "admin", "Verwaltung", "gast", "Gast", "Betreuer", "Betreuung", "team",
		"Betreungskraft", "Eltern", "",
	})
	require.Equal(t, staffRoleOutcome{RoleID: care}, outcomes["  betreuungskraft "], "case and spaces resolve the school's role")
	require.Equal(t, staffRoleOutcome{RoleID: admin}, outcomes["admin"], "the school's role ranks before the system role of the same key")
	require.Equal(t, staffRoleOutcome{RoleID: admin}, outcomes["Verwaltung"], "the alias resolves its raw name with the same ranking")
	require.Equal(t, staffRoleOutcome{RoleID: gast}, outcomes["gast"], "the exact name ranks before the alias")
	require.Equal(t, staffRoleOutcome{RoleID: gast}, outcomes["Gast"])
	require.Equal(t, staffRoleOutcome{RoleID: systemRoleID(t, db, "user")}, outcomes["Betreuer"], "a legacy alias resolves the system role")
	require.Equal(t, staffRoleOutcome{RoleID: systemRoleID(t, db, "user")}, outcomes["Betreuung"])
	require.Equal(t, staffRoleOutcome{RoleID: tieLower}, outcomes["team"], "a tie of equal rank resolves to the smallest ID")
	require.Equal(t, staffRoleOutcome{Errors: []string{"role_not_found:Betreuungskraft"}}, outcomes["Betreungskraft"])
	require.Equal(t, staffRoleOutcome{Errors: []string{"role_not_assignable:"}}, outcomes["Eltern"])
	require.Equal(t, staffRoleOutcome{Errors: []string{"required:"}}, outcomes[""])

	// Without users:manage the grant check reads the role's permissions: the
	// role holding groups:delete may not be granted, one without may.
	plain := namedTenantRole(t, db, "Ohne Rechte")
	limited := runStaffRoleUpload(t, db, module, actor, "Grant", []string{"users:create"}, []string{"Betreuungskraft", "Ohne Rechte"})
	require.Equal(t, staffRoleOutcome{Errors: []string{"role_grant_not_permitted:"}}, limited["Betreuungskraft"])
	require.Equal(t, staffRoleOutcome{RoleID: plain}, limited["Ohne Rechte"])
}

// TestStaffRoleValidationFallsBackToTheRowReadWhenTheBulkReadFails injects a
// SQL error into the one permission read of the file. The validation
// transaction stays usable through the read's savepoint, and every row reads
// its permissions itself with the same outcome as without the failure.
func TestStaffRoleValidationFallsBackToTheRowReadWhenTheBulkReadFails(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	care := namedTenantRole(t, db, "Betreuungskraft")
	permission := testpkg.CreateTestPermission(t, db, "import-role-fallback", "groups", "delete")
	_, err := db.NewRaw(`INSERT INTO auth.role_permissions (role_id, permission_id) VALUES (?, ?)`, care, permission.ID).Exec(context.Background())
	require.NoError(t, err)
	namedTenantRole(t, db, "Ohne Rechte")
	rows := []importModels.StaffImportRow{
		{FirstName: "Eins", LastName: "Rolle", RoleName: "Betreuungskraft"},
		{FirstName: "Zwei", LastName: "Rolle", RoleName: "Ohne Rechte"},
		{FirstName: "Drei", LastName: "Rolle", RoleName: "Betreungskraft"},
		{FirstName: "Vier", LastName: "Rolle", RoleName: "Betreuer"},
	}

	validate := func(failBulk bool) ([]string, []int) {
		t.Helper()
		config, _ := newStammdatenStaffConfig(t, db, &staffInvitationMock{})
		var singleReads []int
		read := config.FindRolePermissionsByRoleIDs
		config.FindRolePermissionsByRoleIDs = func(ctx context.Context, ids []int64) (map[int64][]string, error) {
			if len(ids) == 1 {
				singleReads = append(singleReads, int(ids[0]))
			} else if failBulk {
				tx, ok := tenant.TransactionFromContext(ctx)
				require.True(t, ok)
				_, err := tx.(bun.IDB).NewRaw(`SELECT 1/0`).Exec(ctx)
				require.Error(t, err)
				return nil, err
			}
			return read(ctx, ids)
		}
		var outcome []string
		inTenantTx(t, db, func(ctx context.Context) error {
			ctx = importModels.ContextWithImporterPermissions(ctx, []string{"users:create"})
			require.NoError(t, config.PreloadReferenceData(ctx))
			batch := config.ValidateBatch(ctx, rows)
			for i := range rows {
				row := rows[i]
				errs := append(append([]importModels.ValidationError(nil), batch[i]...), config.Validate(ctx, &row)...)
				var codes []string
				for _, e := range errs {
					codes = append(codes, e.Code+":"+e.Message)
				}
				sort.Strings(codes)
				outcome = append(outcome, fmt.Sprintf("%s → role %d %v", row.RoleName, row.RoleID, codes))
			}
			// The transaction is still usable after the failed bulk read.
			_, err := tx(ctx).NewRaw(`SELECT 1`).Exec(ctx)
			return err
		})
		return outcome, singleReads
	}

	batched, batchedSingles := validate(false)
	fallback, fallbackSingles := validate(true)
	require.Empty(t, batchedSingles, "with the bulk read no row reads its permissions alone")
	require.Len(t, fallbackSingles, 3, "after the failed bulk read each of the three resolved rows reads its permissions alone")
	require.Equal(t, batched, fallback)
	require.Contains(t, batched[0], "role_grant_not_permitted")
	require.NotContains(t, batched[1], "role_grant_not_permitted")
}

func tx(ctx context.Context) bun.IDB {
	value, _ := tenant.TransactionFromContext(ctx)
	return value.(bun.IDB)
}
