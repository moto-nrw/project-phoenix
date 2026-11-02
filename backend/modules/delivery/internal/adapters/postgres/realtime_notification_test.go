package postgres_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"regexp"
	"slices"
	"testing"
	"time"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/realtime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Notifications cross processes as an ID (#5504). The receiving API loads
// title, body and link itself, inside the school's transaction, and shows
// them only to the recipients the stored notification names.

const (
	relayedTitle = "Neue Nachricht im Team-Chat"
	relayedBody  = "Bitte öffnen Sie den Chat."
	relayedLink  = "/staff-messages"
)

type everyGuardianStillPermitted struct{}

func (everyGuardianStillPermitted) FilterAccountsWithStudentAccess(_ context.Context, accounts, _ []int64, _ int64, _ string) ([]int64, error) {
	return accounts, nil
}

type notificationDeliverer interface {
	Deliver(context.Context, deliveryCompose.NotificationEvent) error
}

// notificationChannel builds the SSE channel every process composes over its
// hub; over a relaying hub it stores each notification for the other APIs.
func notificationChannel(t *testing.T, db *bun.DB, hub *realtime.Hub) notificationDeliverer {
	t.Helper()
	channel, err := deliveryCompose.NewTenantSSEChannel(testpkg.TenantRuntime(t, db), hub, db, everyGuardianStillPermitted{}, slog.Default(),
		postgres.New(postgres.RequireTenantDatabase, postgres.RequireAdminDatabase))
	require.NoError(t, err)
	return channel
}

func staffNotification(schoolID int64, accountIDs ...int64) deliveryCompose.NotificationEvent {
	return deliveryCompose.NotificationEvent{
		Type:     "test",
		Audience: deliveryCompose.NotificationAudience{TenantID: schoolID, Scope: deliveryCompose.NotificationAudienceScope("staff"), StaffAccountIDs: accountIDs},
		Priority: "normal", Title: relayedTitle, Body: relayedBody, DeepLink: relayedLink,
		Data: map[string]string{"thread_id": "7"},
	}
}

func guardianNotification(schoolID, guardianAccountID int64) deliveryCompose.NotificationEvent {
	event := staffNotification(schoolID)
	event.Audience = deliveryCompose.NotificationAudience{TenantID: schoolID, Scope: deliveryCompose.NotificationAudienceScope("guardian"), GuardianAccountIDs: []int64{guardianAccountID}}
	return event
}

func staffClient(t *testing.T, hub *realtime.Hub, schoolID, accountID int64) *realtime.Client {
	t.Helper()
	client := &realtime.Client{Channel: make(chan realtime.Event, 16), AccountID: accountID, SubscribedGroups: make(map[string]bool)}
	hub.Register(client, schoolID, nil)
	t.Cleanup(func() { hub.Unregister(client) })
	return client
}

func parentClient(t *testing.T, hub *realtime.Hub, guardianAccountID int64) *realtime.Client {
	t.Helper()
	client := &realtime.Client{Channel: make(chan realtime.Event, 16), UserID: guardianAccountID, AccountID: guardianAccountID}
	hub.RegisterParent(client)
	t.Cleanup(func() { hub.Unregister(client) })
	return client
}

func requireRelayedNotification(t *testing.T, client *realtime.Client) {
	t.Helper()
	event := receiveTransportEvent(t, client)
	require.Equal(t, realtime.EventNotification, event.Type)
	require.NotNil(t, event.Data.Title)
	require.Equal(t, relayedTitle, *event.Data.Title)
	require.NotNil(t, event.Data.Body)
	require.Equal(t, relayedBody, *event.Data.Body)
	require.NotNil(t, event.Data.DeepLink)
	require.Equal(t, relayedLink, *event.Data.DeepLink)
	require.Equal(t, map[string]string{"thread_id": "7"}, event.Data.NotificationData)
}

func TestWorkerNotificationReachesEveryAPI(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	worker := transportHub(t, db, false)
	apiA, apiB := transportHub(t, db, true), transportHub(t, db, true)
	workerChannel := notificationChannel(t, db, worker)
	schoolID := testpkg.Tenant(t)
	_, addressed := testpkg.CreateTestCalendarStaff(t, db, "Relay", "Addressed")
	_, bystander := testpkg.CreateTestCalendarStaff(t, db, "Relay", "Bystander")
	guardian := testpkg.CreateTestParentGuardianChain(t, db)
	a, b := staffClient(t, apiA, schoolID, addressed.ID), staffClient(t, apiB, schoolID, addressed.ID)
	other := staffClient(t, apiA, schoolID, bystander.ID)
	parent := parentClient(t, apiB, guardian.AccountID)

	require.NoError(t, workerChannel.Deliver(context.Background(), staffNotification(schoolID, addressed.ID)))
	requireRelayedNotification(t, a)
	requireRelayedNotification(t, b)
	require.Empty(t, other.Channel, "a colleague who was not addressed saw the notification")
	require.Empty(t, parent.Channel, "a guardian saw a staff notification")

	require.NoError(t, workerChannel.Deliver(context.Background(), guardianNotification(schoolID, guardian.AccountID)))
	requireRelayedNotification(t, parent)
	require.Empty(t, a.Channel, "staff saw a guardian notification")
	require.Empty(t, other.Channel)
}

func TestAPINotificationIsDeliveredOnceLocallyAndRelayedToOtherAPIs(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	apiA, apiB := transportHub(t, db, true), transportHub(t, db, true)
	channelA := notificationChannel(t, db, apiA)
	schoolID := testpkg.Tenant(t)
	_, addressed := testpkg.CreateTestCalendarStaff(t, db, "Relay", "Once")
	a, b := staffClient(t, apiA, schoolID, addressed.ID), staffClient(t, apiB, schoolID, addressed.ID)

	require.NoError(t, channelA.Deliver(context.Background(), staffNotification(schoolID, addressed.ID)))
	requireRelayedNotification(t, a)
	requireRelayedNotification(t, b)
	// The marker travels behind the notification through the same FIFO
	// publisher and listener: an echo of the own signal would arrive first.
	require.NoError(t, apiA.BroadcastToStaffAccounts(schoolID, []int64{addressed.ID}, realtime.NewEvent(realtime.EventStaffTimeTrackingChanged, "", realtime.EventData{})))
	require.Equal(t, realtime.EventStaffTimeTrackingChanged, receiveTransportEvent(t, b).Type)
	require.Equal(t, realtime.EventStaffTimeTrackingChanged, receiveTransportEvent(t, a).Type, "the producing API showed its own notification twice")
	require.Empty(t, a.Channel)
}

func TestNotificationSignalCarriesNoContent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var payloads []string
	producer, err := realtime.NewRelayedHub(slog.Default(), func(payload string) error { payloads = append(payloads, payload); return nil }, nil)
	require.NoError(t, err)
	schoolID := testpkg.Tenant(t)
	addressed := testpkg.CreateTestAccount(t, db, "relay-notification-wire@example.com")

	require.NoError(t, notificationChannel(t, db, producer).Deliver(context.Background(), staffNotification(schoolID, addressed.ID)))
	require.Len(t, payloads, 1)
	for _, content := range []string{relayedTitle, relayedBody, relayedLink, "thread_id", "Bitte", "normal"} {
		require.NotContains(t, payloads[0], content)
	}
	var wire map[string]any
	require.NoError(t, json.Unmarshal([]byte(payloads[0]), &wire))
	require.Equal(t, []string{"account_ids", "audience", "ids", "origin", "school_id", "type"}, slices.Sorted(maps.Keys(wire)))
	require.Equal(t, "notification", wire["type"])
	require.Equal(t, "staff_accounts", wire["audience"])
	ids, ok := wire["ids"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, []string{"notification_id"}, slices.Sorted(maps.Keys(ids)))
	require.Regexp(t, regexp.MustCompile(`^[1-9][0-9]*$`), ids["notification_id"])
}

func TestForgedNotificationSignalsShowNothing(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var payloads []string
	producer, err := realtime.NewRelayedHub(slog.Default(), func(payload string) error { payloads = append(payloads, payload); return nil }, nil)
	require.NoError(t, err)
	api := transportHub(t, db, false)
	schoolID := testpkg.Tenant(t)
	foreignSchool := testpkg.NewTenantScope(t, db).TenantID
	_, addressed := testpkg.CreateTestCalendarStaff(t, db, "Relay", "Forged")
	bystander := testpkg.CreateTestAccount(t, db, "relay-forged-bystander@example.com")
	clients := []*realtime.Client{
		staffClient(t, api, schoolID, addressed.ID),
		staffClient(t, api, schoolID, bystander.ID),
		staffClient(t, api, foreignSchool, addressed.ID),
		staffClient(t, api, foreignSchool, bystander.ID),
		parentClient(t, api, addressed.ID),
		parentClient(t, api, bystander.ID),
	}
	// The foreign school has a genuine notification of its own, so its ID is
	// real, only not for this school.
	require.NoError(t, notificationChannel(t, db, producer).Deliver(context.Background(), staffNotification(foreignSchool, addressed.ID)))
	require.NoError(t, notificationChannel(t, db, producer).Deliver(context.Background(), staffNotification(schoolID, addressed.ID)))
	require.Len(t, payloads, 2)
	foreignID := signalNotificationID(t, payloads[0])
	genuine := decodeSignal(t, payloads[1])

	forged := map[string]map[string]any{
		"unknown ID":                      withSignal(genuine, map[string]any{"ids": map[string]any{"notification_id": "9223372036854775806"}}),
		"ID of another school":            withSignal(genuine, map[string]any{"ids": map[string]any{"notification_id": foreignID}}),
		"another school":                  withSignal(genuine, map[string]any{"school_id": foreignSchool}),
		"another recipient":               withSignal(genuine, map[string]any{"account_ids": []int64{bystander.ID}}),
		"another portal":                  withSignal(genuine, map[string]any{"audience": "school_accounts"}),
		"the whole school":                withSignal(genuine, map[string]any{"audience": "tenant", "account_ids": nil}),
		"the school's administrators":     withSignal(genuine, map[string]any{"audience": "admins", "account_ids": nil}),
		"a guardian":                      withSignal(genuine, map[string]any{"audience": "guardian", "guardian_id": addressed.ID, "account_ids": nil}),
		"content without an ID":           withSignal(genuine, map[string]any{"ids": map[string]any{"title": "Gefälscht", "deep_link": "/forged"}}),
		"invalid ID":                      withSignal(genuine, map[string]any{"ids": map[string]any{"notification_id": "-1"}}),
		"group audience with the same ID": withSignal(genuine, map[string]any{"audience": "group", "topic": "1", "account_ids": nil}),
	}
	for name, signal := range forged {
		payload, err := json.Marshal(signal)
		require.NoError(t, err)
		_ = api.ReceiveSignal(string(payload))
		for _, client := range clients {
			require.Empty(t, client.Channel, "forged signal (%s) reached a client", name)
		}
	}

	require.NoError(t, api.ReceiveSignal(payloads[1]))
	requireRelayedNotification(t, clients[0])
	for _, client := range clients[1:] {
		require.Empty(t, client.Channel, "the genuine signal reached a client it does not address")
	}

	// A recorded signal replayed after the notification expired shows nothing.
	_, err = db.ExecContext(context.Background(), `UPDATE platform.realtime_notifications SET created_at = NOW() - INTERVAL '1 hour' WHERE tenant_id = ?`, schoolID)
	require.NoError(t, err)
	_ = api.ReceiveSignal(payloads[1])
	for _, client := range clients {
		require.Empty(t, client.Channel, "an expired notification was shown again")
	}
}

func TestNotificationStaysLocalWhenItCannotBeStored(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	var published []string
	hub, err := realtime.NewRelayedHub(slog.Default(), func(payload string) error { published = append(published, payload); return nil }, nil)
	require.NoError(t, err)
	channel := notificationChannel(t, db, hub)
	schoolID := testpkg.Tenant(t)
	addressed := testpkg.CreateTestAccount(t, db, "relay-notification-offline@example.com")
	client := staffClient(t, hub, schoolID, addressed.ID)
	require.NoError(t, db.Close())

	require.NoError(t, channel.Deliver(context.Background(), staffNotification(schoolID, addressed.ID)))
	requireRelayedNotification(t, client)
	require.Empty(t, published, "an unstored notification has no ID another API could load")
}

func decodeSignal(t *testing.T, payload string) map[string]any {
	t.Helper()
	var signal map[string]any
	require.NoError(t, json.Unmarshal([]byte(payload), &signal))
	return signal
}

func signalNotificationID(t *testing.T, payload string) string {
	t.Helper()
	ids, ok := decodeSignal(t, payload)["ids"].(map[string]any)
	require.True(t, ok)
	id, ok := ids["notification_id"].(string)
	require.True(t, ok)
	return id
}

func withSignal(signal, changes map[string]any) map[string]any {
	result := maps.Clone(signal)
	for key, value := range changes {
		if value == nil {
			delete(result, key)
			continue
		}
		result[key] = value
	}
	return result
}

func TestRelayedGuardianNotificationRechecksChildAccess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var payload string
	producer, err := realtime.NewRelayedHub(slog.Default(), func(p string) error { payload = p; return nil }, nil)
	require.NoError(t, err)
	api := transportHub(t, db, false)
	schoolID := testpkg.Tenant(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	event := guardianNotification(schoolID, chain.AccountID)
	event.Audience.StudentIDs = []int64{chain.StudentID}
	require.NoError(t, notificationChannel(t, db, producer).Deliver(context.Background(), event))
	require.NotContains(t, payload, "relay_student_ids", "child scope must stay out of the unauthenticated signal")
	parent := parentClient(t, api, chain.AccountID)

	require.NoError(t, api.ReceiveSignal(payload))
	requireRelayedNotification(t, parent)
	_, err = db.ExecContext(context.Background(), `UPDATE auth.guardian_student_access SET permissions = permissions - 'parent_portal.access'
		WHERE relationship_id IN (SELECT id FROM users.student_guardian_relationships WHERE guardian_profile_id = ?)`, chain.GuardianProfileID)
	require.NoError(t, err)
	require.NoError(t, api.ReceiveSignal(payload))
	require.Empty(t, parent.Channel, "revoked access must suppress a replayed notification")

	// A receiving API whose lookup fails shows nothing.
	closable := testpkg.SetupClosableTestDB(t)
	offline := transportHub(t, closable, false)
	offlineParent := parentClient(t, offline, chain.AccountID)
	require.NoError(t, closable.Close())
	require.Error(t, offline.ReceiveSignal(payload))
	require.Empty(t, offlineParent.Channel, "a failed permission lookup must fail closed")

	// Legacy rows lack the child scope needed to authorize parent delivery.
	_, err = db.ExecContext(context.Background(), `UPDATE auth.guardian_student_access SET permissions = permissions || '{"parent_portal.access": true}'::jsonb
		WHERE relationship_id IN (SELECT id FROM users.student_guardian_relationships WHERE guardian_profile_id = ?)`, chain.GuardianProfileID)
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(), `UPDATE platform.realtime_notifications SET content = content - 'relay_student_ids' WHERE tenant_id = ?`, schoolID)
	require.NoError(t, err)
	require.NoError(t, api.ReceiveSignal(payload))
	require.Empty(t, parent.Channel, "legacy guardian content cannot be authorized")
}

func TestNotificationStaysLocalWhenRelayPoolIsBusy(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	var published []string
	hub, err := realtime.NewRelayedHub(slog.Default(), func(p string) error { published = append(published, p); return nil }, nil)
	require.NoError(t, err)
	channel := notificationChannel(t, db, hub)
	schoolID := testpkg.Tenant(t)
	account := testpkg.CreateTestAccount(t, db, "relay-busy-pool@example.com")
	client := staffClient(t, hub, schoolID, account.ID)
	db.SetMaxOpenConns(1)
	conn, err := db.DB.Conn(context.Background())
	require.NoError(t, err)
	defer func() { require.NoError(t, conn.Close()) }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	start := time.Now()
	require.NoError(t, channel.Deliver(ctx, staffNotification(schoolID, account.ID)))
	require.Less(t, time.Since(start), time.Second, "optional relay storage must not wait for the caller's deadline")
	requireRelayedNotification(t, client)
	require.Empty(t, published)
}

func schoolClient(t *testing.T, hub *realtime.Hub, schoolID, accountID int64) *realtime.Client {
	t.Helper()
	client := &realtime.Client{Channel: make(chan realtime.Event, 16), AccountID: accountID}
	hub.RegisterSchool(client, schoolID)
	t.Cleanup(func() { hub.Unregister(client) })
	return client
}

// A relayed notification is shown only while its staff recipient still
// belongs to the school in the addressed portal, the same delivery-time rule
// Web Push applies (#5504). The producer resolved the recipients earlier; a
// delayed or replayed signal must not outlive a revoked membership or role.
func TestRelayedStaffNotificationRechecksMembershipAndRole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var payloads []string
	producer, err := realtime.NewRelayedHub(slog.Default(), func(payload string) error { payloads = append(payloads, payload); return nil }, nil)
	require.NoError(t, err)
	api := transportHub(t, db, false)
	school := testpkg.Tenant(t)
	_, staff := testpkg.CreateTestCalendarStaff(t, db, "Relay", "Staff")
	_, teacher := testpkg.CreateTestCalendarStaff(t, db, "Relay", "Lehrkraft")
	testpkg.AssignLehrkraftSystemRole(t, db, teacher.ID, school)
	staffTab := staffClient(t, api, school, staff.ID)
	teacherTab := schoolClient(t, api, school, teacher.ID)

	sender := notificationChannel(t, db, producer)
	require.NoError(t, sender.Deliver(context.Background(), staffNotification(school, staff.ID)))
	schoolNotice := staffNotification(school, teacher.ID)
	schoolNotice.Portal = "school"
	require.NoError(t, sender.Deliver(context.Background(), schoolNotice))
	require.Len(t, payloads, 2)

	// While both still belong to the school, the relayed copies arrive.
	require.NoError(t, api.ReceiveSignal(payloads[0]))
	requireRelayedNotification(t, staffTab)
	require.NoError(t, api.ReceiveSignal(payloads[1]))
	requireRelayedNotification(t, teacherTab)

	_, err = db.ExecContext(context.Background(), `UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?`, staff.ID, school)
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(), `DELETE FROM auth.account_roles WHERE account_id = ? AND tenant_id = ?
		AND role_id = (SELECT id FROM auth.roles WHERE name = 'lehrkraft' AND is_system)`, teacher.ID, school)
	require.NoError(t, err)

	_ = api.ReceiveSignal(payloads[0])
	_ = api.ReceiveSignal(payloads[1])
	require.Empty(t, staffTab.Channel, "a staff member whose school membership ended still saw the notification")
	require.Empty(t, teacherTab.Channel, "an account without the Lehrkraft role still saw a school-portal notification")
}

// A school's own role marked for moto schule counts like the Lehrkraft role,
// exactly as for Web Push (#4574, #5504); a custom role without the mark
// does not open the school portal.
func TestRelayedSchoolNotificationAcceptsCustomSchoolPortalRoles(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var payloads []string
	producer, err := realtime.NewRelayedHub(slog.Default(), func(payload string) error { payloads = append(payloads, payload); return nil }, nil)
	require.NoError(t, err)
	api := transportHub(t, db, false)
	school := testpkg.Tenant(t)
	assignRole := func(accountID int64, schoolPortal bool) {
		role := testpkg.CreateTestRole(t, db, "relay-school-role")
		_, err := db.ExecContext(context.Background(), `UPDATE auth.roles SET school_portal = ? WHERE id = ?`, schoolPortal, role.ID)
		require.NoError(t, err)
		_, err = db.ExecContext(context.Background(), `INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)`, accountID, role.ID, school)
		require.NoError(t, err)
	}
	_, marked := testpkg.CreateTestCalendarStaff(t, db, "Relay", "Marked")
	_, unmarked := testpkg.CreateTestCalendarStaff(t, db, "Relay", "Unmarked")
	assignRole(marked.ID, true)
	assignRole(unmarked.ID, false)
	markedTab := schoolClient(t, api, school, marked.ID)
	unmarkedTab := schoolClient(t, api, school, unmarked.ID)

	notice := staffNotification(school, marked.ID, unmarked.ID)
	notice.Portal = "school"
	require.NoError(t, notificationChannel(t, db, producer).Deliver(context.Background(), notice))
	require.Len(t, payloads, 1)
	require.NoError(t, api.ReceiveSignal(payloads[0]))
	requireRelayedNotification(t, markedTab)
	require.Empty(t, unmarkedTab.Channel, "a custom role without the school-portal mark opened moto schule")
}
