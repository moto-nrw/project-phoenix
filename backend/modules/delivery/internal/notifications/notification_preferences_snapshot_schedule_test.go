//go:build testhooks

package notifications_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/notifications"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type preferenceGatePair struct {
	portal, first, second, firstType, secondType string
}

// The two settings calls always delegate to the real Settings owner. Only
// the point at which the independent public writer commits is controlled.
type preferenceGateSchedule struct {
	notifications.PreferenceSettings
	first   string
	after   func(context.Context)
	reads   []preferenceGateRead
	barrier error
}

type preferenceGateRead struct {
	Keys   []string
	Values map[string]bool
	Error  string
}

func (s *preferenceGateSchedule) ResolveBool(ctx context.Context, key string) (bool, error) {
	value, err := s.PreferenceSettings.ResolveBool(ctx, key)
	s.reads = append(s.reads, preferenceGateRead{[]string{key}, map[string]bool{key: value}, fmt.Sprint(err)})
	if key == s.first && s.after != nil {
		s.after(ctx)
	}
	return value, err
}

func (s *preferenceGateSchedule) ResolveBools(ctx context.Context, keys []string) (map[string]bool, error) {
	values, err := s.PreferenceSettings.ResolveBools(ctx, keys)
	s.reads = append(s.reads, preferenceGateRead{append([]string(nil), keys...), values, fmt.Sprint(err)})
	// For the rejected candidate, the same boundary is after the real query
	// containing the first gate, not a replacement value or scalar simulation.
	for _, key := range keys {
		if key == s.first && s.after != nil {
			s.after(ctx)
			break
		}
	}
	return values, err
}

type preferenceReadObservation struct {
	devices.StaffNotificationPreferences
	err error
}

func (s *preferenceReadObservation) GetForAccount(ctx context.Context, accountID int64, portal string) (*devices.PreferenceOverview, error) {
	value, err := s.StaffNotificationPreferences.GetForAccount(ctx, accountID, portal)
	s.err = err
	return value, err
}

type preferenceServiceObservation struct {
	deliveryCompose.NotificationPreferenceService
	staff *preferenceReadObservation
}

func (s preferenceServiceObservation) StaffPreferences() devices.StaffNotificationPreferences {
	return s.staff
}

type preferenceWriterOutcome struct {
	Attempted []string
	Completed []string
	Error     string
}

// This is a normal contract test, not a latency measurement. No settings
// writer lock is held by the GET. Both commits use public SetValue, with a
// fixture account's effective school permissions, in one separate tenant unit.
func TestNotificationPreferencesPublicWriterSnapshotSchedule(t *testing.T) {
	t.Parallel()
	pairs := []preferenceGatePair{
		{"staff", "reminders.pickup_upcoming_enabled", "reminders.pickup_overdue_enabled", "pickup_upcoming", "pickup_overdue"},
		{"school", "notifications.absence_reported_enabled", "operations.staff_messaging_enabled", "school_class_absence_reported", "staff_message"},
	}
	for _, pair := range pairs {
		for _, schedule := range []string{"stable_old", "stable_new", "between_reads"} {
			t.Run(pair.portal+"/"+schedule, func(t *testing.T) {
				preferencePublicWriterSchedule(t, pair, schedule)
			})
		}
	}
}

func preferencePublicWriterSchedule(t *testing.T, pair preferenceGatePair, schedule string) {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	database, unit := testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db)
	settings, err := owners.NewSettingsTestModule(database, unit)
	require.NoError(t, err)
	identity, err := api.NewIdentityAccessForTests(database)
	require.NoError(t, err)
	_, account := testpkg.CreateTestStaffWithAccount(t, db, "Snapshot", "Reader")
	testpkg.AssignLehrkraftSystemRole(t, db, account.ID, testpkg.Tenant(t))
	_, writer := testpkg.CreateTestStaffWithAccount(t, db, "Settings", "Writer")
	for _, permission := range []string{"config:update", "config:manage"} {
		testpkg.GrantTestPermission(t, db, testpkg.Tenant(t), writer.ID, permission)
	}
	permissions, err := testpkg.TenantQuery2(t, testpkg.Ctx(t), db, identity.FindEffectivePermissionNamesByAccountIDsForTenant, []int64{writer.ID}, testpkg.Tenant(t))
	require.NoError(t, err)
	require.Contains(t, permissions[writer.ID], "config:update")
	require.Contains(t, permissions[writer.ID], "config:manage")
	writePair := func(ctx context.Context, old bool, outcome *preferenceWriterOutcome) error {
		return settings.Runtime.WithinTenant(ctx, testpkg.Tenant(t), func(txCtx context.Context) error {
			for i, key := range []string{pair.first, pair.second} {
				outcome.Attempted = append(outcome.Attempted, key)
				if err := settings.Settings.SetValue(txCtx, key, (i == 0) == old, &writer.ID, permissions[writer.ID]); err != nil {
					return err
				}
				outcome.Completed = append(outcome.Completed, key)
			}
			return nil
		})
	}
	preferenceScheduleArrange(t, db, settings, writer.ID, permissions[writer.ID])
	require.NoError(t, writePair(testpkg.Ctx(t), true, &preferenceWriterOutcome{}))
	consent := api.NewNotificationConsentTestStore(database)
	for _, key := range []string{pair.firstType, pair.secondType} {
		require.NoError(t, testpkg.TenantCommand3(t, testpkg.Ctx(t), db, consent.RecordConsent, account.ID, key, true))
	}
	preferenceRunSchedule(t, db, settings, identity, consent, pair, schedule, account.ID, writer.ID, permissions[writer.ID], writePair)
}

func preferenceScheduleArrange(t *testing.T, db *bun.DB, settings owners.SettingsTestModule, actor int64, permissions []string) {
	t.Helper()
	values := []struct {
		key   string
		value bool
	}{
		{"notifications.dispatch_enabled", true}, {"notifications.absence_reported_enabled", true},
		{"reminders.activity_start_enabled", true}, {"reminders.activity_overdue_enabled", true},
		{"reminders.activity_running_over_enabled", true}, {"operations.parent_notes_enabled", true},
		{"operations.staff_messaging_enabled", false},
	}
	require.NoError(t, testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context) error {
		for _, value := range values {
			if err := settings.Settings.SetValue(ctx, value.key, value.value, &actor, permissions); err != nil {
				return err
			}
		}
		return nil
	}))
}

func preferenceScheduleState(t *testing.T, ctx context.Context, db *bun.DB, tenantID int64) (string, error) {
	t.Helper()
	var state string
	err := testpkg.WithTenantTx(t, ctx, db, tenantID, func(txCtx context.Context, tx bun.Tx) error {
		var err error
		state, err = testpkg.ReadNotificationPreferenceWriterSnapshot(txCtx, tx, tenantID)
		return err
	})
	return state, err
}

func preferenceScheduleReadOnly(t *testing.T, counter *testpkg.QueryCounter) {
	t.Helper()
	for _, op := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
		require.Empty(t, counter.Operation(op), "only the marked GET must have DML=0")
	}
	require.Zero(t, counter.WriteRows())
	for _, query := range counter.Queries() {
		upper := strings.ToUpper(query)
		for _, lock := range []string{"FOR UPDATE", "FOR SHARE", "PG_ADVISORY"} {
			require.NotContains(t, upper, lock)
		}
	}
}

func preferenceRunSchedule(t *testing.T, db *bun.DB, settings owners.SettingsTestModule, identity notifications.GuardianSchools,
	consent deliveryCompose.NotificationConsentStore, pair preferenceGatePair, schedule string, accountID, writerID int64,
	permissions []string, writePair func(context.Context, bool, *preferenceWriterOutcome) error,
) {
	t.Helper()
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 15*time.Second)
	defer cancel()
	allSQL := testpkg.CaptureQueries(t, db)
	getSQL := testpkg.CaptureQueriesForContext(t, db)
	writerSQL := testpkg.CaptureQueriesForContext(t, db)
	before, beforeErr := preferenceScheduleState(t, ctx, db, testpkg.Tenant(t))
	start, done := make(chan struct{}), make(chan struct{})
	var once sync.Once
	var outcome preferenceWriterOutcome
	go func() {
		defer close(done)
		select {
		case <-start:
			outcome.Error = fmt.Sprint(writePair(writerSQL.Context(ctx), false, &outcome))
		case <-ctx.Done():
			outcome.Error = ctx.Err().Error()
		}
	}()
	defer func() {
		cancel()
		<-done // The writer and every DB call share the bounded context.
	}()
	trigger := func() { once.Do(func() { close(start) }) }
	wait := func() error {
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	source := &preferenceGateSchedule{PreferenceSettings: settings.Settings, first: pair.first}
	if schedule == "between_reads" {
		source.after = func(context.Context) {
			trigger()
			source.barrier = wait()
		}
	}
	service, err := deliveryCompose.NewTenantPreferenceService(testpkg.TenantRuntime(t, db), consent, source, db, identity, securityruntime.HasPermission)
	require.NoError(t, err)
	observed := &preferenceReadObservation{StaffNotificationPreferences: service.StaffPreferences()}
	module := api.DeliveryTestModule{NotificationPreferences: preferenceServiceObservation{service, observed}}
	claims := testutil.TeacherTestClaims(int(accountID))
	path := "/api/notifications/preferences/"
	if pair.portal == "school" {
		claims.Scope, path = "school", "/school/notifications/preferences/"
	}
	request := testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)))
	request = request.WithContext(getSQL.Context(ctx))
	if schedule == "stable_new" {
		trigger()
		source.barrier = wait()
	}
	response := testutil.ExecuteRequestForTest(t, module.NotificationPreferenceRouterForTests(), request)
	getSQL.Stop()
	// Also release a writer if a failed GET never reached its first gate.
	trigger()
	waitErr := wait()
	cancel()
	<-done
	writerSQL.Stop()
	after, afterErr := preferenceScheduleState(t, testpkg.Ctx(t), db, testpkg.Tenant(t))
	allSQL.Stop()
	expected := preferenceScheduleExpected(t, pair, schedule)
	// Preserve every observation before response, schedule or read-only asserts.
	evidence, encodeErr := json.Marshal(map[string]any{
		"portal": pair.portal, "schedule": schedule, "status": response.Code, "headers": response.Header(), "body": response.Body.String(),
		"expected_body": expected, "inner_error": fmt.Sprint(observed.err), "settings_reads": source.reads,
		"writer": outcome, "writer_actor": writerID, "writer_permissions": permissions,
		"get_sql": getSQL.Queries(), "get_write_rows": getSQL.WriteRows(),
		"writer_sql": writerSQL.Queries(), "writer_write_rows": writerSQL.WriteRows(), "all_sql": allSQL.Queries(),
		"before_state": before, "after_state": after, "before_error": fmt.Sprint(beforeErr), "after_error": fmt.Sprint(afterErr),
		"barrier_error": fmt.Sprint(source.barrier), "wait_error": fmt.Sprint(waitErr),
	})
	t.Logf("preference_public_writer_evidence=%s encode_error=%v", evidence, encodeErr)
	require.NoError(t, encodeErr)
	require.NoError(t, beforeErr)
	require.NoError(t, afterErr)
	require.NoError(t, waitErr)
	require.NoError(t, source.barrier)
	require.NoError(t, observed.err)
	preferenceScheduleAssertWriter(t, pair, writerID, outcome, writerSQL, before, after)
	preferenceScheduleReadOnly(t, getSQL)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, http.Header{"Content-Type": []string{"application/json"}}, response.Header())
	require.JSONEq(t, expected, response.Body.String(), "full ordered DTO, including the two independently resolved gate values")
}

func preferenceScheduleAssertWriter(t *testing.T, pair preferenceGatePair, writerID int64, outcome preferenceWriterOutcome, counter *testpkg.QueryCounter, before, after string) {
	t.Helper()
	require.Equal(t, "<nil>", outcome.Error)
	require.Equal(t, []string{pair.first, pair.second}, outcome.Attempted)
	require.Equal(t, outcome.Attempted, outcome.Completed)
	require.Len(t, counter.Operation("BEGIN"), 1, "both SetValue calls share one independently committed tenant transaction")
	require.Len(t, counter.Operation("COMMIT"), 1)
	require.Empty(t, counter.Operation("ROLLBACK"))
	require.Len(t, counter.Operation("INSERT"), 4, "two settings upserts and two audit entries")
	const expectedWriterRows int64 = 4
	require.Equal(t, expectedWriterRows, counter.WriteRows())
	var oldState, newState struct {
		Values []struct {
			Key   string `json:"setting_key"`
			Value bool   `json:"value"`
			Actor int64  `json:"updated_by"`
		} `json:"values"`
		Audit []struct {
			Key   string `json:"setting_key"`
			Old   bool   `json:"old_value"`
			New   bool   `json:"new_value"`
			Actor int64  `json:"changed_by"`
		} `json:"audit"`
		Consent   json.RawMessage `json:"consent"`
		Isolation string          `json:"default_isolation"`
	}
	require.NoError(t, json.Unmarshal([]byte(before), &oldState))
	require.NoError(t, json.Unmarshal([]byte(after), &newState))
	require.Equal(t, "read committed", newState.Isolation)
	require.JSONEq(t, string(oldState.Consent), string(newState.Consent), "GET and settings writer preserve full consent rows and timestamps")
	require.Len(t, newState.Audit, len(oldState.Audit)+2)
	for i, key := range []string{pair.first, pair.second} {
		found := false
		for _, value := range newState.Values {
			if value.Key == key {
				found = true
				require.Equal(t, i == 1, value.Value, key)
				require.Equal(t, writerID, value.Actor, key)
			}
		}
		require.True(t, found, key)
		audit := newState.Audit[len(oldState.Audit)+i]
		require.Equal(t, key, audit.Key)
		require.Equal(t, i == 0, audit.Old)
		require.Equal(t, i == 1, audit.New)
		require.Equal(t, writerID, audit.Actor)
	}
}
