package active

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scopeSettings resolves operations.operational_overview_scope to the given
// value and errors on every other key, so a handler reaching for a second
// access setting fails the test loudly (#2380: there is only one rule).
func scopeSettings(scope string) *configtest.Mock {
	return &configtest.Mock{
		ResolveStringFn: func(_ context.Context, key string) (string, error) {
			if key != configModel.KeyOperationalOverviewScope {
				return "", fmt.Errorf("unexpected settings key: %s", key)
			}
			return scope, nil
		},
	}
}

// failingScopeSettings makes the scope lookup fail, standing in for a
// database fault. The gate must fail closed.
func failingScopeSettings() *configtest.Mock {
	return &configtest.Mock{
		ResolveStringFn: func(_ context.Context, _ string) (string, error) {
			return "", fmt.Errorf("settings unavailable")
		},
	}
}

// stubUserContext implements only the staff-access operations the overview uses.
type stubUserContext struct {
	staff *StaffIdentity
}

func (s *stubUserContext) GetCurrentStaff(_ context.Context) (*StaffIdentity, error) {
	return s.staff, nil
}

func (s *stubUserContext) HasCurrentStaff(context.Context) (bool, error) {
	return s != nil && s.staff != nil, nil
}

// verifiedStaffContext is a caller with a staff record in the current tenant.
func verifiedStaffContext() *stubUserContext {
	return &stubUserContext{staff: &StaffIdentity{ID: 7}}
}

// =============================================================================
// HELPER: build request with JWT claims in context
// =============================================================================

func newRequestWithClaims(method, path string, claims jwt.AppClaims) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	return req.WithContext(claimsCtx(claims))
}

func claimsCtx(claims jwt.AppClaims) context.Context {
	return claimsCtxWithParent(context.Background(), claims)
}

func claimsCtxWithParent(parent context.Context, claims jwt.AppClaims) context.Context {
	ctx := context.WithValue(parent, jwt.CtxClaims, claims)
	principal, err := permissions.NewPrincipal(permissions.PrincipalInput{
		AccountID: int64(claims.ID), TenantID: claims.TenantID, OrganizationID: claims.OrgID, Scope: claims.Scope,
		Roles: claims.Roles, Permissions: claims.Permissions, Admin: claims.IsAdmin,
	})
	if err != nil {
		panic(err)
	}
	return permissions.WithPrincipal(ctx, principal)
}

func adminClaims() jwt.AppClaims {
	return jwt.AppClaims{
		ID:       42,
		IsAdmin:  true,
		Roles:    []string{"admin"},
		TenantID: 10,
	}
}

func staffClaims() jwt.AppClaims {
	return jwt.AppClaims{
		ID:       99,
		IsAdmin:  false,
		Roles:    []string{"user"},
		TenantID: 10,
	}
}

// =============================================================================
// TESTS: operationalOverview — the single school-wide access rule (#2380)
// =============================================================================

func TestOperationalOverview_ScopeOwnKeepsStaffPersonalButAllowsAdmins(t *testing.T) {
	t.Parallel()

	rs := resourceForTest(Resource{Presence: sessionQueryStub{},
		SettingsService:    scopeSettings(configModel.OverviewScopeOwn),
		UserContextService: verifiedStaffContext(),
	})

	assert.True(t, rs.operationalOverview(claimsCtx(adminClaims())), "admins always have the school-wide overview")
	assert.False(t, rs.operationalOverview(claimsCtx(staffClaims())), "staff stay on their own supervisions")
}

func TestOperationalOverview_ScopeAdmins(t *testing.T) {
	t.Parallel()

	rs := resourceForTest(Resource{Presence: sessionQueryStub{},
		SettingsService:    scopeSettings(configModel.OverviewScopeAdmins),
		UserContextService: verifiedStaffContext(),
	})

	assert.True(t, rs.operationalOverview(claimsCtx(adminClaims())))
	assert.False(t, rs.operationalOverview(claimsCtx(staffClaims())), "the admins scope must not leak to staff")
}

func TestOperationalOverview_ScopeAllStaff(t *testing.T) {
	t.Parallel()

	rs := resourceForTest(Resource{Presence: sessionQueryStub{},
		SettingsService:    scopeSettings(configModel.OverviewScopeAllStaff),
		UserContextService: verifiedStaffContext(),
	})

	assert.True(t, rs.operationalOverview(claimsCtx(adminClaims())))
	assert.True(t, rs.operationalOverview(claimsCtx(staffClaims())))
}

func TestOperationalOverview_ScopeAllStaffDeniesNonStaffAccounts(t *testing.T) {
	t.Parallel()

	// A guardian or guest account authenticates against the same portal but
	// has no staff record — the broadest scope must not reach them.
	rs := resourceForTest(Resource{Presence: sessionQueryStub{},
		SettingsService:    scopeSettings(configModel.OverviewScopeAllStaff),
		UserContextService: &stubUserContext{},
	})

	assert.False(t, rs.operationalOverview(claimsCtx(staffClaims())))
}

func TestOperationalOverview_UnknownScopeFallsBackToOwn(t *testing.T) {
	t.Parallel()

	rs := resourceForTest(Resource{Presence: sessionQueryStub{},
		SettingsService:    scopeSettings("everyone_on_the_internet"),
		UserContextService: verifiedStaffContext(),
	})

	assert.True(t, rs.operationalOverview(claimsCtx(adminClaims())))
	assert.False(t, rs.operationalOverview(claimsCtx(staffClaims())))
}

func TestOperationalOverview_SettingsFaultFailsClosed(t *testing.T) {
	t.Parallel()

	rs := resourceForTest(Resource{Presence: sessionQueryStub{},
		SettingsService:    failingScopeSettings(),
		UserContextService: verifiedStaffContext(),
	})

	assert.False(t, rs.operationalOverview(claimsCtx(adminClaims())))
}

func TestOperationalOverview_NilSettings(t *testing.T) {
	t.Parallel()

	rs := resourceForTest(Resource{Presence: sessionQueryStub{}, SettingsService: nil, UserContextService: verifiedStaffContext()})

	assert.False(t, rs.operationalOverview(claimsCtx(adminClaims())))
}

// =============================================================================
// TESTS: getAllActiveSupervisions handler
// =============================================================================

func TestGetAllActiveSupervisions_ForbiddenForNonAdmin(t *testing.T) {
	t.Parallel()

	rs := resourceForTest(Resource{Presence: sessionQueryStub{},
		SettingsService:    scopeSettings(configModel.OverviewScopeAdmins),
		UserContextService: verifiedStaffContext(),
	})
	r := newRequestWithClaims("GET", "/active/supervisors/all", staffClaims())
	w := httptest.NewRecorder()

	rs.getAllActiveSupervisions(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetAllActiveSupervisions_AllStaffScopeAllowsPermissionBearingStaff(t *testing.T) {
	t.Parallel()

	rs := resourceForTest(Resource{Presence: sessionQueryStub{},
		SettingsService:    scopeSettings(configModel.OverviewScopeAllStaff),
		UserContextService: verifiedStaffContext(),
		Operations:         &stubPresenceOperations{},
	})
	r := newRequestWithClaims("GET", "/active/supervisors/all", staffClaims())
	w := httptest.NewRecorder()

	rs.getAllActiveSupervisions(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

// The organisational group mode must no longer decide operational access
// (#2380): open care alone leaves the school on the restrictive default.
func TestGetAllActiveSupervisions_GroupModeAloneGrantsNothing(t *testing.T) {
	t.Parallel()

	settings := &configtest.Mock{
		ResolveStringFn: func(_ context.Context, key string) (string, error) {
			switch key {
			case configModel.KeyGroupMode:
				return configModel.GroupModeOpenCare, nil
			case configModel.KeyOperationalOverviewScope:
				return configModel.OverviewScopeOwn, nil
			default:
				return "", fmt.Errorf("unexpected settings key: %s", key)
			}
		},
	}
	rs := resourceForTest(Resource{Presence: sessionQueryStub{},
		SettingsService:    settings,
		UserContextService: verifiedStaffContext(),
		Operations:         &stubPresenceOperations{},
	})
	r := newRequestWithClaims("GET", "/active/supervisors/all", staffClaims())
	w := httptest.NewRecorder()

	rs.getAllActiveSupervisions(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetAllActiveSupervisions_ForbiddenWhenSettingsNil(t *testing.T) {
	t.Parallel()

	rs := resourceForTest(Resource{Presence: sessionQueryStub{}, SettingsService: nil})
	r := newRequestWithClaims("GET", "/active/supervisors/all", adminClaims())
	w := httptest.NewRecorder()

	rs.getAllActiveSupervisions(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetAllActiveSupervisions_OwnScopeStillAllowsAdmin(t *testing.T) {
	t.Parallel()

	rs := resourceForTest(Resource{Presence: sessionQueryStub{},
		SettingsService:    scopeSettings(configModel.OverviewScopeOwn),
		UserContextService: verifiedStaffContext(),
		Operations:         &stubPresenceOperations{},
	})
	r := newRequestWithClaims("GET", "/active/supervisors/all", adminClaims())
	w := httptest.NewRecorder()

	rs.getAllActiveSupervisions(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetAllActiveSupervisions_ForbiddenOnSettingError(t *testing.T) {
	t.Parallel()

	rs := resourceForTest(Resource{Presence: sessionQueryStub{},
		SettingsService:    failingScopeSettings(),
		UserContextService: verifiedStaffContext(),
	})
	r := newRequestWithClaims("GET", "/active/supervisors/all", adminClaims())
	w := httptest.NewRecorder()

	rs.getAllActiveSupervisions(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetAllActiveSupervisions_SuccessEmptyGroups(t *testing.T) {
	t.Parallel()

	rs := resourceForTest(Resource{Presence: sessionQueryStub{},
		SettingsService:    scopeSettings(configModel.OverviewScopeAdmins),
		UserContextService: verifiedStaffContext(),
		Operations:         &stubPresenceOperations{},
	})
	r := newRequestWithClaims("GET", "/active/supervisors/all", adminClaims())
	w := httptest.NewRecorder()

	rs.getAllActiveSupervisions(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "success", resp["status"])
}

func TestGetAllActiveSupervisions_SuccessWithActiveGroups(t *testing.T) {
	t.Parallel()

	now := time.Now()
	past := now.Add(-2 * time.Hour)
	endedTime := now.Add(-1 * time.Hour)

	rs := resourceForTest(Resource{
		SettingsService:    scopeSettings(configModel.OverviewScopeAdmins),
		UserContextService: verifiedStaffContext(),
		Operations:         &stubPresenceOperations{},
		Presence: sessionQueryStub{
			list: func(_ context.Context, _ studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, error) {
				return []studentpresence.LiveGroup{
					// Active group (no end time)
					{
						ID:        100,
						StartTime: past,
					},
					// Ended group (has end time in the past → IsActive() = false)
					{
						ID:        101,
						StartTime: past,
						EndTime:   &endedTime,
					},
				}, nil
			},
		},
	})
	r := newRequestWithClaims("GET", "/active/supervisors/all", adminClaims())
	w := httptest.NewRecorder()

	rs.getAllActiveSupervisions(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "success", resp["status"])

	// Only the active group should be in the response (the ended one is filtered out)
	data, ok := resp["data"].([]any)
	require.True(t, ok)
	assert.Len(t, data, 1)
}

func TestGetAllActiveSupervisions_ServiceError(t *testing.T) {
	t.Parallel()

	rs := resourceForTest(Resource{
		SettingsService:    scopeSettings(configModel.OverviewScopeAdmins),
		UserContextService: verifiedStaffContext(),
		Operations:         &stubPresenceOperations{},
		Presence: sessionQueryStub{
			list: func(_ context.Context, _ studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, error) {
				return nil, fmt.Errorf("database error")
			},
		},
	})
	r := newRequestWithClaims("GET", "/active/supervisors/all", adminClaims())
	w := httptest.NewRecorder()

	rs.getAllActiveSupervisions(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
