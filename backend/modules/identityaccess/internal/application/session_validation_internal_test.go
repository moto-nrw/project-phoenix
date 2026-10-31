package application

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/require"
)

// Validation must distinguish a failed read from confirmed missing access,
// including after the family lookup has already succeeded.
type validationStore struct {
	*fakeStore
	accountSessionErr error
	rolesErr          error
}

func (s validationStore) FindAccountSessionByToken(ctx context.Context, token string, update bool) (domain.AccountSession, bool, domain.OperationStats, error) {
	if s.accountSessionErr != nil {
		return domain.AccountSession{}, false, stats(), s.accountSessionErr
	}
	return s.fakeStore.FindAccountSessionByToken(ctx, token, update)
}

func (s validationStore) ListTraegerRoles(context.Context, int64, int64) ([]string, domain.OperationStats, error) {
	return []string{identityaccess.TraegerRoleAdministration}, stats(), s.rolesErr
}

type validationMemberships struct {
	err    error
	absent bool
}

func (s validationMemberships) ListAccountTraegerOrganizations(context.Context, int64) ([]identityaccess.TraegerOrganization, error) {
	if s.absent {
		return nil, nil
	}
	return []identityaccess.TraegerOrganization{{ID: 7}}, s.err
}

func TestValidateSessionTokensPreservesLookupErrors(t *testing.T) {
	t.Parallel()
	for _, lookup := range []string{"account", "operator", "mapping", "membership", "switch-membership", "switch-roles", "school", "account-session", "operator-session"} {
		t.Run(lookup, func(t *testing.T) {
			t.Parallel()
			for _, withRefresh := range []bool{false, true} {
				if !withRefresh && (lookup == "account-session" || lookup == "operator-session") {
					continue
				}
				f := newAuthFixture(t)
				f.seedStaff(1, 10, "user")
				f.store.addOperator(1, "operator@example.com", "unused", true)
				f.schools.add(10, 7, "school-10", true, false)
				store := validationStore{fakeStore: f.store}
				f.auth.traeger = validationMemberships{}
				claims := domain.SessionClaims{AccountID: 1, TenantID: 10, FamilyID: "family", ExpiresAt: time.Now().Add(time.Hour).Unix()}
				portal := "tenant"
				switch lookup {
				case "account":
					f.store.findAccountErr = errDB
				case "operator", "operator-session":
					portal, claims.Scope, claims.TenantID = "platform", "platform", 0
					if lookup == "operator" {
						f.store.findOperatorErr = errDB
					} else {
						f.store.findSessionErrs = map[string]error{"handle": errDB}
					}
				case "mapping":
					f.store.lockMappingErr = errDB
				case "membership":
					portal, claims.Scope, claims.TenantID, claims.OrgID = domain.ScopeOrg, domain.ScopeOrg, 0, 7
					f.auth.traeger = validationMemberships{err: errDB}
				case "switch-membership", "switch-roles", "school":
					claims.ViaTraeger, claims.OrgID = true, 7
					switch lookup {
					case "switch-membership":
						f.auth.traeger = validationMemberships{err: errDB}
					case "switch-roles":
						store.rolesErr = errDB
					case "school":
						f.schools.findErr = errDB
					}
				case "account-session":
					store.accountSessionErr = errDB
				}
				f.auth.store = store
				f.sessions.sessions = store
				expiry := time.Now().Add(time.Hour)
				f.store.sessions[1] = domain.AccountSession{ID: 1, AccountID: 1, TenantID: 10, Token: "handle", FamilyID: "family", PortalScope: portal, Expiry: expiry, OrganizationID: claims.OrgID}
				f.store.operatorTokens[1] = domain.OperatorSession{ID: 1, OperatorID: 1, Token: "handle", FamilyID: "family", Expiry: expiry}
				access, refresh, err := (fakeCodec{}).IssueTokenPair(claims, domain.RefreshClaims{AccountID: 1, TenantID: claims.TenantID, OrgID: claims.OrgID, Scope: claims.Scope, Token: "handle", ExpiresAt: expiry.Unix()})
				require.NoError(t, err)
				if !withRefresh {
					refresh = ""
				}
				_, err = f.auth.ValidateSessionTokens(context.Background(), access, refresh, portal)
				require.ErrorIs(t, err, errDB, "lookup=%s refresh=%t", lookup, withRefresh)
				require.NotErrorIs(t, err, identityaccess.ErrInvalidToken)
			}
		})
	}
}

func TestValidateSessionTokensRefusesMissingAccess(t *testing.T) {
	t.Parallel()
	for _, missing := range []string{"account", "operator", "mapping", "membership", "switch-membership", "school", "account-session", "operator-session"} {
		t.Run(missing, func(t *testing.T) {
			t.Parallel()
			f := newAuthFixture(t)
			f.seedStaff(1, 10, "user")
			f.auth.traeger = validationMemberships{}
			claims := domain.SessionClaims{AccountID: 1, TenantID: 10, FamilyID: "family", ExpiresAt: time.Now().Add(time.Hour).Unix()}
			portal := "tenant"
			refresh := ""
			switch missing {
			case "account":
				delete(f.store.accounts, 1)
			case "operator", "operator-session":
				portal, claims.Scope, claims.TenantID = "platform", "platform", 0
			case "mapping":
				f.store.inactive[mappingKey{1, 10}] = true
			case "membership":
				portal, claims.Scope, claims.TenantID, claims.OrgID = domain.ScopeOrg, domain.ScopeOrg, 0, 7
				f.auth.traeger = validationMemberships{absent: true}
			case "switch-membership", "school":
				claims.ViaTraeger, claims.OrgID = true, 7
				if missing == "switch-membership" {
					f.auth.traeger = validationMemberships{absent: true}
				}
			}
			f.auth.store = validationStore{fakeStore: f.store}
			expiry := time.Now().Add(time.Hour)
			f.store.sessions[1] = domain.AccountSession{ID: 1, AccountID: 1, FamilyID: "family", PortalScope: portal, Expiry: expiry}
			f.store.operatorTokens[1] = domain.OperatorSession{ID: 1, OperatorID: 1, FamilyID: "family", Expiry: expiry}
			if missing == "operator-session" {
				f.store.addOperator(1, "operator@example.com", "unused", true)
			}
			access, pairRefresh, err := (fakeCodec{}).IssueTokenPair(claims, domain.RefreshClaims{AccountID: 1, TenantID: claims.TenantID, Scope: claims.Scope, Token: "missing", ExpiresAt: expiry.Unix()})
			require.NoError(t, err)
			if missing == "account-session" || missing == "operator-session" {
				refresh = pairRefresh
			}
			_, err = f.auth.ValidateSessionTokens(context.Background(), access, refresh, portal)
			require.ErrorIs(t, err, identityaccess.ErrInvalidToken)
		})
	}
}
