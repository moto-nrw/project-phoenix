package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	accountHTTP "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"
	operatorHTTP "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentHTTP "github.com/moto-nrw/project-phoenix/workflows/parentportal/http"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestPasswordFailuresLockAccountAcrossPortals(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	_, account := testpkg.CreateTestTeacherWithAccount(t, db, "Login", "Protection")
	password := testpkg.NewTestPassword()
	hash, err := securityruntime.HashPassword(password)
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE auth.accounts SET password_hash = ? WHERE id = ?`, hash, account.ID).Exec(t.Context())
	require.NoError(t, err)
	module, err := NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	for range 5 {
		_, _, err = module.Auth.LoginParentWithAudit(t.Context(), account.Email, testpkg.NewTestPassword(), "", "")
		require.ErrorIs(t, err, identityaccess.ErrInvalidCredentials)
	}
	result, err := module.Auth.LoginWithMFAGate(t.Context(), account.Email, password, "", "", "", "")
	require.ErrorIs(t, err, identityaccess.ErrInvalidCredentials)
	require.True(t, result == nil)
	result, err = module.Auth.LoginSchoolWithMFAGate(t.Context(), account.Email, password, "", "", "")
	require.ErrorIs(t, err, identityaccess.ErrInvalidCredentials)
	require.True(t, result == nil)
}

func passwordLoginFixture(t *testing.T, portal string) (*bun.DB, int64, func(string) error, string) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	password := testpkg.NewTestPassword()
	var id int64
	var address string
	switch portal {
	case "operator":
		operator := testpkg.CreateTestOperator(t, db)
		id, address = operator.ID, operator.Email
	case "parent":
		parent := testpkg.CreateTestParentGuardianChain(t, db)
		id, address = parent.AccountID, parent.Email
	default:
		_, account := testpkg.CreateTestTeacherWithAccount(t, db, "Login", "Lifecycle")
		id, address = account.ID, account.Email
		if portal == "school" {
			testpkg.AssignLehrkraftSystemRole(t, db, id, testpkg.Tenant(t))
		}
	}
	hash, err := securityruntime.HashPassword(password)
	require.NoError(t, err)
	table := "auth.accounts"
	if portal == "operator" {
		table = "platform.operators"
	}
	_, err = db.NewRaw(`UPDATE `+table+` SET password_hash = ? WHERE id = ?`, hash, id).Exec(t.Context())
	require.NoError(t, err)
	module, err := NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	login := func(value string) error {
		// Normalization must not create separate counters.
		email := " " + strings.ToUpper(address) + " "
		switch portal {
		case "operator":
			_, err := module.Auth.LoginOperatorWithMFAGate(t.Context(), email, value, "", "", "")
			return err
		case "parent":
			_, _, err := module.Auth.LoginParentWithAudit(t.Context(), email, value, "", "")
			return err
		case "school":
			_, err := module.Auth.LoginSchoolWithMFAGate(t.Context(), email, value, "", "", "")
			return err
		default:
			_, err := module.Auth.LoginWithMFAGate(t.Context(), email, value, "", "", "", "")
			return err
		}
	}
	return db, id, login, password
}

func TestPasswordLoginLockExpiresAndSuccessfulCredentialsResetAllPortals(t *testing.T) {
	t.Parallel()
	for _, portal := range []string{"tenant", "school", "parent", "operator"} {
		t.Run(portal, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db, id, login, password := passwordLoginFixture(t, portal)
			invalid := identityaccess.ErrInvalidCredentials
			table := "auth.accounts"
			if portal == "operator" {
				invalid, table = identityaccess.ErrOperatorInvalidCredentials, "platform.operators"
			}
			wrong := testpkg.NewTestPassword()
			for range 2 {
				for range 4 {
					require.ErrorIs(t, login(wrong), invalid)
				}
				require.NoError(t, login(password), "valid credentials reset the password counter before the separate MFA gate")
			}
			for range 5 {
				require.ErrorIs(t, login(wrong), invalid)
			}
			var until time.Time
			require.NoError(t, db.NewRaw(`SELECT password_locked_until FROM `+table+` WHERE id = ?`, id).Scan(t.Context(), &until))
			require.WithinDuration(t, time.Now().Add(15*time.Minute), until, 5*time.Second)
			require.ErrorIs(t, login(password), invalid)
			require.ErrorIs(t, login(wrong), invalid)
			var unchanged time.Time
			require.NoError(t, db.NewRaw(`SELECT password_locked_until FROM `+table+` WHERE id = ?`, id).Scan(t.Context(), &unchanged))
			require.True(t, until.Equal(unchanged), "requests during a lock must not extend it")
			// Advance the persisted deadline instead of waiting fifteen minutes.
			_, err := db.NewRaw(`UPDATE `+table+` SET password_locked_until = ? WHERE id = ?`, time.Now().Add(-time.Minute), id).Exec(t.Context())
			require.NoError(t, err)
			for range 4 {
				require.ErrorIs(t, login(wrong), invalid)
			}
			require.NoError(t, login(password), "expiry starts a fresh attempt budget")
		})
	}
}

func TestConcurrentPasswordFailuresCannotLoseAttemptsOrLockOtherIdentities(t *testing.T) {
	t.Parallel()
	db, id, login, password := passwordLoginFixture(t, "tenant")
	wrong := testpkg.NewTestPassword()
	const workers = 8
	results := make(chan error, workers)
	var group sync.WaitGroup
	for range workers {
		group.Go(func() { results <- login(wrong) })
	}
	group.Wait()
	close(results)
	for err := range results {
		require.ErrorIs(t, err, identityaccess.ErrInvalidCredentials)
	}
	require.ErrorIs(t, login(password), identityaccess.ErrInvalidCredentials)
	var attempts int
	require.NoError(t, db.NewRaw(`SELECT password_attempts FROM auth.accounts WHERE id = ?`, id).Scan(t.Context(), &attempts))
	require.Equal(t, 5, attempts, "only the first five guesses count; locked requests neither reset nor extend")
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	other := testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("isolated-%d", otherTenant), password)
	testpkg.EnsureAccountTenant(t, db, other.ID, otherTenant)
	module, err := NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	_, err = module.Auth.LoginWithMFAGate(t.Context(), other.Email, password, "", "", "", "")
	require.NoError(t, err, "another identity and school retain their own budget")
}

func TestPasswordLoginRefusalsDoNotEnumerateAccountsAcrossPortals(t *testing.T) {
	t.Parallel()
	for _, portal := range []string{"tenant", "school", "parent", "operator"} {
		t.Run(portal, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db, id, login, password := passwordLoginFixture(t, portal)
			table := "auth.accounts"
			if portal == "operator" {
				table = "platform.operators"
			}
			var address string
			require.NoError(t, db.NewRaw(`SELECT email FROM `+table+` WHERE id = ?`, id).Scan(t.Context(), &address))
			module, err := NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
			require.NoError(t, err)
			root := chi.NewRouter()
			switch portal {
			case "tenant":
				resource := &accountHTTP.Resource{Sessions: module.Auth, MFAService: module.MFA}
				root.Mount("/auth", resource.Router(TenantSessionChain))
			case "school":
				_, resource, _, _ := setupSchoolRoute(t)
				resource.AuthService, resource.MFAService = SchoolPortalAuth(module.Auth), SchoolPortalMFA(module.MFA)
				root.Mount("/", newSchoolRouter(resource, resource.MFAService))
			case "parent":
				resource := &parentHTTP.Resource{Auth: parentPortalLogin(module.Auth)}
				root.Mount("/", resource.RouterWithAuthRateLimiter(nil, ParentSessionChain))
			case "operator":
				resource := operatorHTTP.NewResource(module.Auth, operatorHTTP.OperatorResponses(nil))
				root.Post("/auth/login", resource.Login)
			}
			refusal := func(email, value string) string {
				body, err := json.Marshal(map[string]string{"email": email, "password": value})
				require.NoError(t, err)
				request := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(string(body)))
				request.Header.Set("Content-Type", "application/json")
				response := httptest.NewRecorder()
				root.ServeHTTP(response, request)
				require.Equal(t, http.StatusUnauthorized, response.Code)
				require.Zero(t, len(response.Result().Cookies()))
				require.Empty(t, response.Header().Get("Retry-After"), "a lock must not have a distinct header")
				return response.Body.String()
			}
			wrong := testpkg.NewTestPassword()
			unknown := refusal("missing-"+address, wrong)
			incorrect := refusal(address, wrong)
			require.Equal(t, unknown, incorrect)
			for range 4 {
				require.Error(t, login(wrong))
			}
			require.Equal(t, unknown, refusal(address, password), "a correct password during the lock gives the same refusal")
		})
	}
}
