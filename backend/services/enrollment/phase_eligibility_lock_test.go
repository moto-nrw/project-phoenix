package enrollment

import (
	"context"
	"errors"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The collectability guards and the settings-side collection guard protect two
// halves of one invariant: an active phase restricted to classes or grades must
// never coexist with the collection toggle that feeds the restriction being
// off. Each side validates on a read and then writes, so without a shared lock
// two concurrent writers both pass on a stale read and commit the broken pair.
// These tests pin that the phase side actually takes that lock, and that it
// refuses to proceed when the lock cannot be taken — a guard that validated on
// an unlocked read would be exactly the race the lock exists to close (#1663).

// lockRecordingSettings is a settings double that records lock acquisitions and
// reports both collection toggles as on, so the guards run to completion unless
// the lock itself fails.
func lockRecordingSettings(lockErr error) (*configtest.Mock, *int) {
	calls := 0
	mock := &configtest.Mock{
		ResolveBoolFn: func(context.Context, string) (bool, error) { return true, nil },
		ResolveIntFn:  func(context.Context, string) (int, error) { return 4, nil },
		LockClassCollectionPairFn: func(context.Context) error {
			calls++
			return lockErr
		},
	}
	return mock, &calls
}

func TestEnsureEligibleClassesCollectable_TakesSharedLock(t *testing.T) {
	t.Run("acquires the lock before reading the toggles", func(t *testing.T) {
		settings, calls := lockRecordingSettings(nil)

		require.NoError(t, ensureEligibleClassesCollectable(context.Background(), settings, []string{"3a"}))
		assert.Equal(t, 1, *calls, "the class guard must serialize against the settings side")
	})

	t.Run("a failed lock aborts instead of validating on an unlocked read", func(t *testing.T) {
		lockErr := errors.New("lock unavailable")
		settings, _ := lockRecordingSettings(lockErr)

		err := ensureEligibleClassesCollectable(context.Background(), settings, []string{"3a"})

		require.Error(t, err)
		assert.ErrorIs(t, err, lockErr)
		assert.NotErrorIs(t, err, ErrInvalidPhase,
			"an unavailable lock is operational, not a rejection of the phase")
	})

	t.Run("no restriction needs no lock", func(t *testing.T) {
		settings, calls := lockRecordingSettings(nil)

		require.NoError(t, ensureEligibleClassesCollectable(context.Background(), settings, nil))
		assert.Zero(t, *calls, "an unrestricted phase touches no shared invariant")
	})
}

func TestEnsureEligibleGradeLevelsCollectable_TakesSharedLock(t *testing.T) {
	t.Run("acquires the lock before reading the grade toggle", func(t *testing.T) {
		settings, calls := lockRecordingSettings(nil)

		require.NoError(t, ensureEligibleGradeLevelsCollectable(context.Background(), settings, []int{3}))
		assert.Equal(t, 1, *calls, "the grade guard must serialize against the settings side")
	})

	t.Run("a failed lock aborts instead of validating on an unlocked read", func(t *testing.T) {
		lockErr := errors.New("lock unavailable")
		settings, _ := lockRecordingSettings(lockErr)

		err := ensureEligibleGradeLevelsCollectable(context.Background(), settings, []int{3})

		require.Error(t, err)
		assert.ErrorIs(t, err, lockErr)
		assert.NotErrorIs(t, err, ErrInvalidPhase)
	})

	t.Run("no restriction needs no lock", func(t *testing.T) {
		settings, calls := lockRecordingSettings(nil)

		require.NoError(t, ensureEligibleGradeLevelsCollectable(context.Background(), settings, nil))
		assert.Zero(t, *calls)
	})
}

// A settings double that does NOT implement LockClassCollectionPair must still
// be accepted: the guard type-asserts the lock as an optional capability so
// minimal fakes and any future partial implementation degrade to best-effort
// rather than failing the phase write outright.
type locklessSettings struct{}

func (locklessSettings) ResolveBool(context.Context, string) (bool, error) { return true, nil }
func (locklessSettings) ResolveInt(context.Context, string) (int, error)   { return 4, nil }

func TestEligibilityGuards_LockIsOptional(t *testing.T) {
	require.NoError(t, ensureEligibleClassesCollectable(context.Background(), locklessSettings{}, []string{"3a"}))
	require.NoError(t, ensureEligibleGradeLevelsCollectable(context.Background(), locklessSettings{}, []int{3}))
}

// The guards must still reject the pair they exist to catch — a lock that is
// taken but a toggle that is off is a real invalid phase (400), not a 500.
func TestEligibilityGuards_RejectUncollectableRestriction(t *testing.T) {
	offSettings := &configtest.Mock{
		ResolveBoolFn: func(_ context.Context, key string) (bool, error) {
			return key != configModel.KeyEnrollmentCollectSchoolClass, nil
		},
		ResolveIntFn: func(context.Context, string) (int, error) { return 4, nil },
	}

	err := ensureEligibleClassesCollectable(context.Background(), offSettings, []string{"3a"})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidPhase)
}
