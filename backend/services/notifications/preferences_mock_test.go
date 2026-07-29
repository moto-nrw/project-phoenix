// Consent service tests against fakes. The staff-portal paths are pure — a
// repository double and a settings double are enough — which is why they live
// here rather than in the database-backed file next to them.
package notifications_test

import (
	"context"
	"errors"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePreferenceRepo implements the four methods the service actually calls.
// The embedded interface is nil on purpose: if the service ever reaches for
// another method, the test panics instead of quietly passing.
type fakePreferenceRepo struct {
	userModel.NotificationPreferenceRepository

	stored        []*userModel.NotificationPreference
	optedIn       map[string][]int64
	upserted      []*userModel.NotificationPreference
	disabled      []int64
	disabledTypes [][]string
	listErr       error
	upsertErr     error
	filterErr     error
	disableErr    error
	filterArgs    []int64
}

func (f *fakePreferenceRepo) ListByAccount(_ context.Context, _ int64) ([]*userModel.NotificationPreference, error) {
	return f.stored, f.listErr
}

func (f *fakePreferenceRepo) Upsert(_ context.Context, pref *userModel.NotificationPreference) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.upserted = append(f.upserted, pref)
	return nil
}

func (f *fakePreferenceRepo) FilterOptedIn(_ context.Context, notificationType string, accountIDs []int64) ([]int64, error) {
	if f.filterErr != nil {
		return nil, f.filterErr
	}
	f.filterArgs = accountIDs
	return f.optedIn[notificationType], nil
}

func (f *fakePreferenceRepo) DisableAllForAccount(_ context.Context, accountID int64, notificationTypes []string) error {
	if f.disableErr != nil {
		return f.disableErr
	}
	f.disabled = append(f.disabled, accountID)
	f.disabledTypes = append(f.disabledTypes, notificationTypes)
	return nil
}

// allSettingsOn answers every boolean setting with true: dispatch is on and no
// type is gated off school-wide.
func allSettingsOn() *configtest.Mock {
	return &configtest.Mock{
		ResolveBoolFn: func(_ context.Context, _ string) (bool, error) { return true, nil },
	}
}

// The account IDs here are in-memory arguments to fakes, not database rows.
const (
	prefAccountA int64 = 41
	prefAccountB int64 = 42
)

func TestGetForAccountMergesStoredDecisions(t *testing.T) {
	repo := &fakePreferenceRepo{stored: []*userModel.NotificationPreference{
		{AccountID: prefAccountA, NotificationType: notifications.TypePickupUpcoming, Enabled: true},
		{AccountID: prefAccountA, NotificationType: notifications.TypePickupOverdue, Enabled: false},
	}}
	svc := notifications.NewPreferenceService(repo, allSettingsOn(), nil, nil)

	overview, err := svc.GetForAccount(context.Background(), prefAccountA, notifications.PortalStaff)
	require.NoError(t, err)

	assert.True(t, overview.TenantEnabled)
	byKey := make(map[string]bool, len(overview.Types))
	for _, state := range overview.Types {
		byKey[state.Key] = state.Enabled
		assert.NotEmpty(t, state.Label, "%s renders on the profile page and needs copy", state.Key)
		assert.True(t, state.Available, "no gate is off in this setup")
	}

	assert.True(t, byKey[notifications.TypePickupUpcoming])
	assert.False(t, byKey[notifications.TypePickupOverdue], "an explicit no reads as off")
	assert.False(t, byKey[notifications.TypeMyActivityStarting], "a type never touched reads as off")
	assert.NotContains(t, byKey, notifications.TypeParentAnnouncement,
		"a parent type must not surface in the staff catalogue")
}

// A type the school switched off is still listed and still switchable — the
// person's choice has to survive an admin toggling the school setting.
func TestGetForAccountMarksSchoolGatedTypesUnavailable(t *testing.T) {
	settings := &configtest.Mock{
		ResolveBoolFn: func(_ context.Context, key string) (bool, error) {
			return key != configModel.KeyRemindersPickupUpcomingEnabled, nil
		},
	}
	svc := notifications.NewPreferenceService(&fakePreferenceRepo{}, settings, nil, nil)

	overview, err := svc.GetForAccount(context.Background(), prefAccountA, notifications.PortalStaff)
	require.NoError(t, err)

	for _, state := range overview.Types {
		if state.Key == notifications.TypePickupUpcoming {
			assert.False(t, state.Available)
			return
		}
	}
	t.Fatal("pickup_upcoming missing from the staff catalogue")
}

func TestGetForAccountFailures(t *testing.T) {
	t.Run("account id is required", func(t *testing.T) {
		svc := notifications.NewPreferenceService(&fakePreferenceRepo{}, allSettingsOn(), nil, nil)
		_, err := svc.GetForAccount(context.Background(), 0, notifications.PortalStaff)
		require.Error(t, err)
	})

	t.Run("a failing repository surfaces", func(t *testing.T) {
		repo := &fakePreferenceRepo{listErr: errors.New("boom")}
		svc := notifications.NewPreferenceService(repo, allSettingsOn(), nil, nil)
		_, err := svc.GetForAccount(context.Background(), prefAccountA, notifications.PortalStaff)
		require.Error(t, err)
	})

	t.Run("a failing settings read surfaces", func(t *testing.T) {
		settings := &configtest.Mock{
			ResolveBoolFn: func(_ context.Context, _ string) (bool, error) { return false, errors.New("boom") },
		}
		svc := notifications.NewPreferenceService(&fakePreferenceRepo{}, settings, nil, nil)
		_, err := svc.GetForAccount(context.Background(), prefAccountA, notifications.PortalStaff)
		require.Error(t, err, "a broken read must not silently widen or narrow who gets notified")
	})

	t.Run("without a settings service nothing is enabled", func(t *testing.T) {
		svc := notifications.NewPreferenceService(&fakePreferenceRepo{}, nil, nil, nil)
		overview, err := svc.GetForAccount(context.Background(), prefAccountA, notifications.PortalStaff)
		require.NoError(t, err)
		assert.False(t, overview.TenantEnabled)
	})
}

func TestSetForAccount(t *testing.T) {
	t.Run("records the decision", func(t *testing.T) {
		repo := &fakePreferenceRepo{}
		svc := notifications.NewPreferenceService(repo, allSettingsOn(), nil, nil)

		require.NoError(t, svc.SetForAccount(context.Background(), prefAccountA, notifications.TypePickupUpcoming, true))

		require.Len(t, repo.upserted, 1)
		assert.Equal(t, prefAccountA, repo.upserted[0].AccountID)
		assert.Equal(t, notifications.TypePickupUpcoming, repo.upserted[0].NotificationType)
		assert.True(t, repo.upserted[0].Enabled)
	})

	t.Run("an unknown type is rejected", func(t *testing.T) {
		repo := &fakePreferenceRepo{}
		svc := notifications.NewPreferenceService(repo, allSettingsOn(), nil, nil)

		err := svc.SetForAccount(context.Background(), prefAccountA, "not_a_type", true)
		require.ErrorIs(t, err, notifications.ErrUnknownNotificationType)
		assert.Empty(t, repo.upserted)
	})

	t.Run("a parent type is rejected on the staff portal", func(t *testing.T) {
		repo := &fakePreferenceRepo{}
		svc := notifications.NewPreferenceService(repo, allSettingsOn(), nil, nil)

		err := svc.SetForAccount(context.Background(), prefAccountA, notifications.TypeParentAnnouncement, true)
		require.ErrorIs(t, err, notifications.ErrUnknownNotificationType)
		assert.Empty(t, repo.upserted)
	})

	t.Run("account id is required", func(t *testing.T) {
		svc := notifications.NewPreferenceService(&fakePreferenceRepo{}, allSettingsOn(), nil, nil)
		require.Error(t, svc.SetForAccount(context.Background(), 0, notifications.TypePickupUpcoming, true))
	})

	t.Run("a failing write surfaces", func(t *testing.T) {
		repo := &fakePreferenceRepo{upsertErr: errors.New("boom")}
		svc := notifications.NewPreferenceService(repo, allSettingsOn(), nil, nil)
		require.Error(t, svc.SetForAccount(context.Background(), prefAccountA, notifications.TypePickupUpcoming, true))
	})
}

func TestDisableAllForAccount(t *testing.T) {
	repo := &fakePreferenceRepo{}
	svc := notifications.NewPreferenceService(repo, allSettingsOn(), nil, nil)

	require.NoError(t, svc.DisableAllForAccount(context.Background(), prefAccountA))
	assert.Equal(t, []int64{prefAccountA}, repo.disabled)
	require.Len(t, repo.disabledTypes, 1)
	assert.Contains(t, repo.disabledTypes[0], notifications.TypePickupUpcoming)
	assert.NotContains(t, repo.disabledTypes[0], notifications.TypeParentAnnouncement,
		"the staff action must not disable parent-portal consent")

	require.Error(t, svc.DisableAllForAccount(context.Background(), 0))

	failing := notifications.NewPreferenceService(&fakePreferenceRepo{disableErr: errors.New("boom")}, allSettingsOn(), nil, nil)
	require.Error(t, failing.DisableAllForAccount(context.Background(), prefAccountA))
}

// FilterOptedIn is the single enforcement point for "nothing without consent",
// so each of its answers is pinned separately.
func TestFilterOptedIn(t *testing.T) {
	candidates := []int64{prefAccountA, prefAccountB}

	t.Run("returns only those who agreed", func(t *testing.T) {
		repo := &fakePreferenceRepo{optedIn: map[string][]int64{
			notifications.TypePickupUpcoming: {prefAccountA},
		}}
		svc := notifications.NewPreferenceService(repo, allSettingsOn(), nil, nil)

		got, err := svc.FilterOptedIn(context.Background(), notifications.TypePickupUpcoming, candidates)
		require.NoError(t, err)
		assert.Equal(t, []int64{prefAccountA}, got)
		assert.Equal(t, candidates, repo.filterArgs, "the whole candidate set is offered to the filter")
	})

	t.Run("no candidates means nobody", func(t *testing.T) {
		svc := notifications.NewPreferenceService(&fakePreferenceRepo{}, allSettingsOn(), nil, nil)
		got, err := svc.FilterOptedIn(context.Background(), notifications.TypePickupUpcoming, nil)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("an unknown type fails closed", func(t *testing.T) {
		repo := &fakePreferenceRepo{optedIn: map[string][]int64{"": candidates}}
		svc := notifications.NewPreferenceService(repo, allSettingsOn(), nil, nil)

		got, err := svc.FilterOptedIn(context.Background(), "not_a_type", candidates)
		require.ErrorIs(t, err, notifications.ErrUnknownNotificationType)
		assert.Empty(t, got, "a producer naming a type that does not exist must reach nobody, not everybody")
	})

	t.Run("a type the school switched off reaches nobody", func(t *testing.T) {
		settings := &configtest.Mock{
			ResolveBoolFn: func(_ context.Context, key string) (bool, error) {
				return key != configModel.KeyRemindersPickupUpcomingEnabled, nil
			},
		}
		repo := &fakePreferenceRepo{optedIn: map[string][]int64{
			notifications.TypePickupUpcoming: candidates,
		}}
		svc := notifications.NewPreferenceService(repo, settings, nil, nil)

		got, err := svc.FilterOptedIn(context.Background(), notifications.TypePickupUpcoming, candidates)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("a failing gate read surfaces", func(t *testing.T) {
		settings := &configtest.Mock{
			ResolveBoolFn: func(_ context.Context, _ string) (bool, error) { return false, errors.New("boom") },
		}
		svc := notifications.NewPreferenceService(&fakePreferenceRepo{}, settings, nil, nil)
		_, err := svc.FilterOptedIn(context.Background(), notifications.TypePickupUpcoming, candidates)
		require.Error(t, err)
	})

	t.Run("a failing repository surfaces", func(t *testing.T) {
		repo := &fakePreferenceRepo{filterErr: errors.New("boom")}
		svc := notifications.NewPreferenceService(repo, allSettingsOn(), nil, nil)
		_, err := svc.FilterOptedIn(context.Background(), notifications.TypePickupUpcoming, candidates)
		require.Error(t, err)
	})
}

// The guardian portal is cross-tenant and needs the database handle plus the
// mapping repository. Without them the service must refuse rather than act on
// the request context's tenant, which for a parent request is nobody's school.
func TestGuardianPathsRequireTheirDependencies(t *testing.T) {
	svc := notifications.NewPreferenceService(&fakePreferenceRepo{}, allSettingsOn(), nil, nil)
	ctx := context.Background()

	_, err := svc.GetForParent(ctx, prefAccountA)
	require.Error(t, err)

	require.Error(t, svc.SetForParent(ctx, prefAccountA, notifications.TypeParentAnnouncement, true))
	require.Error(t, svc.DisableAllForParent(ctx, prefAccountA))

	t.Run("account id is required", func(t *testing.T) {
		_, err := svc.GetForParent(ctx, 0)
		require.Error(t, err)
		require.Error(t, svc.SetForParent(ctx, 0, notifications.TypeParentAnnouncement, true))
		require.Error(t, svc.DisableAllForParent(ctx, 0))
	})

	t.Run("a staff type cannot be set through the parent portal", func(t *testing.T) {
		err := svc.SetForParent(ctx, prefAccountA, notifications.TypePickupUpcoming, true)
		require.ErrorIs(t, err, notifications.ErrUnknownNotificationType)
	})
}
