package notifications

import (
	"database/sql"
	"testing"

	deliveryPorts "github.com/moto-nrw/project-phoenix/modules/delivery/internal/ports"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

// The construction tests never call a collaborator, so each stub embeds its
// interface unset.
type (
	stubBroadcaster       struct{ realtime.Broadcaster }
	stubChildAccess       struct{ GuardianChildAccessRepository }
	stubPushSubscriptions struct {
		deliveryPorts.PushSubscriptionRepository
	}
	stubPushOutbox     struct{ PushOutbox }
	stubService        struct{ Service }
	stubRecipients     struct{ StaffRecipientResolver }
	stubConsent        struct{ ConsentStore }
	stubPrefSettings   struct{ PreferenceSettings }
	stubGuardianSchool struct{ GuardianSchools }
)

func lazyTestDatabase() *bun.DB {
	// The connector opens no connection until a query runs.
	return bun.NewDB(sql.OpenDB(pgdriver.NewConnector()), pgdialect.New())
}

func TestTenantConstructorsBindTheRuntimeAtConstruction(t *testing.T) {
	t.Parallel()
	runtime, db := newMockTenantRuntime(t, nil), lazyTestDatabase()
	permits := PermissionMatch(func(string, []string) bool { return true })

	sse, err := NewTenantSSEChannel(runtime, stubBroadcaster{}, db, stubChildAccess{}, nil)
	require.NoError(t, err)
	assert.NotNil(t, sse.(*sseChannel).tenantRuntime)
	assert.NotNil(t, sse.(*sseChannel).guardians, "the guardian child-access check is wired")

	push, err := NewTenantDurableWebPushChannel(runtime, db, stubPushSubscriptions{}, VAPIDConfig{}, stubPushOutbox{}, nil)
	require.NoError(t, err)
	assert.NotNil(t, push.(*webPushChannel).tenantRuntime)
	assert.NotNil(t, push.(*webPushChannel).outbox)

	absence, err := NewTenantAbsenceNotifier(runtime, stubService{}, stubRecipients{}, db, nil)
	require.NoError(t, err)
	assert.NotNil(t, absence.(*absenceNotifier).tenantRuntime)

	preferences, err := NewTenantPreferenceService(runtime, stubConsent{}, stubPrefSettings{}, db, stubGuardianSchool{}, permits)
	require.NoError(t, err)
	assert.NotNil(t, preferences.(*preferenceService).tenantRuntime)

	subscriptions, err := NewTenantPushSubscriptionService(runtime, db, stubPushSubscriptions{}, stubGuardianSchool{}, VAPIDConfig{}, nil)
	require.NoError(t, err)
	assert.NotNil(t, subscriptions.(*pushSubscriptionService).tenantRuntime)
}

func TestTenantConstructorsRejectAMissingDependency(t *testing.T) {
	t.Parallel()
	runtime, db := newMockTenantRuntime(t, nil), lazyTestDatabase()
	permits := PermissionMatch(func(string, []string) bool { return true })

	for name, construct := range map[string]func(tenant.UnitOfWork) error{
		"sse": func(runtime tenant.UnitOfWork) error {
			_, err := NewTenantSSEChannel(runtime, stubBroadcaster{}, db, stubChildAccess{}, nil)
			return err
		},
		"web push": func(runtime tenant.UnitOfWork) error {
			_, err := NewTenantDurableWebPushChannel(runtime, db, stubPushSubscriptions{}, VAPIDConfig{}, stubPushOutbox{}, nil)
			return err
		},
		"absence": func(runtime tenant.UnitOfWork) error {
			_, err := NewTenantAbsenceNotifier(runtime, stubService{}, stubRecipients{}, db, nil)
			return err
		},
		"preferences": func(runtime tenant.UnitOfWork) error {
			_, err := NewTenantPreferenceService(runtime, stubConsent{}, stubPrefSettings{}, db, stubGuardianSchool{}, permits)
			return err
		},
		"push subscriptions": func(runtime tenant.UnitOfWork) error {
			_, err := NewTenantPushSubscriptionService(runtime, db, stubPushSubscriptions{}, stubGuardianSchool{}, VAPIDConfig{}, nil)
			return err
		},
	} {
		require.ErrorIs(t, construct(tenant.UnitOfWork{}), tenant.ErrRuntimeRequired, name)
		require.NoError(t, construct(runtime), name)
	}

	missing := map[string]error{}
	_, missing["sse broadcaster"] = NewTenantSSEChannel(runtime, nil, db, stubChildAccess{}, nil)
	_, missing["sse database"] = NewTenantSSEChannel(runtime, stubBroadcaster{}, nil, stubChildAccess{}, nil)
	_, missing["sse child access"] = NewTenantSSEChannel(runtime, stubBroadcaster{}, db, nil, nil)
	_, missing["web push database"] = NewTenantDurableWebPushChannel(runtime, nil, stubPushSubscriptions{}, VAPIDConfig{}, stubPushOutbox{}, nil)
	_, missing["web push repository"] = NewTenantDurableWebPushChannel(runtime, db, nil, VAPIDConfig{}, stubPushOutbox{}, nil)
	_, missing["web push outbox"] = NewTenantDurableWebPushChannel(runtime, db, stubPushSubscriptions{}, VAPIDConfig{}, nil, nil)
	_, missing["absence notifier"] = NewTenantAbsenceNotifier(runtime, nil, stubRecipients{}, db, nil)
	_, missing["absence recipients"] = NewTenantAbsenceNotifier(runtime, stubService{}, nil, db, nil)
	_, missing["absence database"] = NewTenantAbsenceNotifier(runtime, stubService{}, stubRecipients{}, nil, nil)
	_, missing["preferences consent"] = NewTenantPreferenceService(runtime, nil, stubPrefSettings{}, db, stubGuardianSchool{}, permits)
	_, missing["preferences settings"] = NewTenantPreferenceService(runtime, stubConsent{}, nil, db, stubGuardianSchool{}, permits)
	_, missing["preferences database"] = NewTenantPreferenceService(runtime, stubConsent{}, stubPrefSettings{}, nil, stubGuardianSchool{}, permits)
	_, missing["preferences schools"] = NewTenantPreferenceService(runtime, stubConsent{}, stubPrefSettings{}, db, nil, permits)
	_, missing["preferences permissions"] = NewTenantPreferenceService(runtime, stubConsent{}, stubPrefSettings{}, db, stubGuardianSchool{}, nil)
	_, missing["push subscriptions database"] = NewTenantPushSubscriptionService(runtime, nil, stubPushSubscriptions{}, stubGuardianSchool{}, VAPIDConfig{}, nil)
	_, missing["push subscriptions repository"] = NewTenantPushSubscriptionService(runtime, db, nil, stubGuardianSchool{}, VAPIDConfig{}, nil)
	_, missing["push subscriptions schools"] = NewTenantPushSubscriptionService(runtime, db, stubPushSubscriptions{}, nil, VAPIDConfig{}, nil)
	for name, err := range missing {
		assert.Error(t, err, name)
	}
}
