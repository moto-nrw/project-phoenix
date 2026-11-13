package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every account session, in every portal, is minted after the last-login
// stamp found the account alive in the mint transaction (#5546): an account
// the erasure of unused accounts anonymized after the caller's checks gets
// no session, whatever the scope and whether a guard ran.
func TestSessionMintRefusesAnAccountErasedAfterItsChecks(t *testing.T) {
	t.Parallel()
	for _, portal := range []domain.Portal{domain.PortalTenant, domain.PortalParent, domain.PortalSchool, domain.PortalCaterer, domain.PortalOrg} {
		t.Run("portal "+portal.Name, func(t *testing.T) {
			t.Parallel()
			f := newAuthFixture(t)
			f.auth.store, f.auth.caterers, f.auth.traeger = admitEveryPortal(f, 1, 10, 7), admittedCaterer{}, validationMemberships{}
			f.store.erasedBeforeMint = true
			tenantID, orgID := int64(10), int64(0)
			if portal.Name == domain.PortalOrg.Name {
				tenantID, orgID = 0, 7
			}
			_, err := f.auth.mintAccountSession(context.Background(), mintRequest{
				portal: portal, account: f.store.accounts[1], tenantID: tenantID, orgID: orgID, booked: []int64{10},
			})
			require.ErrorIs(t, err, domain.ErrAccountNotFound)
			assert.Empty(t, f.store.sessionsOf(1), "no session for the erased account")
			assert.NotContains(t, f.store.calls, "InsertAccountSession")
		})
	}
}

// The password logins end like an unknown account, which their routes render
// as wrong credentials, and write no session.
func TestLoginsRefuseAnAccountErasedAfterTheCredentialCheck(t *testing.T) {
	t.Parallel()
	for name, login := range map[string]func(f *authFixture) error{
		"tenant": func(f *authFixture) error {
			_, _, err := tenantPasswordLogin(f.auth, context.Background(), "user1@example.com", "secret", "192.0.2.1", "tests")
			return err
		},
		"parent": func(f *authFixture) error {
			_, _, err := f.auth.loginParentWithAudit(context.Background(), "user2@example.com", "secret", "192.0.2.1", "tests")
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newAuthFixture(t)
			f.seedStaff(1, 10, "user")
			f.seedStaff(2, 10, "guardian")
			f.store.erasedBeforeMint = true
			err := login(f)
			require.ErrorIs(t, err, domain.ErrAccountNotFound)
			assert.ErrorIs(t, authenticationError(err), identityaccess.ErrAccountNotFound, "the public error is the unknown account")
			assert.Empty(t, f.store.sessionsOf(1))
			assert.Empty(t, f.store.sessionsOf(2))
		})
	}
}

// The live-account check sits in authorizeMint, which only
// persistSessionInTransaction calls. The refresh rotation writes its
// successor after it locked the live account (lockAccount). No other code of
// the package writes an account session, so no mint path passes the check
// by (#5546).
func TestEveryAccountSessionIsWrittenBehindTheLiveAccountCheck(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	writers := map[string]int{}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := os.ReadFile(file)
		require.NoError(t, err)
		if count := strings.Count(string(source), "sessions.CreateAccountSession("); count > 0 {
			writers[file] = count
		}
	}
	assert.Equal(t, map[string]int{"session_mint.go": 1, "session_refresh.go": 1}, writers)

	persist := functionSource(t, "session_mint.go", "func (s *AccountAuthentication) persistSessionInTransaction(")
	requireCalledBefore(t, persist, "s.authorizeMint(", "s.sessions.CreateAccountSession(",
		"the login mint checks the live account before it writes the session")
	// Both writers open their transaction through the one mint unit.
	assert.Contains(t, persist, "s.mintUnit(ctx, ", "the mint transaction opens through mintUnit")
	rotate := functionSource(t, "session_refresh.go", "func (s *AccountAuthentication) refreshSessionInTransaction(")
	assert.Contains(t, rotate, "s.mintUnit(ctx, rotated.run)", "the rotation transaction opens through mintUnit")
	ledger := functionSource(t, "account_login.go", "func (s *AccountAuthentication) logAuthEvent(")
	assert.Contains(t, ledger, "s.ledgerUnit(", "an authentication event opens its school transaction through ledgerUnit")
	refresh := functionSource(t, "session_refresh.go", "func (r *refreshRotation) run(")
	requireCalledBefore(t, refresh, "r.lockAccount(", "r.persist(",
		"the refresh rotation locks the live account before it writes the successor")
}

// requireCalledBefore requires that both calls appear in source and that
// check comes first. A missing call fails on its own, not as an index of -1
// that would pass the order check.
func requireCalledBefore(t *testing.T, source, check, write, message string) {
	t.Helper()
	checkAt := strings.Index(source, check)
	writeAt := strings.Index(source, write)
	require.GreaterOrEqual(t, checkAt, 0, "%s: %s is called", message, check)
	require.GreaterOrEqual(t, writeAt, 0, "%s: %s is called", message, write)
	assert.Less(t, checkAt, writeAt, message)
}

func functionSource(t *testing.T, file, signature string) string {
	t.Helper()
	source, err := os.ReadFile(file)
	require.NoError(t, err)
	text := string(source)
	start := strings.Index(text, signature)
	require.GreaterOrEqual(t, start, 0, signature)
	end := strings.Index(text[start:], "\n}\n")
	require.Positive(t, end, signature)
	return text[start : start+end]
}

// admittedStore holds what the guards of the caterer, Träger and Träger
// school sessions ask beyond the login fakes: a caterer, the Träger right
// "In OGS wechseln" and the school's admin role.
type admittedStore struct{ *fakeStore }

type admittedCaterer struct{ ports.CatererStore }

func (admittedCaterer) FindCatererOfAccount(context.Context, int64) (domain.Caterer, bool, domain.OperationStats, error) {
	return domain.Caterer{ID: 1}, true, stats(), nil
}

func (s admittedStore) ListTraegerRoles(context.Context, int64, int64) ([]string, domain.OperationStats, error) {
	return []string{identityaccess.TraegerRoleSchoolAccess}, stats(), nil
}

func (s admittedStore) FindTraegerSchoolRole(context.Context, string, int64) (domain.RoleAssignment, []string, bool, domain.OperationStats, error) {
	return domain.RoleAssignment{RoleID: 900, Name: domain.AdminRoleName, IsSystem: true}, nil, true, stats(), nil
}

// admitEveryPortal gives the account what the real guards of the tenant,
// parent and school sessions demand at the school of the Träger: a mapping
// with a staff, guardian and teacher role. The returned store adds the Träger
// right and the school's admin role; admittedCaterer the caterer.
func admitEveryPortal(f *authFixture, accountID, tenantID, organizationID int64) admittedStore {
	f.seedStaff(accountID, tenantID, "user")
	f.store.addMapping(accountID, tenantID,
		domain.RoleAssignment{RoleID: 101, Name: domain.GuardianRoleName},
		domain.RoleAssignment{RoleID: 102, Name: domain.LehrkraftRoleName, IsSystem: true})
	f.schools.add(tenantID, organizationID, fmt.Sprintf("school-%d", tenantID), true, false)
	return admittedStore{fakeStore: f.store}
}
