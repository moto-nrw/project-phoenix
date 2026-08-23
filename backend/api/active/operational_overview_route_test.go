// Route-level acceptance for the school-wide operational overview (#2380).
//
// These tests drive the production Router() through the real middleware chain
// and the real settings service, because the claim under test is precisely
// that list, detail and write paths answer the SAME question: the UI must
// never show a running module whose detail route then returns 403.
package active_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// setOverviewScopeForTest stores the tenant override through the real settings
// service, so the test exercises the same resolution path a school admin's
// save produces.
func setOverviewScopeForTest(t *testing.T, tc *testContext, scope string) {
	t.Helper()

	ctx := tenant.WithTenantID(context.Background(), testpkg.Tenant(t))
	require.NoError(t, tc.services.Settings.SetValue(
		ctx, configModel.KeyOperationalOverviewScope, scope, nil, nil,
	))
}

// caregiverClaims builds a plain staff caller: no admin role, no wildcard
// permission — the Betreuungskraft the issue is about.
func caregiverClaims(t *testing.T, tc *testContext, name string) jwt.AppClaims {
	t.Helper()

	_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Overview", name)
	return jwt.AppClaims{
		ID:          int(account.ID),
		TenantID:    testpkg.Tenant(t),
		Sub:         fmt.Sprintf("%d", account.ID),
		Roles:       []string{"user"},
		Permissions: []string{permissions.GroupsRead},
	}
}

// foreignActiveGroup creates a running module nobody in the test supervises.
func foreignActiveGroup(t *testing.T, tc *testContext, label string) int64 {
	t.Helper()

	room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Overview Room %s", label))
	activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Overview Activity %s", label))
	return testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID).ID
}

func getWithClaims(t *testing.T, router chi.Router, path string, claims jwt.AppClaims) int {
	t.Helper()

	req := testutil.NewJSONRequest(t, "GET", path, nil)
	return testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{permissions.GroupsRead}).Code
}

// TestOperationalOverviewScope_OwnKeepsCaregiverOnOwnSupervisions is the
// default and the deactivation case in one: without the freigabe, a caregiver
// neither gets the school-wide list nor the detail of a module they do not
// supervise.
func TestOperationalOverviewScope_OwnKeepsCaregiverOnOwnSupervisions(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)
	setOverviewScopeForTest(t, tc, configModel.OverviewScopeOwn)

	claims := caregiverClaims(t, tc, "Own")
	groupID := foreignActiveGroup(t, tc, "own")

	assert.Equal(t, http.StatusForbidden,
		getWithClaims(t, router, "/active/supervisors/all", claims),
		"the school-wide list must stay closed on the own scope")
	assert.Equal(t, http.StatusForbidden,
		getWithClaims(t, router, fmt.Sprintf("/active/groups/%d/visits/display", groupID), claims),
		"a foreign module's detail must stay closed on the own scope")
}

// TestOperationalOverviewScope_AdminsDoesNotReachCaregivers pins the middle
// step: opening the overview for administrators must not widen anything for a
// Betreuungskraft.
func TestOperationalOverviewScope_AdminsDoesNotReachCaregivers(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)
	setOverviewScopeForTest(t, tc, configModel.OverviewScopeAdmins)

	claims := caregiverClaims(t, tc, "Admins")
	groupID := foreignActiveGroup(t, tc, "admins")

	assert.Equal(t, http.StatusForbidden,
		getWithClaims(t, router, "/active/supervisors/all", claims))
	assert.Equal(t, http.StatusForbidden,
		getWithClaims(t, router, fmt.Sprintf("/active/groups/%d/visits/display", groupID), claims))
}

// TestOperationalOverviewScope_AllStaffOpensListAndDetail is the acceptance
// criterion: with the freigabe a caregiver opens every running module, and the
// list and the detail route agree — no block without a working detail call.
func TestOperationalOverviewScope_AllStaffOpensListAndDetail(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)
	setOverviewScopeForTest(t, tc, configModel.OverviewScopeAllStaff)

	claims := caregiverClaims(t, tc, "AllStaff")
	groupID := foreignActiveGroup(t, tc, "allstaff")

	assert.Equal(t, http.StatusOK,
		getWithClaims(t, router, "/active/supervisors/all", claims),
		"the school-wide list must open for verified staff")
	assert.Equal(t, http.StatusOK,
		getWithClaims(t, router, fmt.Sprintf("/active/groups/%d/visits/display", groupID), claims),
		"every module the list shows must also answer its detail route")
}

// TestOperationalOverviewScope_GroupModeIsNotAnAccessRule pins the decoupling
// the issue asks for: the organisational group mode alone opens nothing.
func TestOperationalOverviewScope_GroupModeIsNotAnAccessRule(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)

	ctx := tenant.WithTenantID(context.Background(), testpkg.Tenant(t))
	require.NoError(t, tc.services.Settings.SetValue(
		ctx, configModel.KeyGroupMode, configModel.GroupModeOpenCare, nil, nil,
	))

	claims := caregiverClaims(t, tc, "GroupMode")
	groupID := foreignActiveGroup(t, tc, "groupmode")

	assert.Equal(t, http.StatusForbidden,
		getWithClaims(t, router, "/active/supervisors/all", claims),
		"open care is an organisational decision, not a freigabe")
	assert.Equal(t, http.StatusForbidden,
		getWithClaims(t, router, fmt.Sprintf("/active/groups/%d/visits/display", groupID), claims))
}
