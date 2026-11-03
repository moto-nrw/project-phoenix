package application

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every account session, in every portal, is minted after the last-login
// stamp found the account alive in the mint transaction (#5546): an account
// the erasure of unused accounts anonymized after the caller's checks gets
// no session, whatever the scope and whether a guard ran.
func TestSessionMintRefusesAnAccountErasedAfterItsChecks(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{domain.ScopeTenant, domain.ScopeParent, domain.ScopeSchool, domain.ScopeCaterer, domain.ScopeOrg} {
		t.Run("scope "+domain.PersistedPortalScope(scope), func(t *testing.T) {
			t.Parallel()
			f := newAuthFixture(t)
			f.seedStaff(1, 10, "user")
			f.store.erasedBeforeMint = true
			tenantID := int64(10)
			if scope == domain.ScopeOrg {
				tenantID = 0
			}
			_, err := f.auth.createRefreshSessionGuarded(context.Background(), f.store.accounts[1], tenantID, scope, nil, "", true)
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
			_, _, err := f.auth.loginWithAudit(context.Background(), "user1@example.com", "secret", "192.0.2.1", "tests", "")
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

// The live-account check sits in prepareSessionMint, which only
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
	assert.Equal(t, map[string]int{"account_login.go": 1, "session_refresh.go": 1}, writers)

	persist := functionSource(t, "account_login.go", "func (s *AccountAuthentication) persistSessionInTransaction(")
	requireCalledBefore(t, persist, "s.prepareSessionMint(", "s.sessions.CreateAccountSession(",
		"the login mint checks the live account before it writes the session")
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
