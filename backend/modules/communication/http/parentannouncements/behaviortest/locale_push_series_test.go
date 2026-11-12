package announcement_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	"github.com/moto-ogs/project-phoenix/modules/communication"
	announcement "github.com/moto-ogs/project-phoenix/modules/communication/http/parentannouncements"
	"github.com/moto-ogs/project-phoenix/modules/delivery/devices"
	"github.com/moto-ogs/project-phoenix/modules/delivery/objects"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The language variants of one parent push (#5869 poll reminder, #5876
// publication) go through the production router, the durable Web Push
// channel and the real outbox. Every guardian has one device; the push rows
// are the delivery history these tests compare.

var seriesLocales = []string{"tr", "de", "en"}

type localePushFixture struct {
	db       *bun.DB
	ctx      context.Context
	settings interface {
		SetValue(context.Context, string, any, *int64, []string) error
	}
	router  chi.Router
	service communication.ParentAnnouncementCapability
	token   string
	pollID  int64
	// localeOf and subscriptionOf describe each guardian account.
	localeOf       map[int64]string
	subscriptionOf map[int64]int64
	counter        *testpkg.QueryCounter
}

func newLocalePushFixture(t *testing.T, locales []string, perLocale int) *localePushFixture {
	t.Helper()
	return newLocalePushFixtureFor(t, locales, perLocale, false)
}

// newLocalePushFixtureFor builds a poll, or with scheduled a plain
// announcement with a reminder an hour from now (polls take none).
func newLocalePushFixtureFor(t *testing.T, locales []string, perLocale int, scheduled bool) *localePushFixture {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	database := testpkg.NewDatabase(t, db)
	ctx := testpkg.Ctx(t)
	unit := testpkg.TenantRuntime(t, db)
	organizations, err := owners.NewOrganizationTenancy(database)
	require.NoError(t, err)
	repos := api.NewParentMessagingSuiteRepositories(database).WithSchoolProjections(organizations)
	settings, err := owners.NewSettingsTestModule(database, unit)
	require.NoError(t, err)
	for _, setting := range []struct {
		key   string
		value any
	}{
		{"operations.parent_news_enabled", true},
		{"notifications.dispatch_enabled", true},
		{"notifications.active_window_start", "00:00"},
		{"notifications.active_window_end", "00:00"},
		{"module.eltern_app", true},
	} {
		require.NoError(t, settings.Settings.SetValue(ctx, setting.key, setting.value, nil, nil), setting.key)
	}
	delivery, err := api.NewDeliveryTestModule(database, unit)
	require.NoError(t, err)
	notifier, err := api.NewDeliveryTestPushNotifier(database, unit, settings.Settings, delivery)
	require.NoError(t, err)
	uploads, err := objects.PrivateUploadsBackend()
	require.NoError(t, err)
	files, _, err := api.NewFileStoreTestModule(database, unit, uploads)
	require.NoError(t, err)
	service, err := owners.BuildParentAnnouncements(owners.RootEnvironment{Logger: slog.Default(), Links: owners.ServiceLinks{ParentsURL: "http://localhost"}},
		settings.Settings, delivery.EmailOutbox, delivery.Runtime.Module, delivery.Runtime.EmailOutcomes,
		owners.DeliveryNotifications{Notifier: notifier}, repos.MessagingRepositories, files)
	require.NoError(t, err)

	_, viewer := testpkg.CreateTestStaffWithAccount(t, db, "Teacher", "Locale")
	token := testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(viewer.ID), Sub: viewer.Email, TenantID: testpkg.Tenant(t), Roles: []string{"user"}, Permissions: []string{permissions.AdminWildcard}})
	input := communication.ParentAnnouncementInput{Title: "Locale poll", Body: "Body", ResponseType: "single_choice", Options: []string{"Ja", "Nein"}, SendEmail: true}
	if scheduled {
		reminderAt := time.Now().Add(time.Hour)
		input = communication.ParentAnnouncementInput{Title: "Locale notice", Body: "Body", SendEmail: true, ReminderAt: &reminderAt}
	}
	f := &localePushFixture{db: db, ctx: ctx, settings: settings.Settings, service: service, token: token, localeOf: map[int64]string{}, subscriptionOf: map[int64]int64{}}
	for _, locale := range locales {
		for i := 0; i < perLocale; i++ {
			chain := testpkg.CreateTestParentGuardianChain(t, db)
			_, err := db.NewRaw(`UPDATE users.guardian_profiles SET portal_locale = ? WHERE id = ?`, locale, chain.GuardianProfileID).Exec(ctx)
			require.NoError(t, err)
			var subscriptionID int64
			require.NoError(t, db.NewRaw(`INSERT INTO iot.push_subscriptions (tenant_id,account_id,portal,endpoint,p256dh,auth) VALUES (?,?,'parent',?,'p256dh-key','auth-key') RETURNING id`,
				chain.TenantID, chain.AccountID, fmt.Sprintf("https://fcm.googleapis.com/fcm/send/locale-%d", chain.AccountID)).Scan(ctx, &subscriptionID))
			f.localeOf[chain.AccountID] = locale
			f.subscriptionOf[chain.AccountID] = subscriptionID
			student := chain.StudentID
			input.Targets = append(input.Targets, communication.ParentAnnouncementTargetInput{TargetType: "student", RefID: &student})
		}
	}
	require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(c context.Context) error {
		a, e := service.CreateParentAnnouncement(c, viewer.ID, input)
		if e != nil {
			return e
		}
		f.pollID = a.ID
		return nil
	}))
	f.router = chi.NewRouter()
	f.router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	f.router.Mount("/api/parent-announcements", announcement.NewResource(service, nil).Router(api.ProtectedTenantRoutes))
	f.counter = testpkg.CaptureQueriesForContext(t, db)
	f.counter.Stop()
	return f
}

func (f *localePushFixture) post(t *testing.T, suffix string) (*httptest.ResponseRecorder, []string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/parent-announcements/%d%s", f.pollID, suffix), strings.NewReader("{}")).WithContext(f.counter.Context(f.ctx))
	req.Header.Set("Authorization", "Bearer "+f.token)
	req.Header.Set("Content-Type", "application/json")
	f.counter.Reset()
	f.counter.Start()
	rec := testutil.ExecuteRequestForTest(t, f.router, req)
	f.counter.Stop()
	return rec, f.counter.Queries()
}

// stored is the poll's committed state after a request.
type stored struct {
	push         []pushRow
	emails       int
	published    bool
	reminderSent bool
}

type pushRow struct {
	Key       string `json:"key"`
	Kind      string `json:"kind"`
	Related   string `json:"related"`
	Account   int64  `json:"account"`
	Sub       int64  `json:"subscription"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Status    string `json:"status"`
	Attempts  int    `json:"attempts"`
	DeepLink  string `json:"deep_link"`
	Priority  string `json:"priority"`
	HasLeases bool   `json:"has_lease"`
}

func (f *localePushFixture) stored(t *testing.T) stored {
	t.Helper()
	var rows []struct {
		Kind      string          `bun:"kind"`
		Key       *string         `bun:"idempotency_key"`
		Type      *string         `bun:"related_entity_type"`
		ID        *int64          `bun:"related_entity_id"`
		Recipient json.RawMessage `bun:"recipient"`
		Payload   json.RawMessage `bun:"payload"`
		Status    string          `bun:"status"`
		Attempts  int             `bun:"attempts"`
		Lease     *string         `bun:"lease_token"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT kind, idempotency_key, related_entity_type, related_entity_id, recipient, payload, status, attempts, lease_token
		FROM platform.push_outbox WHERE tenant_id = ?`, testpkg.Tenant(t)).Scan(f.ctx, &rows))
	var out stored
	for _, row := range rows {
		var recipient struct {
			SubscriptionID int64 `json:"subscription_id"`
			AccountID      int64 `json:"account_id"`
		}
		var payload struct {
			Title    string `json:"title"`
			Body     string `json:"body"`
			DeepLink string `json:"deepLink"`
			Priority string `json:"priority"`
		}
		require.NoError(t, json.Unmarshal(row.Recipient, &recipient))
		require.NoError(t, json.Unmarshal(row.Payload, &payload))
		require.NotNil(t, row.Key)
		require.NotNil(t, row.Type)
		require.NotNil(t, row.ID)
		out.push = append(out.push, pushRow{Key: *row.Key, Kind: row.Kind, Related: fmt.Sprintf("%s:%d", *row.Type, *row.ID), Account: recipient.AccountID, Sub: recipient.SubscriptionID,
			Title: payload.Title, Body: payload.Body, Status: row.Status, Attempts: row.Attempts, DeepLink: payload.DeepLink, Priority: payload.Priority, HasLeases: row.Lease != nil})
	}
	sort.Slice(out.push, func(i, j int) bool { return out.push[i].Key < out.push[j].Key })
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM platform.email_outbox WHERE tenant_id = ?`, testpkg.Tenant(t)).Scan(f.ctx, &out.emails))
	var state struct {
		PublishedAt    *time.Time `bun:"published_at"`
		ReminderSentAt *time.Time `bun:"reminder_sent_at"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT published_at, reminder_sent_at FROM users.parent_announcements WHERE id = ?`, f.pollID).Scan(f.ctx, &state))
	out.published = state.PublishedAt != nil
	out.reminderSent = state.ReminderSentAt != nil
	return out
}

func (f *localePushFixture) announcementStamp(t *testing.T) (time.Time, time.Time) {
	t.Helper()
	var row struct {
		UpdatedAt   time.Time `bun:"updated_at"`
		PublishedAt time.Time `bun:"published_at"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT updated_at, published_at FROM users.parent_announcements WHERE id = ?`, f.pollID).Scan(f.ctx, &row))
	return row.UpdatedAt, row.PublishedAt
}

// expectedPush is the history the language variants must leave: one row per
// device, its guardian's own language and a key of its own.
func (f *localePushFixture) expectedPush(keyOf func(locale string) string, kind string, related string) []pushRow {
	policy := devices.ParentDeliveryPolicy{}
	var rows []pushRow
	for account, locale := range f.localeOf {
		title, body := policy.ParentAnnouncementCopy(locale, kind)
		sub := f.subscriptionOf[account]
		rows = append(rows, pushRow{Key: fmt.Sprintf("%s:subscription:%d", keyOf(locale), sub), Kind: communication.TypeParentAnnouncement, Related: related, Account: account, Sub: sub,
			Title: title, Body: body, Status: "pending", DeepLink: "/", Priority: "normal"})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Key < rows[j].Key })
	return rows
}

func subscriptionReads(queries []string) int {
	n := 0
	for _, q := range queries {
		if strings.Contains(q, "delivery_push_subscriptions") {
			n++
		}
	}
	return n
}

func (f *localePushFixture) publishKey(t *testing.T) func(string) string {
	updatedAt, _ := f.announcementStamp(t)
	return func(locale string) string {
		return fmt.Sprintf("parent-announcement:%d:%d:%s:%s", f.pollID, updatedAt.UTC().UnixNano(), communication.ParentPollPublished, locale)
	}
}

func (f *localePushFixture) reminderKey(t *testing.T) func(string) string {
	updatedAt, _ := f.announcementStamp(t)
	return func(string) string {
		return fmt.Sprintf("parent-poll-reminder:%d:%d", f.pollID, updatedAt.UTC().UnixNano())
	}
}

// TestLocalePushSeriesKeepsEveryGuardiansLanguageWithOneDeviceRead pins the
// delivery history of publication and poll reminder for one and for three
// languages, and that the devices are read once per action.
func TestLocalePushSeriesKeepsEveryGuardiansLanguageWithOneDeviceRead(t *testing.T) {
	t.Parallel()
	for _, locales := range [][]string{{"de"}, seriesLocales} {
		t.Run(fmt.Sprintf("locales_%d", len(locales)), func(t *testing.T) {
			testpkg.OwnTenant(t)
			f := newLocalePushFixture(t, locales, 2)
			rec, publish := f.post(t, "/publish")
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			after := f.stored(t)
			published := f.expectedPush(f.publishKey(t), communication.ParentPollPublished, fmt.Sprintf("parent_announcement:%d", f.pollID))
			require.Equal(t, published, after.push)
			require.Equal(t, len(f.localeOf), after.emails)

			rec, remind := f.post(t, "/remind")
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Contains(t, rec.Body.String(), fmt.Sprintf(`"reminded_count":%d`, len(f.localeOf)))
			after = f.stored(t)
			reminded := f.expectedPush(f.reminderKey(t), communication.ParentPollReminder, fmt.Sprintf("parent_announcement_poll_reminder:%d", f.pollID))
			want := append(append([]pushRow{}, published...), reminded...)
			sort.Slice(want, func(i, j int) bool { return want[i].Key < want[j].Key })
			require.Equal(t, want, after.push)
			require.Equal(t, 2*len(f.localeOf), after.emails)

			t.Logf("languages=%d publish statements=%d device reads=%d; remind statements=%d device reads=%d",
				len(locales), len(publish), subscriptionReads(publish), len(remind), subscriptionReads(remind))
			require.Equal(t, 1, subscriptionReads(publish), "publication reads the devices of all languages once")
			require.Equal(t, 1, subscriptionReads(remind), "the poll reminder reads the devices of all languages once")
		})
	}
}

// failPushInsert makes the push enqueue for one device fail, or every push
// enqueue when subscriptionID is zero.
func (f *localePushFixture) failPushInsert(t *testing.T, subscriptionID int64) {
	t.Helper()
	condition := "TRUE"
	if subscriptionID > 0 {
		condition = fmt.Sprintf("NEW.idempotency_key LIKE '%%:subscription:%d'", subscriptionID)
	}
	_, err := f.db.ExecContext(f.ctx, `CREATE FUNCTION public.q4_fail_push() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected push failure'; END $$`)
	require.NoError(t, err)
	_, err = f.db.ExecContext(f.ctx, fmt.Sprintf(`CREATE TRIGGER q4_fail_push BEFORE INSERT ON platform.push_outbox FOR EACH ROW WHEN (%s) EXECUTE FUNCTION public.q4_fail_push()`, condition))
	require.NoError(t, err)
}

func (f *localePushFixture) failEmailInsert(t *testing.T) {
	t.Helper()
	_, err := f.db.ExecContext(f.ctx, `CREATE FUNCTION public.q4_fail_email() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected email failure'; END $$`)
	require.NoError(t, err)
	_, err = f.db.ExecContext(f.ctx, `CREATE TRIGGER q4_fail_email BEFORE INSERT ON platform.email_outbox FOR EACH ROW EXECUTE FUNCTION public.q4_fail_email()`)
	require.NoError(t, err)
}

// secondVariantDevice is a device of the language sent second, the point
// after which a failure would otherwise leave the first variant behind.
func (f *localePushFixture) secondVariantDevice() int64 {
	sorted := append([]string(nil), seriesLocales...)
	sort.Strings(sorted)
	for account, locale := range f.localeOf {
		if locale == sorted[1] {
			return f.subscriptionOf[account]
		}
	}
	return 0
}

func assertErrorEnvelope(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	require.Equal(t, status, rec.Code, rec.Body.String())
	var body struct {
		Status int    `json:"status"`
		Code   string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	require.Equal(t, status, body.Status)
	require.Equal(t, code, body.Code)
}

// TestLocalePushSeriesFailureLeavesNoPartialDelivery injects a failure at
// each point of a series: the device read, the first and the second language
// variant, and the e-mail after all pushes. Each answers 500 and leaves
// nothing behind: no push, no mail, no publication.
func TestLocalePushSeriesFailureLeavesNoPartialDelivery(t *testing.T) {
	t.Parallel()
	points := []struct {
		name   string
		inject func(*testing.T, *localePushFixture)
		remind bool
	}{
		{"publish/device_read", func(t *testing.T, f *localePushFixture) {
			_, err := f.db.ExecContext(f.ctx, `REVOKE SELECT ON platform.delivery_push_subscriptions FROM phoenix_tenant`)
			require.NoError(t, err)
		}, false},
		{"publish/first_variant", func(t *testing.T, f *localePushFixture) { f.failPushInsert(t, 0) }, false},
		{"publish/second_variant", func(t *testing.T, f *localePushFixture) { f.failPushInsert(t, f.secondVariantDevice()) }, false},
		{"publish/mail_after_push", func(t *testing.T, f *localePushFixture) { f.failEmailInsert(t) }, false},
		{"remind/first_variant", func(t *testing.T, f *localePushFixture) { f.failPushInsert(t, 0) }, true},
		{"remind/second_variant", func(t *testing.T, f *localePushFixture) { f.failPushInsert(t, f.secondVariantDevice()) }, true},
	}
	for _, point := range points {
		t.Run(point.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			f := newLocalePushFixture(t, seriesLocales, 2)
			var before stored
			if point.remind {
				rec, _ := f.post(t, "/publish")
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				before = f.stored(t)
			}
			point.inject(t, f)
			rec, _ := f.post(t, map[bool]string{false: "/publish", true: "/remind"}[point.remind])
			assertErrorEnvelope(t, rec, http.StatusInternalServerError, "general.server")
			require.Equal(t, before, f.stored(t), "a failed series leaves the state before the request")
		})
	}
}

// TestLocalePushSeriesGateSuppressesEveryVariant keeps the tenant gates per
// variant: a closed dispatch switch, a closed window or an unbooked Eltern-App
// stops every push, while the mails still leave and the action succeeds.
func TestLocalePushSeriesGateSuppressesEveryVariant(t *testing.T) {
	t.Parallel()
	gates := []struct {
		name  string
		key   string
		value any
	}{
		{"dispatch_disabled", "notifications.dispatch_enabled", false},
		{"outside_window", "notifications.active_window_end", ""},
		{"module_unbooked", "module.eltern_app", false},
	}
	for _, gate := range gates {
		t.Run(gate.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			f := newLocalePushFixture(t, seriesLocales, 2)
			if gate.name == "outside_window" {
				// A one-hour window starting two hours from now is closed now.
				start := (calendar.Now().Hour() + 2) % 24
				require.NoError(t, f.settings.SetValue(f.ctx, "notifications.active_window_start", fmt.Sprintf("%02d:00", start), nil, nil))
				gate.value = fmt.Sprintf("%02d:00", (start+1)%24)
			}
			require.NoError(t, f.settings.SetValue(f.ctx, gate.key, gate.value, nil, nil))
			rec, _ := f.post(t, "/publish")
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			after := f.stored(t)
			require.Empty(t, after.push)
			require.True(t, after.published)
			require.Equal(t, len(f.localeOf), after.emails)
			rec, _ = f.post(t, "/remind")
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Contains(t, rec.Body.String(), fmt.Sprintf(`"reminded_count":%d`, len(f.localeOf)))
			after = f.stored(t)
			require.Empty(t, after.push)
			require.Equal(t, 2*len(f.localeOf), after.emails)
		})
	}
}

// sendDueReminders runs the scheduler's reminder tick for the fixture's
// tenant in a tenant transaction of its own, as the scheduler does.
func (f *localePushFixture) sendDueReminders(t *testing.T) (int, []string, error) {
	t.Helper()
	var sent int
	f.counter.Reset()
	f.counter.Start()
	err := testpkg.WithinTenantContext(t, f.counter.Context(f.ctx), f.db, testpkg.Tenant(t), func(c context.Context) error {
		var e error
		sent, _, e = f.service.SendDueParentAnnouncementReminders(c, time.Now(), time.Now().Add(2*time.Hour))
		return e
	})
	f.counter.Stop()
	return sent, f.counter.Queries(), err
}

func (f *localePushFixture) scheduledReminderKey(t *testing.T) func(string) string {
	var row struct {
		ReminderAt  time.Time `bun:"reminder_at"`
		PublishedAt time.Time `bun:"published_at"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT reminder_at, published_at FROM users.parent_announcements WHERE id = ?`, f.pollID).Scan(f.ctx, &row))
	return func(locale string) string {
		return fmt.Sprintf("parent-announcement-reminder:%d:%d:%d:%s", f.pollID, row.ReminderAt.UTC().UnixNano(), row.PublishedAt.UTC().UnixNano(), locale)
	}
}

// TestLocalePushSeriesScheduledReminder covers the durable receipt path of
// the scheduled reminder: its claim, the language of every guardian, one
// device read, and a failure at the second variant that leaves the reminder
// unclaimed and unsent.
func TestLocalePushSeriesScheduledReminder(t *testing.T) {
	t.Parallel()
	t.Run("delivered", func(t *testing.T) {
		testpkg.OwnTenant(t)
		f := newLocalePushFixtureFor(t, seriesLocales, 2, true)
		rec, _ := f.post(t, "/publish")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		before := f.stored(t)
		sent, queries, err := f.sendDueReminders(t)
		require.NoError(t, err)
		require.Equal(t, 1, sent)
		after := f.stored(t)
		require.True(t, after.reminderSent)
		reminded := f.expectedPush(f.scheduledReminderKey(t), communication.ParentAnnouncementReminder, fmt.Sprintf("parent_announcement_reminder:%d", f.pollID))
		want := append(append([]pushRow{}, before.push...), reminded...)
		sort.Slice(want, func(i, j int) bool { return want[i].Key < want[j].Key })
		require.Equal(t, want, after.push)
		require.Equal(t, 2*len(f.localeOf), after.emails)
		t.Logf("languages=%d scheduled reminder statements=%d device reads=%d", len(seriesLocales), len(queries), subscriptionReads(queries))
		require.Equal(t, 1, subscriptionReads(queries), "the scheduled reminder reads the devices of all languages once")
	})
	t.Run("second_variant_fails", func(t *testing.T) {
		testpkg.OwnTenant(t)
		f := newLocalePushFixtureFor(t, seriesLocales, 2, true)
		rec, _ := f.post(t, "/publish")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		before := f.stored(t)
		f.failPushInsert(t, f.secondVariantDevice())
		sent, _, err := f.sendDueReminders(t)
		require.ErrorContains(t, err, "injected push failure")
		require.Zero(t, sent)
		require.Equal(t, before, f.stored(t), "the claim and every variant roll back together")
		require.False(t, f.stored(t).reminderSent)
	})
}
