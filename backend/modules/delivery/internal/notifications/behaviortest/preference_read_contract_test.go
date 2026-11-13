//go:build testhooks

package notifications_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/delivery/internal/notifications"
	"github.com/moto-ogs/project-phoenix/modules/securityruntime"
	"github.com/moto-ogs/project-phoenix/modules/settings"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func notificationBudgetWireLog(t *testing.T, portal string, n, status int, header http.Header, body string, writeRows int64) {
	t.Helper()
	t.Logf("notification_wire portal=%s n=%d status=%d headers=%v body=%s write_rows=%d", portal, n, status, header, body, writeRows)
}

// A partial immutable snapshot still owns its key even when the catalogue
// also needs keys absent from it. A bulk query must not replace that value.
func TestNotificationPreferencesPartialSnapshotContract(t *testing.T) {
	t.Parallel()
	for _, portal := range []string{"staff", "school"} {
		t.Run(portal, func(t *testing.T) {
			db := testpkg.SetupIsolatedTestDB(t)
			database := testpkg.NewDatabase(t, db)
			unit := testpkg.TenantRuntime(t, db)
			module, err := api.NewDeliveryTestModule(database, unit)
			require.NoError(t, err)
			_, account := testpkg.CreateTestStaffWithAccount(t, db, "Snapshot", "Preferences")
			testpkg.AssignLehrkraftSystemRole(t, db, account.ID, testpkg.Tenant(t))
			notificationBudgetSettings(t, db, testpkg.Tenant(t), true)
			source := api.NewAuthTestSettings(database, unit)
			batch, ok := source.(interface {
				ResolveMany(context.Context, []string) (*settings.Snapshot, error)
			})
			require.True(t, ok, "real Settings composition supports snapshots")
			def, known := notifications.GetType(notifications.TypeStudentAbsenceReported)
			require.True(t, known)
			key := def.TenantGate
			snapshot, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, batch.ResolveMany, []string{key})
			require.NoError(t, err)
			testpkg.SetTestTenantSetting(t, db, testpkg.Tenant(t), key, "false")
			live, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, source.ResolveBool, key)
			require.NoError(t, err)
			require.False(t, live, "live value deliberately differs from the immutable snapshot")
			claims := testutil.TeacherTestClaims(int(account.ID))
			path := "/api/notifications/preferences/"
			if portal == "school" {
				claims.Scope, path = "school", "/school/notifications/preferences/"
			}
			request := testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)))
			request = request.WithContext(settings.WithSettingsSnapshot(request.Context(), snapshot))
			counter := testpkg.CaptureQueries(t, db)
			response := testutil.ExecuteRequestForTest(t, module.NotificationPreferenceRouterForTests(), request)
			counter.Stop()
			notificationBudgetLog(t, portal+"_partial_snapshot", 0, counter.Measurement(counter.Queries()).Statements)
			notificationBudgetWireLog(t, portal+"_partial_snapshot", 0, response.Code, response.Header(), response.Body.String(), counter.WriteRows())
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			notificationBudgetAssertResponse(t, response.Body.Bytes(), notificationBudgetCatalogue(t, portal), true)
			notificationBudgetReadOnly(t, counter)
		})
	}
}

// Invalid values retain their gate identity: the first failure in offered
// catalogue order determines the complete HTTP diagnostic.
func TestNotificationPreferencesFirstGateErrorContract(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"staff", "school", "staff_later", "school_later"} {
		t.Run(scenario, func(t *testing.T) {
			portal := strings.TrimSuffix(scenario, "_later")
			later := strings.HasSuffix(scenario, "_later")
			db := testpkg.SetupIsolatedTestDB(t)
			module, err := api.NewDeliveryTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
			require.NoError(t, err)
			_, account := testpkg.CreateTestStaffWithAccount(t, db, "Invalid", "Preferences")
			testpkg.AssignLehrkraftSystemRole(t, db, account.ID, testpkg.Tenant(t))
			notificationBudgetSettings(t, db, testpkg.Tenant(t), true)
			def, known := notifications.GetType(notifications.TypeStudentAbsenceReported)
			require.True(t, known)
			firstGate := def.TenantGate
			if portal == "staff" {
				def, known = notifications.GetType(notifications.TypePickupUpcoming)
				require.True(t, known)
				firstGate = def.TenantGate
			}
			invalidKeys := []string{firstGate, settings.KeyStaffMessagingEnabled}
			if later {
				firstGate = settings.KeyStaffMessagingEnabled
				invalidKeys = []string{firstGate}
			}
			for _, key := range invalidKeys {
				testpkg.SetTestTenantSetting(t, db, testpkg.Tenant(t), key, `"not-a-bool"`)
			}
			claims := testutil.TeacherTestClaims(int(account.ID))
			path := "/api/notifications/preferences/"
			if portal == "school" {
				claims.Scope, path = "school", "/school/notifications/preferences/"
			}
			request := testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)))
			counter := testpkg.CaptureQueries(t, db)
			response := testutil.ExecuteRequestForTest(t, module.NotificationPreferenceRouterForTests(), request)
			counter.Stop()
			notificationBudgetLog(t, scenario+"_invalid_gate", 0, counter.Measurement(counter.Queries()).Statements)
			notificationBudgetWireLog(t, scenario+"_invalid_gate", 0, response.Code, response.Header(), response.Body.String(), counter.WriteRows())
			require.Equal(t, http.StatusInternalServerError, response.Code)
			require.Equal(t, "application/json", response.Header().Get("Content-Type"))
			require.JSONEq(t, fmt.Sprintf(`{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-unerwarteter-fehler","title":"Internal Server Error","status":500,"detail":"resolve %s: settings service error in snapshot_bool: expected bool, got string","instance":"","code":"general.server"}`, firstGate), response.Body.String())
			notificationBudgetReadOnly(t, counter)
		})
	}
}

func TestNotificationPreferencesGateCancellationHTTPContract(t *testing.T) {
	t.Parallel()
	for _, portal := range []string{notifications.PortalStaff, notifications.PortalSchool} {
		t.Run(portal, func(t *testing.T) {
			db := testpkg.SetupIsolatedTestDB(t)
			database := testpkg.NewDatabase(t, db)
			_, account := testpkg.CreateTestStaffWithAccount(t, db, "Cancelled", "Preferences")
			claims := testutil.TeacherTestClaims(int(account.ID))
			path, firstType := "/api/notifications/preferences/", notifications.TypePickupUpcoming
			if portal == notifications.PortalSchool {
				claims.Scope, path, firstType = "school", "/school/notifications/preferences/", notifications.TypeSchoolClassAbsenceReported
			}
			request := testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)))
			ctx, cancel := context.WithCancel(request.Context())
			defer cancel()
			request = request.WithContext(ctx)
			var calls []string
			source := &settingsDouble{ResolveBoolFn: func(ctx context.Context, key string) (bool, error) {
				calls = append(calls, key)
				if len(calls) == 1 {
					cancel()
					return true, nil
				}
				return false, ctx.Err()
			}}
			module := api.DeliveryTestModule{NotificationPreferences: notifications.NewPreferenceService(api.NewNotificationConsentTestStore(database), source, nil, nil, securityruntime.HasPermission)}
			counter := testpkg.CaptureQueries(t, db)
			response := testutil.ExecuteRequestForTest(t, module.NotificationPreferenceRouterForTests(), request)
			counter.Stop()
			notificationBudgetLog(t, portal+"_gate_cancelled", 0, counter.Measurement(counter.Queries()).Statements)
			notificationBudgetWireLog(t, portal+"_gate_cancelled", 0, response.Code, response.Header(), response.Body.String(), counter.WriteRows())
			def, known := notifications.GetType(firstType)
			require.True(t, known)
			require.Equal(t, []string{"notifications.dispatch_enabled", def.TenantGate}, calls)
			// The client went away: 499, no server error, no buffered success.
			require.Equal(t, 499, response.Code)
			require.Equal(t, "application/json", response.Header().Get("Content-Type"))
			require.JSONEq(t, fmt.Sprintf(`{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-gerade-nicht-erreichbar","title":"Client Closed Request","status":499,"detail":"resolve %s: context canceled","instance":"","code":"general.unavailable"}`, def.TenantGate), response.Body.String())
			notificationBudgetReadOnly(t, counter)
		})
	}
}
