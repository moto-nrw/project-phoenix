//go:build testhooks

package notifications_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// N is the number of persisted decisions for this account, not the fixed-size
// catalogue. The school catalogue deliberately ignores stored OGS-only types.
func TestStaffNotificationPreferencesHTTPQueryBudget(t *testing.T) {
	t.Parallel()
	notificationPreferencesStaffBudget(t, false)
}

func TestSchoolNotificationPreferencesHTTPQueryBudget(t *testing.T) {
	t.Parallel()
	notificationPreferencesStaffBudget(t, true)
}

func notificationPreferencesStaffBudget(t *testing.T, school bool) {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	database := testpkg.NewDatabase(t, db)
	module, err := api.NewDeliveryTestModule(database, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	router := module.NotificationPreferenceRouterForTests()
	_, account := testpkg.CreateTestStaffWithAccount(t, db, "Budget", "Preferences")
	testpkg.AssignLehrkraftSystemRole(t, db, account.ID, testpkg.Tenant(t))
	claims := testutil.TeacherTestClaims(int(account.ID))
	claims.TenantID = testpkg.Tenant(t)
	path, portal := "/api/notifications/preferences/", "staff"
	if school {
		path, portal, claims.Scope = "/school/notifications/preferences/", "school", "school"
	}
	notificationBudgetSettings(t, db, testpkg.Tenant(t), true)
	// A matching account/type in another school must not affect this switch.
	foreign := testpkg.NewTenantScope(t, db)
	testpkg.MapAccountToTenant(t, db, account.ID, foreign.TenantID)
	notificationBudgetSettings(t, db, foreign.TenantID, false)
	consent := api.NewNotificationConsentTestStore(database)
	keys := []string{"staff_message", "school_class_absence_reported", "school_parent_conference_booking", "pickup_upcoming", "pickup_overdue", "activity_start", "activity_overdue", "activity_running_over"}
	for _, key := range keys {
		require.NoError(t, testpkg.TenantCommand3(t, foreign.Context(), db, consent.RecordConsent, account.ID, key, true))
	}
	other := testpkg.CreateTestAccount(t, db, "other-preferences")
	for _, key := range keys {
		require.NoError(t, testpkg.TenantCommand3(t, testpkg.Ctx(t), db, consent.RecordConsent, other.ID, key, true))
	}
	token := testutil.MintTestJWT(t, claims)
	expected := notificationBudgetCatalogue(t, portal)
	counter := testpkg.CaptureQueries(t, db)
	previous, previousSQL := 0, 0
	for _, n := range []int{0, 3, 8} {
		for i := previous; i < n; i++ {
			enabled := i%2 == 0
			require.NoError(t, testpkg.TenantCommand3(t, testpkg.Ctx(t), db, consent.RecordConsent, account.ID, keys[i], enabled))
			for _, row := range expected {
				if row["key"] == keys[i] {
					row["enabled"] = enabled
				}
			}
		}
		count, countErr := db.NewSelect().TableExpr("users.notification_preferences").Where("account_id = ?", account.ID).Where("tenant_id = ?", testpkg.Tenant(t)).Count(context.Background())
		require.NoError(t, countErr)
		require.Equal(t, int64(n), count)
		rowsBefore := notificationBudgetRows(t, db)
		counter.Reset()
		request := testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, testutil.WithJWTBearer(token))
		response := testutil.ExecuteRequestForTest(t, router, request)
		counter.Stop()
		notificationBudgetLog(t, portal, n, counter.Queries())
		if school {
			querybudgets.AssertQueryBudget(t, "school.notifications.preferences", counter.Queries())
		} else {
			querybudgets.AssertQueryBudget(t, "api.notifications.preferences", counter.Queries())
		}
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		notificationBudgetAssertResponse(t, response.Body.Bytes(), expected, true)
		notificationBudgetWireLog(t, portal, n, response.Code, response.Header(), response.Body.String(), counter.WriteRows())
		notificationBudgetReadOnly(t, counter)
		require.Len(t, counter.Selects("users.notification_preferences"), 1)
		require.Equal(t, rowsBefore, notificationBudgetRows(t, db), "GET preserves all consent rows and timestamps")
		if previousSQL > 0 {
			require.Len(t, counter.Queries(), previousSQL, "decision rows must not add round trips")
		}
		previousSQL = len(counter.Queries())
		previous = n
		counter.Start()
	}
	counter.Stop()
	notificationBudgetRejectWrongPortal(t, router, path, account.ID, school)
}

// Parent GET intentionally aggregates two active guardian schools. N counts
// decisions distributed across them (0/3/8), not school fan-out. One extra
// staff-only mapping and one unrelated guardian's school are exclusion probes.
func TestParentNotificationPreferencesHTTPQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	database := testpkg.NewDatabase(t, db)
	module, err := api.NewDeliveryTestModule(database, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	router := module.NotificationPreferenceRouterForTests()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	second := testpkg.NewTenantScope(t, db)
	testpkg.MapAccountToTenant(t, db, chain.AccountID, second.TenantID)
	notificationBudgetGuardianRole(t, db, chain.AccountID, second.TenantID)
	notificationBudgetSettings(t, db, chain.TenantID, true)
	notificationBudgetSettings(t, db, second.TenantID, false)
	staffOnly := testpkg.NewTenantScope(t, db)
	testpkg.MapAccountToTenant(t, db, chain.AccountID, staffOnly.TenantID)
	unrelated := testpkg.CreateTestParentGuardianChain(t, db)
	foreign := testpkg.NewTenantScope(t, db)
	testpkg.MapAccountToTenant(t, db, unrelated.AccountID, foreign.TenantID)
	notificationBudgetGuardianRole(t, db, unrelated.AccountID, foreign.TenantID)
	consent := api.NewNotificationConsentTestStore(database)
	// Opposite decisions in non-guardian/unrelated schools cannot enable the
	// current guardian's switches, even for this same account in staffOnly.
	for _, id := range []int64{staffOnly.TenantID, foreign.TenantID} {
		ctx := testpkg.ContextForTenant(context.Background(), id)
		for _, key := range []string{"parent_announcement", "parent_message", "parent_request_decided"} {
			require.NoError(t, testpkg.TenantCommand3(t, ctx, db, consent.RecordConsent, chain.AccountID, key, true))
		}
		notificationBudgetSettings(t, db, id, true)
	}
	type decision struct {
		tenant  int64
		key     string
		enabled bool
	}
	decisions := []decision{
		{chain.TenantID, "parent_announcement", true},
		{second.TenantID, "parent_announcement", true},
		{chain.TenantID, "parent_message", true},
		{second.TenantID, "parent_message", false},
		{chain.TenantID, "parent_appointment", true},
		{second.TenantID, "parent_appointment", true},
		{chain.TenantID, "parent_request_decided", false},
		{second.TenantID, "parent_request_decided", true},
	}
	token := testutil.MintTestJWT(t, testutil.ParentTestClaims(int(chain.AccountID)))
	expected := notificationBudgetCatalogue(t, "parent")
	counter := testpkg.CaptureQueries(t, db)
	previous, previousSQL := 0, 0
	for _, n := range []int{0, 3, 8} {
		for _, row := range decisions[previous:n] {
			ctx := testpkg.ContextForTenant(context.Background(), row.tenant)
			require.NoError(t, testpkg.TenantCommand3(t, ctx, db, consent.RecordConsent, chain.AccountID, row.key, row.enabled))
		}
		for _, row := range expected {
			row["enabled"] = (n >= 3 && row["key"] == "parent_announcement") || (n == 8 && row["key"] == "parent_appointment")
		}
		count, countErr := db.NewSelect().TableExpr("users.notification_preferences").Where("account_id = ?", chain.AccountID).Where("tenant_id IN (?, ?)", chain.TenantID, second.TenantID).Count(context.Background())
		require.NoError(t, countErr)
		require.Equal(t, int64(n), count)
		rowsBefore := notificationBudgetRows(t, db)
		counter.Reset()
		request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/parent/me/notification-preferences/", nil, testutil.WithJWTBearer(token))
		response := testutil.ExecuteRequestForTest(t, router, request)
		counter.Stop()
		notificationBudgetLog(t, "parent", n, counter.Queries())
		querybudgets.AssertQueryBudget(t, "parent.me.notification_preferences", counter.Queries())
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		notificationBudgetAssertResponse(t, response.Body.Bytes(), expected, true)
		notificationBudgetReadOnly(t, counter)
		require.Len(t, counter.Selects("users.notification_preferences"), 2, "only two active guardian schools")
		require.Equal(t, rowsBefore, notificationBudgetRows(t, db))
		if previousSQL > 0 {
			require.Len(t, counter.Queries(), previousSQL)
		}
		previous, previousSQL = n, len(counter.Queries())
		counter.Start()
	}
	counter.Stop()
	// A parent account with no guardian school still receives the complete
	// catalogue, entirely unavailable/off, without reading anyone else's rows.
	empty := testpkg.CreateTestAccount(t, db, "no-guardian-school")
	emptyExpected := notificationBudgetCatalogue(t, "parent")
	for _, row := range emptyExpected {
		row["available"] = false
	}
	counter.Reset()
	counter.Start()
	request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/parent/me/notification-preferences/", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.ParentTestClaims(int(empty.ID)))))
	response := testutil.ExecuteRequestForTest(t, router, request)
	counter.Stop()
	notificationBudgetLog(t, "parent-no-schools", 0, counter.Queries())
	querybudgets.AssertQueryBudget(t, "parent.me.notification_preferences.no_schools", counter.Queries())
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	notificationBudgetAssertResponse(t, response.Body.Bytes(), emptyExpected, false)
	notificationBudgetReadOnly(t, counter)
	require.Empty(t, counter.Selects("users.notification_preferences"))
	wrong := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/parent/me/notification-preferences/", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.TeacherTestClaims(int(chain.AccountID)))))
	require.Equal(t, http.StatusUnauthorized, testutil.ExecuteRequestForTest(t, router, wrong).Code)
	unauthenticated := testutil.NewRequest(http.MethodGet, "/parent/me/notification-preferences/", nil)
	require.Equal(t, http.StatusUnauthorized, testutil.ExecuteRequestForTest(t, router, unauthenticated).Code)
}

func notificationBudgetSettings(t *testing.T, db *bun.DB, tenantID int64, primary bool) {
	t.Helper()
	for _, key := range []string{"notifications.absence_reported_enabled", "reminders.pickup_upcoming_enabled", "reminders.activity_start_enabled", "reminders.activity_overdue_enabled", "reminders.activity_running_over_enabled", "module.eltern_app", "module.unterricht_vormittag"} {
		testpkg.SetTestTenantSetting(t, db, tenantID, key, "true")
	}
	testpkg.SetTestTenantSetting(t, db, tenantID, "notifications.dispatch_enabled", fmt.Sprint(primary))
	testpkg.SetTestTenantSetting(t, db, tenantID, "reminders.pickup_overdue_enabled", "false")
	testpkg.SetTestTenantSetting(t, db, tenantID, "operations.staff_messaging_enabled", "false")
	testpkg.SetTestTenantSetting(t, db, tenantID, "operations.parent_notes_enabled", fmt.Sprint(primary))
	testpkg.SetTestTenantSetting(t, db, tenantID, "calendar.appointment_reminder_enabled", "true")
}

func notificationBudgetGuardianRole(t *testing.T, db *bun.DB, accountID, tenantID int64) {
	t.Helper()
	// The catalog has no second-school guardian helper. Reuse its existing
	// seeded-role arrangement, without importing Identity's implementation.
	_, err := db.NewRaw(`INSERT INTO auth.account_roles (account_id, role_id, tenant_id)
        SELECT ?, id, ? FROM auth.roles WHERE name = 'guardian'`, accountID, tenantID).Exec(context.Background())
	require.NoError(t, err)
}

func notificationBudgetRows(t *testing.T, db *bun.DB) string {
	t.Helper()
	var rows string
	require.NoError(t, db.NewRaw(`SELECT COALESCE(jsonb_agg(to_jsonb(p) ORDER BY p.id), '[]'::jsonb)::text FROM users.notification_preferences p`).Scan(context.Background(), &rows))
	return rows
}

func notificationBudgetAssertResponse(t *testing.T, body []byte, expected []map[string]any, enabled bool) {
	t.Helper()
	want, err := json.Marshal(map[string]any{"status": "success", "message": "Notification preferences retrieved successfully",
		"data": map[string]any{"tenant_enabled": enabled, "types": expected}})
	require.NoError(t, err)
	require.JSONEq(t, string(want), string(body), "exact DTO fields, catalogue ordering, availability and opt-in decisions")
}

func notificationBudgetReadOnly(t *testing.T, counter *testpkg.QueryCounter) {
	t.Helper()
	for _, op := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
		require.Empty(t, counter.Operation(op), op)
	}
	require.Zero(t, counter.WriteRows())
	for _, sql := range counter.Queries() {
		upper := strings.ToUpper(sql)
		require.NotContains(t, upper, "FOR UPDATE")
		require.NotContains(t, upper, "FOR SHARE")
		require.NotContains(t, upper, "PG_ADVISORY")
	}
}

func notificationBudgetLog(t *testing.T, portal string, n int, queries []string) {
	t.Helper()
	encoded, err := json.Marshal(queries)
	require.NoError(t, err)
	t.Logf("notification_preferences portal=%s n=%d statements=%d queries_json=%s", portal, n, len(queries), encoded)
}

func notificationBudgetRejectWrongPortal(t *testing.T, router chi.Router, path string, accountID int64, school bool) {
	t.Helper()
	claims := testutil.ParentTestClaims(int(accountID))
	if school {
		claims = testutil.TeacherTestClaims(int(accountID))
	}
	request := testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)))
	require.Equal(t, http.StatusUnauthorized, testutil.ExecuteRequestForTest(t, router, request).Code)
	unauthenticated := testutil.NewRequest(http.MethodGet, path, nil)
	require.Equal(t, http.StatusUnauthorized, testutil.ExecuteRequestForTest(t, router, unauthenticated).Code)
}

// These frozen wire catalogues deliberately do not call the implementation's
// type registry: a removed/reordered type or DTO field must fail this test.
func notificationBudgetCatalogue(t *testing.T, portal string) []map[string]any {
	t.Helper()
	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(notificationBudgetCatalogues[portal]), &rows))
	return rows
}

var notificationBudgetCatalogues = map[string]string{
	"staff": `[
  {
    "key": "pickup_upcoming",
    "label": "Anstehende Abholung",
    "description": "Kurz bevor ein Kind abgeholt wird, das Sie gerade betreuen.",
    "group": "abholung",
    "enabled": false,
    "available": true
  },
  {
    "key": "pickup_overdue",
    "label": "Überfällige Abholung",
    "description": "Wenn ein Kind nach seiner Abholzeit noch da ist.",
    "group": "abholung",
    "enabled": false,
    "available": false
  },
  {
    "key": "activity_start",
    "label": "Aktivität in Ihrem Raum",
    "description": "Kurz bevor eine geplante Aktivität in einem Raum startet, den Sie gerade beaufsichtigen.",
    "group": "aktivitaeten",
    "enabled": false,
    "available": true
  },
  {
    "key": "activity_overdue",
    "label": "Aktivität nicht gestartet",
    "description": "Wenn eine geplante Aktivität überfällig ist und noch niemand sie gestartet hat.",
    "group": "aktivitaeten",
    "enabled": false,
    "available": true
  },
  {
    "key": "activity_running_over",
    "label": "AG dauert länger",
    "description": "Wenn eine gestartete Aktivität in einem Raum Ihrer Aufsicht nach dem geplanten Ende noch läuft.",
    "group": "aktivitaeten",
    "enabled": false,
    "available": true
  },
  {
    "key": "my_activity_starting",
    "label": "Ihr nächster Einsatz",
    "description": "Kurz bevor ein Einsatz aus Ihrem Betreuungsplan beginnt, auch wenn Sie gerade keine Aufsicht führen.",
    "group": "aktivitaeten",
    "enabled": false,
    "available": true
  },
  {
    "key": "student_absence_reported",
    "label": "Krankmeldung eines Kindes",
    "description": "Wenn für ein Kind aus Ihren Gruppen eine Krankmeldung oder Entschuldigung für heute eingetragen wird.",
    "group": "kinder",
    "enabled": false,
    "available": true
  },
  {
    "key": "staff_parent_message",
    "label": "Nachrichten von Eltern",
    "description": "Wenn Eltern der OGS eine neue Nachricht schreiben.",
    "group": "mitteilungen",
    "enabled": false,
    "available": true
  },
  {
    "key": "staff_message",
    "label": "Nachrichten von Kolleginnen und Kollegen",
    "description": "Wenn Ihnen jemand aus dem Team eine Nachricht im Team-Chat schreibt.",
    "group": "mitteilungen",
    "enabled": false,
    "available": false
  },
  {
    "key": "staff_survey",
    "label": "Umfragen der Einrichtung",
    "description": "Wenn die Einrichtung eine Umfrage an das Team veröffentlicht oder an eine offene Umfrage erinnert.",
    "group": "mitteilungen",
    "enabled": false,
    "available": true
  },
  {
    "key": "staff_qualification_own_expiry",
    "label": "Eigene Nachweise",
    "description": "Wenn einer Ihrer Nachweise bald abläuft und am Ablauftag. Eine E-Mail bekommen Sie in jedem Fall.",
    "group": "mitteilungen",
    "enabled": false,
    "available": true
  }
]`,
	"school": `[
  {
    "key": "school_class_absence_reported",
    "label": "Krankmeldungen meiner Klassen",
    "description": "Wenn ein Kind aus Ihren Klassen für heute krankgemeldet oder entschuldigt wird.",
    "group": "kinder",
    "enabled": false,
    "available": true
  },
  {
    "key": "staff_message",
    "label": "Nachrichten von Kolleginnen und Kollegen",
    "description": "Wenn Ihnen jemand aus dem Team eine Nachricht im Team-Chat schreibt.",
    "group": "mitteilungen",
    "enabled": false,
    "available": false
  },
  {
    "key": "school_parent_conference_booking",
    "label": "Buchungen am Elternsprechtag",
    "description": "Wenn Eltern einen Termin an Ihrem Elternsprechtag buchen, verschieben oder absagen.",
    "group": "termine",
    "enabled": false,
    "available": true
  }
]`,
	"parent": `[
  {
    "key": "parent_request_decided",
    "label": "Entscheidung über Ihre Anfrage",
    "description": "Wenn die OGS über eine Ihrer Anfragen entschieden hat, etwa zu Betreuungszeiten, Stammdaten oder einer Abmeldung.",
    "group": "kinder",
    "enabled": false,
    "available": false
  },
  {
    "key": "parent_announcement",
    "label": "Neue Mitteilung der Schule",
    "description": "Wenn die Schule eine neue Mitteilung im Elternportal veröffentlicht.",
    "group": "mitteilungen",
    "enabled": false,
    "available": true
  },
  {
    "key": "parent_message",
    "label": "Neue Nachricht der OGS",
    "description": "Wenn die OGS Ihnen zu einem Kind schreibt.",
    "group": "mitteilungen",
    "enabled": false,
    "available": false
  },
  {
    "key": "parent_billing_document",
    "label": "Neues Dokument vom Träger",
    "description": "Wenn der Träger Ihnen ein Dokument schickt, zum Beispiel ein Bestätigungsschreiben.",
    "group": "mitteilungen",
    "enabled": false,
    "available": true
  },
  {
    "key": "parent_appointment",
    "label": "Neuer oder geänderter Termin",
    "description": "Wenn ein Termin für Sie angelegt, geändert oder abgesagt wird.",
    "group": "termine",
    "enabled": false,
    "available": true
  },
  {
    "key": "parent_appointment_reminder",
    "label": "Terminerinnerung",
    "description": "Kurz vor einem Termin, zu dem Sie eingeladen sind.",
    "group": "termine",
    "enabled": false,
    "available": true
  },
  {
    "key": "parent_care_cancelled",
    "label": "Betreuung fällt aus",
    "description": "Wenn die OGS einen Betreuungstermin Ihres Kindes absagt.",
    "group": "termine",
    "enabled": false,
    "available": true
  }
]`,
}
