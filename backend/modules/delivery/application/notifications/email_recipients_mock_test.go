// Opt-in e-mail types (#3780) against fakes. The building block knows
// nothing about enrollment, so it is pinned here with a test-only e-mail type
// of its own; the account IDs are in-memory arguments, not database rows.
package notifications_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testEmailType       = "test_email_digest"
	testEmailPermission = "digest:receive"
	emailTenant         = int64(41)
	emailAccountA       = int64(101)
	emailAccountB       = int64(102)
	emailAccountC       = int64(103)
)

func init() {
	notifications.RegisterType(notifications.TypeDefinition{
		Key:        testEmailType,
		Label:      "Testübersicht",
		Portal:     notifications.PortalStaff,
		Channel:    notifications.ChannelEmail,
		Permission: testEmailPermission,
	})
}

type fakeEmailSubscribers struct {
	optedIn   map[string][]int64
	listErr   error
	listTypes []string
}

func (f *fakeEmailSubscribers) ListOptedIn(_ context.Context, notificationType string) ([]int64, error) {
	f.listTypes = append(f.listTypes, notificationType)
	return f.optedIn[notificationType], f.listErr
}

type fakeEmailAccounts struct {
	active      map[int64]bool
	permissions map[int64][]string
	emails      map[int64]string
	tenants     []int64
	emailArgs   []int64
}

func (f *fakeEmailAccounts) ListActiveAccountIDsForTenant(_ context.Context, tenantID int64, accountIDs []int64) ([]int64, error) {
	f.tenants = append(f.tenants, tenantID)
	var active []int64
	for _, id := range accountIDs {
		if f.active[id] {
			active = append(active, id)
		}
	}
	return active, nil
}

func (f *fakeEmailAccounts) FindEffectivePermissionNamesByAccountIDsForTenant(_ context.Context, accountIDs []int64, tenantID int64) (map[int64][]string, error) {
	f.tenants = append(f.tenants, tenantID)
	names := make(map[int64][]string, len(accountIDs))
	for _, id := range accountIDs {
		names[id] = f.permissions[id]
	}
	return names, nil
}

func (f *fakeEmailAccounts) ListAccountEmails(_ context.Context, accountIDs []int64) (map[int64]string, error) {
	f.emailArgs = accountIDs
	emails := make(map[int64]string, len(accountIDs))
	for _, id := range accountIDs {
		emails[id] = f.emails[id]
	}
	return emails, nil
}

func emailCtx() context.Context {
	return tenant.WithTenantID(context.Background(), emailTenant)
}

func TestEmailTypeCatalogue(t *testing.T) {
	t.Parallel()

	def, ok := notifications.EmailType(notifications.TypeEnrollmentSubmitted)
	require.True(t, ok, "the enrollment mail is an e-mail type")
	assert.Equal(t, permissions.ConfigManage, def.Permission)

	_, ok = notifications.EmailType(notifications.TypePickupUpcoming)
	assert.False(t, ok, "a push type is no e-mail type")
	_, ok = notifications.EmailType("nope")
	assert.False(t, ok)

	for _, portal := range []string{notifications.PortalStaff, notifications.PortalSchool, notifications.PortalParent} {
		for _, listed := range notifications.TypesForPortal(portal) {
			assert.NotEqual(t, notifications.ChannelEmail, listed.Channel,
				"e-mail types are decided where their event happens, never on the profile page (%s)", portal)
		}
	}

	assert.Panics(t, func() {
		notifications.RegisterType(notifications.TypeDefinition{
			Key: "test_email_without_permission", Label: "x", Portal: notifications.PortalStaff, Channel: notifications.ChannelEmail,
		})
	}, "an e-mail type without a permission would reach everyone who switched it on")
}

func TestEmailRecipientResolver(t *testing.T) {
	t.Parallel()

	newAccounts := func() *fakeEmailAccounts {
		return &fakeEmailAccounts{
			active: map[int64]bool{emailAccountA: true, emailAccountB: true},
			permissions: map[int64][]string{
				emailAccountA: {testEmailPermission},
				emailAccountB: {"groups:read"},
				emailAccountC: {testEmailPermission},
			},
			emails: map[int64]string{
				emailAccountA: "Leitung@Schule.test",
				emailAccountB: "team@schule.test",
				emailAccountC: "weg@schule.test",
			},
		}
	}

	t.Run("switched on and permitted, deduplicated with the extra addresses", func(t *testing.T) {
		t.Parallel()
		consent := &fakeEmailSubscribers{optedIn: map[string][]int64{testEmailType: {emailAccountA, emailAccountB, emailAccountC}}}
		accounts := newAccounts()
		svc := notifications.NewEmailRecipientResolver(consent, accounts)

		got, err := svc.Recipients(emailCtx(), testEmailType, []string{"extern@example.test", "leitung@schule.test", "EXTERN@example.test"})

		require.NoError(t, err)
		assert.Equal(t, []string{"extern@example.test", "leitung@schule.test"}, got,
			"B lacks the permission, C has no active membership; A's address is already listed")
		assert.Equal(t, []string{testEmailType}, consent.listTypes)
		assert.Equal(t, []int64{emailTenant, emailTenant}, accounts.tenants, "every identity read is bound to the school in context")
		assert.Equal(t, []int64{emailAccountA}, accounts.emailArgs, "only permitted accounts' addresses are read")
	})

	t.Run("nobody switched on keeps the extra addresses", func(t *testing.T) {
		t.Parallel()
		accounts := newAccounts()
		svc := notifications.NewEmailRecipientResolver(&fakeEmailSubscribers{}, accounts)

		got, err := svc.Recipients(emailCtx(), testEmailType, []string{" extern@example.test "})

		require.NoError(t, err)
		assert.Equal(t, []string{"extern@example.test"}, got)
		assert.Empty(t, accounts.tenants, "no identity read without a candidate")
	})

	t.Run("a push type or an unknown type is refused", func(t *testing.T) {
		t.Parallel()
		svc := notifications.NewEmailRecipientResolver(&fakeEmailSubscribers{}, newAccounts())
		for _, key := range []string{notifications.TypePickupUpcoming, "unknown_type"} {
			_, err := svc.Recipients(emailCtx(), key, nil)
			require.ErrorIs(t, err, notifications.ErrUnknownNotificationType, key)
		}
	})

	t.Run("without a school in context nothing is resolved", func(t *testing.T) {
		t.Parallel()
		consent := &fakeEmailSubscribers{optedIn: map[string][]int64{testEmailType: {emailAccountA}}}
		svc := notifications.NewEmailRecipientResolver(consent, newAccounts())

		_, err := svc.Recipients(context.Background(), testEmailType, nil)

		require.Error(t, err)
		assert.Empty(t, consent.listTypes)
	})

	t.Run("a consent read failure is reported", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("boom")
		svc := notifications.NewEmailRecipientResolver(&fakeEmailSubscribers{listErr: boom}, newAccounts())

		_, err := svc.Recipients(emailCtx(), testEmailType, []string{"extern@example.test"})

		require.ErrorIs(t, err, boom)
	})
}

func TestEmailSubscriptionOwnDecision(t *testing.T) {
	t.Parallel()

	repo := &fakePreferenceRepo{stored: map[string]bool{testEmailType: true, notifications.TypePickupUpcoming: true}}
	svc := notifications.NewPreferenceService(repo, allSettingsOn(), nil, nil)
	granted := []string{testEmailPermission}

	on, err := svc.EmailSubscribed(emailCtx(), emailAccountA, granted, testEmailType)
	require.NoError(t, err)
	assert.True(t, on)

	on, err = svc.EmailSubscribed(emailCtx(), emailAccountA, []string{permissions.ConfigManage}, notifications.TypeEnrollmentSubmitted)
	require.NoError(t, err)
	assert.False(t, on, "no stored decision reads as off")

	require.NoError(t, svc.SetEmailSubscribed(emailCtx(), emailAccountA, granted, testEmailType, false))
	assert.Equal(t, []recordedConsent{{AccountID: emailAccountA, NotificationType: testEmailType, Enabled: false}}, repo.upserted)

	_, err = svc.EmailSubscribed(emailCtx(), emailAccountA, []string{"groups:read"}, testEmailType)
	require.ErrorIs(t, err, notifications.ErrEmailPermissionRequired, "without the type's permission nothing is read")
	require.ErrorIs(t, svc.SetEmailSubscribed(emailCtx(), emailAccountA, nil, testEmailType, true), notifications.ErrEmailPermissionRequired)

	_, err = svc.EmailSubscribed(emailCtx(), emailAccountA, granted, notifications.TypePickupUpcoming)
	require.ErrorIs(t, err, notifications.ErrUnknownNotificationType, "push decisions stay on the profile page")
	require.ErrorIs(t, svc.SetEmailSubscribed(emailCtx(), emailAccountA, granted, notifications.TypePickupUpcoming, true), notifications.ErrUnknownNotificationType)
	require.ErrorIs(t, svc.SetForAccount(emailCtx(), emailAccountA, testEmailType, true), notifications.ErrUnknownNotificationType,
		"the profile page cannot switch an e-mail type on past its permission check")
	assert.Len(t, repo.upserted, 1)
}
