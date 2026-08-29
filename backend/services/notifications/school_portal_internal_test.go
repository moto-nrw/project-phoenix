package notifications

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The school portal (#2208) borrows from the staff catalogue: only types that
// opt in with SchoolPortal are offered, and a decision made there lands on
// the same (account, type) row.
func TestSchoolPortalCatalogue(t *testing.T) {
	t.Parallel()

	keys := make([]string, 0)
	for _, def := range TypesForPortal(PortalSchool) {
		keys = append(keys, def.Key)
	}
	assert.Equal(t, []string{TypeStaffMessage}, keys)

	staffMessage, _ := GetType(TypeStaffMessage)
	pickup, _ := GetType(TypePickupUpcoming)
	assert.True(t, OfferedInPortal(staffMessage, PortalSchool))
	assert.True(t, OfferedInPortal(staffMessage, PortalStaff), "still offered in the OGS portal")
	assert.False(t, OfferedInPortal(pickup, PortalSchool), "supervision reminders never address a Lehrkraft")
}

// A school registration rebinds the browser endpoint: the school a Lehrkraft
// left keeps neither its row nor its pushes, and both steps share one
// transaction so a failure never leaves the device unregistered.
func TestSubscribeSchoolRebindsTheEndpoint(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() { _ = sqlDB.Close() })

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	repo := &recordingPushRepository{}
	service := newMockPushSubscriptionService(t, db, repo, nil, testVAPID(), nil)
	ctx := tenant.WithTenantID(context.Background(), 41)

	require.NoError(t, service.SubscribeSchool(ctx, 42, validPushInput()))
	require.Len(t, repo.upserted, 1)
	assert.Equal(t, iot.PushPortalSchool, repo.upserted[0].Portal)
	assert.Equal(t, int64(41), repo.upserted[0].TenantID)
	assert.Equal(t, validPushInput().Endpoint, repo.reboundEndpoint)
	assert.Equal(t, []string{"clear-school", "upsert:41"}, repo.operations)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSubscribeSchoolReportsRebindFailures(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() { _ = sqlDB.Close() })

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	repo := &recordingPushRepository{deleteSchoolErr: errPushRepository}
	service := newMockPushSubscriptionService(t, db, repo, nil, testVAPID(), nil)
	ctx := tenant.WithTenantID(context.Background(), 41)

	err = service.SubscribeSchool(ctx, 42, validPushInput())
	require.ErrorIs(t, err, errPushRepository)
	assert.ErrorContains(t, err, "clearing previous school push subscription bindings")
	assert.Empty(t, repo.upserted)
	require.NoError(t, mock.ExpectationsWereMet())
}

// Without a school on the context there is nothing to bind the device to;
// registering it anyway would write a row no school ever reads.
func TestSubscribeSchoolRequiresASchoolContext(t *testing.T) {
	t.Parallel()
	repo := &recordingPushRepository{}
	service := newMockPushSubscriptionService(t, nil, repo, nil, testVAPID(), nil)

	err := service.SubscribeSchool(context.Background(), 42, validPushInput())
	require.EqualError(t, err, "school push subscription requires a school context")
	assert.Empty(t, repo.upserted)
}

// A school device gets the school host's deep link; every other device the
// payload as marshalled. A notification without a school destination sends
// the device to the portal root instead of to a path that does not exist
// there.
func TestPortalPayloadsPickTheSchoolDeepLink(t *testing.T) {
	t.Parallel()

	event := Event{
		Type:           TypeStaffMessage,
		Title:          "Neue Nachricht aus dem Team",
		DeepLink:       "/team-chat/7",
		SchoolDeepLink: "/school/nachrichten/7",
	}
	base, err := marshalPushPayload(event)
	require.NoError(t, err)
	payloads := &portalPayloads{event: event, base: base}

	staffWire, err := payloads.forSubscription(&iot.PushSubscription{Portal: iot.PushPortalStaff})
	require.NoError(t, err)
	assert.Equal(t, "/team-chat/7", deepLinkOf(t, staffWire))

	schoolWire, err := payloads.forSubscription(&iot.PushSubscription{Portal: iot.PushPortalSchool})
	require.NoError(t, err)
	assert.Equal(t, "/school/nachrichten/7", deepLinkOf(t, schoolWire))

	noSchool := &portalPayloads{event: Event{Type: "test", Title: "t", DeepLink: "/dashboard"}, base: base}
	wire, err := noSchool.forSubscription(&iot.PushSubscription{Portal: iot.PushPortalSchool})
	require.NoError(t, err)
	assert.Equal(t, "/school", deepLinkOf(t, wire))
}

func TestSchoolPushDeliveryIsLimitedToSupportedTypes(t *testing.T) {
	t.Parallel()

	staffSub := testSub(1, 41, "https://fcm.googleapis.com/staff")
	staffSub.AccountID = 42
	schoolSub := testSub(2, 41, "https://fcm.googleapis.com/school")
	schoolSub.AccountID = 42
	schoolSub.Portal = iot.PushPortalSchool
	channel := testChannel(&fakePushRepo{
		staffAccounts:  map[int64][]*iot.PushSubscription{42: {staffSub}},
		schoolAccounts: map[int64][]*iot.PushSubscription{42: {schoolSub}},
	}, &fakeSender{})

	unsupported, err := channel.resolveEventSubscriptions(context.Background(), Event{
		Type:     TypePickupUpcoming,
		Audience: Audience{Scope: ScopeStaff, StaffAccountIDs: []int64{42}},
	})
	require.NoError(t, err)
	assert.Equal(t, []*iot.PushSubscription{staffSub}, unsupported)

	supported, err := channel.resolveEventSubscriptions(context.Background(), Event{
		Type:     TypeStaffMessage,
		Audience: Audience{Scope: ScopeStaff, StaffAccountIDs: []int64{42}},
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []*iot.PushSubscription{staffSub, schoolSub}, supported)
}

// The same split holds for the devices: a test fired in moto schule reaches
// the school registrations only, and one fired in the OGS portal the staff
// registrations only.
func TestTestPushStaysInTheRequestingPortal(t *testing.T) {
	t.Parallel()

	staffSub := testSub(1, 41, "https://fcm.googleapis.com/staff")
	staffSub.AccountID = 42
	schoolSub := testSub(2, 41, "https://fcm.googleapis.com/school")
	schoolSub.AccountID = 42
	schoolSub.Portal = iot.PushPortalSchool
	channel := testChannel(&fakePushRepo{
		staffAccounts:  map[int64][]*iot.PushSubscription{42: {staffSub}},
		schoolAccounts: map[int64][]*iot.PushSubscription{42: {schoolSub}},
	}, &fakeSender{})

	audience := Audience{Scope: ScopeStaff, StaffAccountIDs: []int64{42}}

	fromSchool, err := channel.resolveEventSubscriptions(context.Background(), Event{
		Type:     TypeTest,
		Portal:   PortalSchool,
		Audience: audience,
	})
	require.NoError(t, err)
	assert.Equal(t, []*iot.PushSubscription{schoolSub}, fromSchool)

	fromStaff, err := channel.resolveEventSubscriptions(context.Background(), Event{
		Type:     TypeTest,
		Audience: audience,
	})
	require.NoError(t, err)
	assert.Equal(t, []*iot.PushSubscription{staffSub}, fromStaff)
}

// One browser can be registered in both staff portals: the rows differ by
// portal, the endpoint does not. Sending both would put the same message on
// the same device twice, so only the current registration survives — the most
// recently written row — and its portal decides the deep link. Here the
// device last registered in moto schule, so it must not be sent the OGS link.
func TestSchoolPushDoesNotDuplicateASharedEndpoint(t *testing.T) {
	t.Parallel()

	const sharedEndpoint = "https://fcm.googleapis.com/shared"
	staffSub := testSub(1, 41, sharedEndpoint)
	staffSub.AccountID = 42
	staffSub.UpdatedAt = time.Now().Add(-time.Hour)
	schoolSub := testSub(2, 41, sharedEndpoint)
	schoolSub.AccountID = 42
	schoolSub.Portal = iot.PushPortalSchool
	schoolSub.UpdatedAt = time.Now()
	repo := &fakePushRepo{
		staffAccounts:  map[int64][]*iot.PushSubscription{42: {staffSub}},
		schoolAccounts: map[int64][]*iot.PushSubscription{42: {schoolSub}},
	}
	sender := &fakeSender{}
	channel := testChannel(repo, sender)

	resolved, err := channel.resolveEventSubscriptions(context.Background(), Event{
		Type:     TypeStaffMessage,
		Audience: Audience{Scope: ScopeStaff, StaffAccountIDs: []int64{42}},
	})
	require.NoError(t, err)
	assert.Equal(t, []*iot.PushSubscription{schoolSub}, resolved,
		"the shared endpoint is pushed to once, through its current registration")

	db, mock := mockTenantTx(t)
	channel.db = db
	channel.SetTenantRuntime(newMockTenantRuntime(t, db))
	require.NoError(t, channel.DeliverBatch(context.Background(), []Event{{
		Type:           TypeStaffMessage,
		Audience:       Audience{TenantID: 41, Scope: ScopeStaff, StaffAccountIDs: []int64{42}},
		Priority:       PriorityNormal,
		Title:          "Neue Nachricht aus dem Team",
		DeepLink:       "/team-chat/7",
		SchoolDeepLink: "/school/nachrichten/7",
	}}))
	require.Eventually(t, func() bool {
		sender.mu.Lock()
		defer sender.mu.Unlock()
		return len(sender.sent) == 1
	}, time.Second, 10*time.Millisecond)
	sender.mu.Lock()
	sent := append([]sentPush(nil), sender.sent...)
	sender.mu.Unlock()
	require.Len(t, sent, 1, "the batched path deduplicates the same endpoint too")
	assert.Equal(t, "/school/nachrichten/7", deepLinkOf(t, sent[0].payload),
		"the surviving school row carries the link that exists in moto schule")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// The reverse case: the device last registered in the OGS portal keeps the
// OGS deep link.
func TestSharedEndpointKeepsTheCurrentStaffRegistration(t *testing.T) {
	t.Parallel()

	const sharedEndpoint = "https://fcm.googleapis.com/shared"
	staffSub := testSub(1, 41, sharedEndpoint)
	staffSub.AccountID = 42
	staffSub.UpdatedAt = time.Now()
	schoolSub := testSub(2, 41, sharedEndpoint)
	schoolSub.AccountID = 42
	schoolSub.Portal = iot.PushPortalSchool
	schoolSub.UpdatedAt = time.Now().Add(-time.Hour)
	channel := testChannel(&fakePushRepo{
		staffAccounts:  map[int64][]*iot.PushSubscription{42: {staffSub}},
		schoolAccounts: map[int64][]*iot.PushSubscription{42: {schoolSub}},
	}, &fakeSender{})

	resolved, err := channel.resolveEventSubscriptions(context.Background(), Event{
		Type:     TypeStaffMessage,
		Audience: Audience{Scope: ScopeStaff, StaffAccountIDs: []int64{42}},
	})
	require.NoError(t, err)
	assert.Equal(t, []*iot.PushSubscription{staffSub}, resolved)
}

// Two portals on one browser are two origins, so the same device holds two
// different push endpoints. Nothing links the two rows to one machine, so both
// are delivered, each through its own portal: the OGS registration gets the
// tenant deep link, the school registration the moto-schule one. Collapsing
// them by User-Agent would drop a second, genuinely separate device that
// happens to report the same string.
func TestBothPortalRegistrationsOfOneAccountAreDelivered(t *testing.T) {
	t.Parallel()

	const browser = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0) Safari/605.1.15"
	staffSub := testSub(1, 41, "https://fcm.googleapis.com/ogs-origin")
	staffSub.AccountID = 42
	staffSub.UserAgent = browser
	staffSub.UpdatedAt = time.Now().Add(-time.Hour)
	schoolSub := testSub(2, 41, "https://fcm.googleapis.com/school-origin")
	schoolSub.AccountID = 42
	schoolSub.Portal = iot.PushPortalSchool
	schoolSub.UserAgent = browser
	schoolSub.UpdatedAt = time.Now()

	sender := &fakeSender{}
	channel := testChannel(&fakePushRepo{
		staffAccounts:  map[int64][]*iot.PushSubscription{42: {staffSub}},
		schoolAccounts: map[int64][]*iot.PushSubscription{42: {schoolSub}},
	}, sender)

	resolved, err := channel.resolveEventSubscriptions(context.Background(), Event{
		Type:     TypeStaffMessage,
		Audience: Audience{Scope: ScopeStaff, StaffAccountIDs: []int64{42}},
	})
	require.NoError(t, err)
	assert.Equal(t, []*iot.PushSubscription{staffSub, schoolSub}, resolved,
		"a shared User-Agent is not a device identity, so neither row is dropped")

	db, mock := mockTenantTx(t)
	channel.db = db
	channel.SetTenantRuntime(newMockTenantRuntime(t, db))
	require.NoError(t, channel.DeliverBatch(context.Background(), []Event{{
		Type:           TypeStaffMessage,
		Audience:       Audience{TenantID: 41, Scope: ScopeStaff, StaffAccountIDs: []int64{42}},
		Priority:       PriorityNormal,
		Title:          "Neue Nachricht aus dem Team",
		DeepLink:       "/team-chat/7",
		SchoolDeepLink: "/school/nachrichten/7",
	}}))
	require.Eventually(t, func() bool {
		sender.mu.Lock()
		defer sender.mu.Unlock()
		return len(sender.sent) == 2
	}, time.Second, 10*time.Millisecond)
	sender.mu.Lock()
	sent := append([]sentPush(nil), sender.sent...)
	sender.mu.Unlock()
	require.Len(t, sent, 2)
	links := []string{deepLinkOf(t, sent[0].payload), deepLinkOf(t, sent[1].payload)}
	assert.ElementsMatch(t, []string{"/team-chat/7", "/school/nachrichten/7"}, links,
		"every registration keeps the deep link that resolves in its own portal")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func deepLinkOf(t *testing.T, wire []byte) string {
	t.Helper()
	var payload map[string]any
	require.NoError(t, json.Unmarshal(wire, &payload))
	link, _ := payload["deepLink"].(string)
	return link
}
