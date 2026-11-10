//go:build testhooks

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityoperator "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The active-operator check of ADR 0161 step 2 (#5795) on a real database:
// status and code at every refusal and failure point, the order of the two
// reads, the school and portal boundary before them, and the bundled unit.
// Everything but the unit's shape holds on the code before the bundle too.

type operatorLivenessRig struct {
	db       *bun.DB
	identity identityaccess.Owner
	handler  http.Handler
}

func newOperatorLivenessRig(t *testing.T) operatorLivenessRig {
	t.Helper()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)
	_, identity := buildOperatorProvisioningWithIdentity(t, db)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := identityoperator.RequiresOperatorScope(identityoperator.RequiresActiveOperator(identity)(next))
	return operatorLivenessRig{db: db, identity: identity, handler: handler}
}

// session signs an operator in and returns its refresh family.
func (r operatorLivenessRig) session(t *testing.T, operatorID int64) string {
	t.Helper()
	_, _, err := r.identity.IssueTokensForAuthenticatedOperator(provisioningContext(t, r.db), operatorID, nil, "127.0.0.1", "operator-liveness")
	require.NoError(t, err)
	var family string
	require.NoError(t, r.db.NewRaw("SELECT family_id FROM platform.operator_refresh_tokens WHERE operator_id = ? ORDER BY id DESC LIMIT 1", operatorID).Scan(context.Background(), &family))
	require.NotEmpty(t, family)
	return family
}

func (r operatorLivenessRig) serve(t *testing.T, claims routetest.Claims) (*httptest.ResponseRecorder, []string) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/operator/ping", nil)
	request = request.WithContext(routetest.WithAuthenticatedContext(provisioningContext(t, r.db), claims, nil))
	counter := testpkg.CaptureQueries(t, r.db)
	response := httptest.NewRecorder()
	r.handler.ServeHTTP(response, request)
	queries := counter.Queries()
	counter.Stop()
	return response, queries
}

func operatorClaims(operatorID int64, family string) routetest.Claims {
	return routetest.Claims{ID: int(operatorID), Scope: "platform", FamilyID: family}
}

func isOperatorRead(q string) bool {
	return strings.Contains(q, `FROM "platform"."operators" AS "operator"`)
}

func isOperatorSessionRead(q string) bool {
	return strings.Contains(q, `FROM "platform"."operator_refresh_tokens" AS "operator_refresh_token"`)
}

func firstIndex(queries []string, match func(string) bool) int {
	for i, q := range queries {
		if match(q) {
			return i
		}
	}
	return -1
}

func operatorWire(t *testing.T, response *httptest.ResponseRecorder) (int, string, string) {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body), response.Body.String())
	return response.Code, fmt.Sprint(body["code"]), response.Body.String()
}

func TestRequiresActiveOperatorRefusalsOnTheDatabase(t *testing.T) {
	t.Parallel()
	rig := newOperatorLivenessRig(t)
	ctx := context.Background()
	operator := testpkg.CreateTestOperator(t, rig.db).ID
	family := rig.session(t, operator)
	other := testpkg.CreateTestOperator(t, rig.db).ID
	otherFamily := rig.session(t, other)
	expired := testpkg.CreateTestOperator(t, rig.db).ID
	expiredFamily := rig.session(t, expired)
	_, err := rig.db.NewRaw("UPDATE platform.operator_refresh_tokens SET expiry = now() - interval '1 minute' WHERE family_id = ?", expiredFamily).Exec(ctx)
	require.NoError(t, err)
	inactive := testpkg.CreateTestOperator(t, rig.db).ID
	inactiveFamily := rig.session(t, inactive)
	_, err = rig.db.NewRaw("UPDATE platform.operators SET active = false WHERE id = ?", inactive).Exec(ctx)
	require.NoError(t, err)
	removed := testpkg.CreateTestOperator(t, rig.db).ID
	_, err = rig.db.NewRaw("DELETE FROM platform.operators WHERE id = ?", removed).Exec(ctx)
	require.NoError(t, err)

	response, queries := rig.serve(t, operatorClaims(operator, family))
	require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
	read, session := firstIndex(queries, isOperatorRead), firstIndex(queries, isOperatorSessionRead)
	require.True(t, read >= 0 && read < session, "operator before its session: %q", queries)

	// Unknown, expired and foreign families look alike from outside.
	_, _, ended := operatorWire(t, func() *httptest.ResponseRecorder {
		r, _ := rig.serve(t, operatorClaims(operator, "keine-familie"))
		return r
	}())
	for _, tc := range []struct {
		name        string
		claims      routetest.Claims
		status      int
		code        string
		sessionRead bool
	}{
		{"unknown family", operatorClaims(operator, "keine-familie"), http.StatusForbidden, "identity.account_inactive", true},
		{"another operator's family", operatorClaims(operator, otherFamily), http.StatusForbidden, "identity.account_inactive", true},
		{"expired session", operatorClaims(expired, expiredFamily), http.StatusForbidden, "identity.account_inactive", true},
		{"inactive operator", operatorClaims(inactive, inactiveFamily), http.StatusForbidden, "identity.account_inactive", false},
		{"no family in the token", operatorClaims(operator, ""), http.StatusForbidden, "identity.account_inactive", false},
		{"unknown operator", operatorClaims(removed, family), http.StatusUnauthorized, "identity.invalid_credentials", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, queries := rig.serve(t, tc.claims)
			status, code, wire := operatorWire(t, response)
			require.Equal(t, tc.status, status, wire)
			require.Equal(t, tc.code, code, wire)
			if tc.status == http.StatusForbidden {
				require.Equal(t, ended, wire, "refusals are indistinguishable")
			}
			require.GreaterOrEqual(t, firstIndex(queries, isOperatorRead), 0, "%q", queries)
			require.Equal(t, tc.sessionRead, firstIndex(queries, isOperatorSessionRead) >= 0, "%q", queries)
		})
	}

	// School and parent tokens of two schools stop before any read, alike.
	var wires []string
	for _, school := range []string{"school A", "school B"} {
		t.Run(school, func(t *testing.T) {
			schoolID := testpkg.OwnTenant(t)
			account := testpkg.CreateTestAccount(t, rig.db, "operator-scope")
			for _, scope := range []string{"tenant", "parent"} {
				response, queries := rig.serve(t, routetest.Claims{ID: int(account.ID), Scope: scope, TenantID: schoolID, FamilyID: family})
				status, code, wire := operatorWire(t, response)
				require.Equal(t, http.StatusForbidden, status, wire)
				require.Equal(t, "general.permission", code, wire)
				require.Empty(t, queries)
				wires = append(wires, wire)
			}
		})
	}
	require.Len(t, wires, 4)
	for _, wire := range wires[1:] {
		require.Equal(t, wires[0], wire)
	}
}

// TestRequiresActiveOperatorDatabaseFaults fails each read for real. Both
// answer 503 like before, and only ROLLBACK follows the failing statement.
func TestRequiresActiveOperatorDatabaseFaults(t *testing.T) {
	t.Parallel()
	rig := newOperatorLivenessRig(t)
	operator := testpkg.CreateTestOperator(t, rig.db).ID
	family := rig.session(t, operator)
	for _, fault := range []struct {
		name, table string
		failing     func(string) bool
		sessionRead bool
	}{
		{"operator read", "platform.operators", isOperatorRead, false},
		{"session read", "platform.operator_refresh_tokens", isOperatorSessionRead, true},
	} {
		t.Run(fault.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := rig.db.NewRaw("REVOKE SELECT ON " + fault.table + " FROM phoenix_admin").Exec(ctx)
			require.NoError(t, err)
			response, queries := rig.serve(t, operatorClaims(operator, family))
			_, err = rig.db.NewRaw("GRANT SELECT ON " + fault.table + " TO phoenix_admin").Exec(ctx)
			require.NoError(t, err)

			status, code, wire := operatorWire(t, response)
			require.Equal(t, http.StatusServiceUnavailable, status, wire)
			require.Equal(t, "general.unavailable", code, wire)
			failed := firstIndex(queries, fault.failing)
			require.GreaterOrEqual(t, failed, 0, "%q", queries)
			require.Equal(t, []string{"ROLLBACK"}, queries[failed+1:], "%q", queries)
			require.Equal(t, fault.sessionRead, firstIndex(queries, isOperatorSessionRead) >= 0, "%q", queries)

			ok, _ := rig.serve(t, operatorClaims(operator, family))
			require.Equal(t, http.StatusNoContent, ok.Code, "grant restored")
		})
	}
}

// TestRequiresActiveOperatorIsOneAdministrativeUnit pins the bundle: alone it
// opens one administrative unit, and a refusal between the reads ends it with
// ROLLBACK.
func TestRequiresActiveOperatorIsOneAdministrativeUnit(t *testing.T) {
	t.Parallel()
	rig := newOperatorLivenessRig(t)
	operator := testpkg.CreateTestOperator(t, rig.db).ID
	family := rig.session(t, operator)

	_, queries := rig.serve(t, operatorClaims(operator, family))
	require.Len(t, queries, 5, "%q", queries)
	require.Equal(t, []string{"BEGIN", "SET LOCAL ROLE phoenix_admin"}, queries[:2])
	require.True(t, isOperatorRead(queries[2]) && isOperatorSessionRead(queries[3]), "%q", queries)
	require.Equal(t, "COMMIT", queries[4])

	_, queries = rig.serve(t, operatorClaims(operator, ""))
	require.Len(t, queries, 4, "%q", queries)
	require.Equal(t, "ROLLBACK", queries[3])

	// The unit leaves no role or school on the pooled connection.
	rig.db.SetMaxIdleConns(1)
	rig.db.SetMaxOpenConns(1)
	var login string
	require.NoError(t, rig.db.NewRaw("SELECT current_user").Scan(context.Background(), &login))
	for _, claimFamily := range []string{family, ""} {
		rig.serve(t, operatorClaims(operator, claimFamily))
		var role, school string
		require.NoError(t, rig.db.NewRaw("SELECT current_user, coalesce(current_setting('app.current_tenant_id', true), '')").Scan(context.Background(), &role, &school))
		require.Equal(t, login, role)
		require.Empty(t, school)
	}
}

// TestRequiresActiveOperatorJoinsAnOpenTransaction covers the ambient entries
// of ADR 0161 point 7: like the two reads alone, the check joins an open
// administrative transaction and an open school transaction.
func TestRequiresActiveOperatorJoinsAnOpenTransaction(t *testing.T) {
	t.Parallel()
	rig := newOperatorLivenessRig(t)
	operator := testpkg.CreateTestOperator(t, rig.db).ID
	family := rig.session(t, operator)

	ctx := provisioningContext(t, rig.db)
	counter := testpkg.CaptureQueries(t, rig.db)
	require.NoError(t, tenant.WithAdminTx(ctx, rig.db, func(txCtx context.Context, _ bun.Tx) error {
		counter.Reset()
		return rig.identity.VerifyOperatorSession(txCtx, operator, family)
	}))
	joined := counter.Queries()
	counter.Stop()
	require.Len(t, joined, 3, "%q", joined)
	require.True(t, isOperatorRead(joined[0]) && isOperatorSessionRead(joined[1]), "%q", joined)
	require.Equal(t, "COMMIT", joined[2])

	// In a school's transaction it joins as well and reads nothing the
	// school role may not see; the verdict is the same for every school.
	var verdicts []string
	for _, school := range []string{"school A", "school B"} {
		t.Run(school, func(t *testing.T) {
			schoolID := testpkg.OwnTenant(t)
			counter := testpkg.CaptureQueries(t, rig.db)
			var verdict error
			txErr := tenant.WithTenantTx(tenant.WithTenantID(ctx, schoolID), rig.db, schoolID, func(txCtx context.Context, _ bun.Tx) error {
				counter.Reset()
				verdict = rig.identity.VerifyOperatorSession(txCtx, operator, family)
				return nil
			})
			queries := counter.Queries()
			counter.Stop()
			t.Logf("school frame: verdict=%v tx=%v statements=%q", verdict, txErr, queries)
			require.Error(t, verdict, "a school frame never confirms an operator")
			require.NotErrorIs(t, verdict, identityaccess.ErrOperatorInactive)
			require.Equal(t, -1, firstIndex(queries, func(q string) bool { return q == "BEGIN" || q == "SET LOCAL ROLE phoenix_admin" }), "joins the school frame: %q", queries)
			verdicts = append(verdicts, fmt.Sprintf("%v|%v|%d", verdict, txErr, len(queries)))
		})
	}
	require.Len(t, verdicts, 2)
	require.Equal(t, verdicts[0], verdicts[1])
}
