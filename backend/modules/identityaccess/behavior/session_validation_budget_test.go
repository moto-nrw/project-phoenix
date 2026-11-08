package behavior_test

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken/tokentest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// TestValidateSessionQueryBudget pins what the BFF's family check costs on
// every page view, per session kind: one administrative transaction that
// reads the family, the account and the kind's facts once, plus the module
// booking for the school and parents portals. Validation writes nothing, so
// it takes no row lock.
func TestValidateSessionQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	config := authTestFactoryConfig(false)
	identityModule, err := api.NewTestIdentityAccessWithConfig(testpkg.NewDatabase(t, db), slog.Default(), config)
	require.NoError(t, err)
	service := identityaccess.AccountAuthentication(identityModule)
	signer, err := sessiontoken.NewTokenAuthWithDurations(config.JWTSecret, config.JWTExpiry, config.JWTRefreshExpiry)
	require.NoError(t, err)
	owner, err := identityaccessCompose.New(identityaccessCompose.Dependencies{DB: db, TenantRuntime: testpkg.TenantRuntime(t, db), Observe: func(identityaccessCompose.Observation) {}})
	require.NoError(t, err)
	sessions := owner.(ports.Records)
	ctx := testpkg.Ctx(t)
	school := testpkg.Tenant(t)
	var organizationID int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, school).Scan(ctx, &organizationID))

	for kind, assertBudget := range map[string]func(t *testing.T, queries []string){
		"tenant": func(t *testing.T, q []string) {
			querybudgets.AssertQueryBudget(t, "identity.session_validate.tenant", q)
		},
		"school": func(t *testing.T, q []string) {
			querybudgets.AssertQueryBudget(t, "identity.session_validate.school", q)
		},
		"parent": func(t *testing.T, q []string) {
			querybudgets.AssertQueryBudget(t, "identity.session_validate.parent", q)
		},
		"caterer": func(t *testing.T, q []string) {
			querybudgets.AssertQueryBudget(t, "identity.session_validate.caterer", q)
		},
		"org": func(t *testing.T, q []string) { querybudgets.AssertQueryBudget(t, "identity.session_validate.org", q) },
		"traeger_school": func(t *testing.T, q []string) {
			querybudgets.AssertQueryBudget(t, "identity.session_validate.traeger_school", q)
		},
	} {
		t.Run(kind, func(t *testing.T) {
			account := testpkg.CreateTestAccount(t, db, "validate-budget")
			testpkg.EnsureAccountTenant(t, db, account.ID, school)
			accountID, portal := account.ID, kind
			claims := sessiontoken.AppClaims{Sub: account.Email, Roles: []string{"user"}, Scope: kind}
			row := ports.AccountSession{TenantID: school, PortalScope: kind}
			switch kind {
			case "tenant":
				assignSchoolSystemRole(t, db, accountID, school, "admin")
				claims.Scope, claims.TenantID = "", school
			case "school":
				assignSchoolSystemRole(t, db, accountID, school, "lehrkraft")
				claims.TenantID = school
			case "parent":
				assignSchoolSystemRole(t, db, accountID, school, "guardian")
			case "caterer":
				acceptance, _ := acceptedCatererAccount(t, db, identityModule, school, "validate-budget")
				accountID = acceptance.AccountID
			case "org":
				testpkg.CreateTestTraegerMember(t, db, organizationID, accountID, identityaccess.TraegerRoleSchoolAccess)
				claims.OrgID = organizationID
				row.TenantID, row.OrganizationID = 0, organizationID
			case "traeger_school":
				testpkg.CreateTestTraegerMember(t, db, organizationID, accountID, identityaccess.TraegerRoleSchoolAccess)
				portal = "tenant"
				claims.Scope, claims.TenantID, claims.OrgID, claims.ViaTraeger = "", school, organizationID, true
				row.PortalScope, row.OrganizationID = "tenant", organizationID
			}
			row.AccountID = accountID
			row.Token = fmt.Sprintf("validate-budget-%s-%d", kind, time.Now().UnixNano())
			row.FamilyID = row.Token + "-family"
			row.Expiry = time.Now().Add(time.Hour)
			rowCtx := ctx
			if row.TenantID == 0 {
				// A Träger row names no school.
				rowCtx = context.Background()
			}
			_, err := sessions.CreateAccountSession(rowCtx, row)
			require.NoError(t, err)
			claims.ID, claims.FamilyID = int(accountID), row.FamilyID
			access, _, err := tokentest.GenTokenPair(signer, claims, tokentest.RefreshClaims{ID: int(accountID), Token: row.Token, Scope: claims.Scope, TenantID: claims.TenantID})
			require.NoError(t, err)

			counter := testpkg.CaptureQueriesForContext(t, db)
			run := func() []string {
				counter.Reset()
				_, err := service.ValidateSessionTokens(counter.Context(context.Background()), access, "", portal)
				require.NoError(t, err)
				return counter.Queries()
			}
			first := run()
			again := run()
			assert.Len(t, again, len(first), "validation reads the same facts every time")
			for _, statement := range again {
				upper := strings.ToUpper(statement)
				assert.NotContains(t, upper, "FOR SHARE", "validation takes no row lock")
				assert.NotContains(t, upper, "FOR UPDATE", "validation takes no row lock")
			}
			assertBudget(t, again)
		})
	}
}
