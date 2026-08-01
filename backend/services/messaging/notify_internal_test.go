// Guardian push for staff replies (#1671). In-memory: only the notifier and the
// consent service are faked, and notifyGuardianDevice is driven directly — the
// send paths that call it are covered by the messaging tests around them.
package messaging

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type captureNotifier struct {
	events []notifications.Event
	err    error
}

func (c *captureNotifier) Notify(_ context.Context, event notifications.Event) error {
	c.events = append(c.events, event)
	return c.err
}

type stubPreferences struct {
	notifications.PreferenceService
	optedIn []int64
	err     error
	gotType string
	asked   []int64
}

func (s *stubPreferences) FilterOptedIn(_ context.Context, notificationType string, accountIDs []int64) ([]int64, error) {
	s.gotType = notificationType
	s.asked = append([]int64(nil), accountIDs...)
	return s.optedIn, s.err
}

func testThread() *usersModels.ParentMessageThread {
	thread := &usersModels.ParentMessageThread{
		StudentID:         55,
		GuardianAccountID: 42,
	}
	thread.ID = 9
	thread.SetTenantID(7)
	return thread
}

func newNotifyService(notifier notifications.Service, prefs notifications.PreferenceService) *Service {
	return NewService(Config{
		Logger:      slog.Default(),
		Notifier:    notifier,
		Preferences: prefs,
	})
}

func TestNotifyGuardianDevice(t *testing.T) {
	t.Run("addresses the thread's guardian and nobody else", func(t *testing.T) {
		notifier := &captureNotifier{}
		prefs := &stubPreferences{optedIn: []int64{42}}

		newNotifyService(notifier, prefs).notifyGuardianDevice(context.Background(), testThread())

		require.Len(t, notifier.events, 1)
		event := notifier.events[0]
		assert.Equal(t, notifications.TypeParentMessage, event.Type)
		assert.Equal(t, notifications.ScopeGuardian, event.Audience.Scope)
		assert.Equal(t, []int64{42}, event.Audience.GuardianAccountIDs)
		assert.Equal(t, int64(7), event.Audience.TenantID)
		assert.Equal(t, "/messages", event.DeepLink)

		assert.Equal(t, notifications.TypeParentMessage, prefs.gotType,
			"consent has to be asked for this type, not for another producer's")
		assert.Equal(t, []int64{42}, prefs.asked)
	})

	t.Run("says nothing about the child or the sender", func(t *testing.T) {
		notifier := &captureNotifier{}
		newNotifyService(notifier, &stubPreferences{optedIn: []int64{42}}).
			notifyGuardianDevice(context.Background(), testThread())

		require.Len(t, notifier.events, 1)
		// A push is rendered on a lock screen. Everything identifying sits behind
		// the authenticated deep link.
		assert.NotContains(t, notifier.events[0].Body, "55")
		assert.NotEmpty(t, notifier.events[0].Title)
	})

	t.Run("a guardian who did not agree is not pushed to", func(t *testing.T) {
		notifier := &captureNotifier{}

		newNotifyService(notifier, &stubPreferences{optedIn: nil}).
			notifyGuardianDevice(context.Background(), testThread())

		assert.Empty(t, notifier.events, "no row means off")
	})

	t.Run("an unreadable consent state pushes to nobody", func(t *testing.T) {
		notifier := &captureNotifier{}

		newNotifyService(notifier, &stubPreferences{err: errors.New("db down")}).
			notifyGuardianDevice(context.Background(), testThread())

		assert.Empty(t, notifier.events, "consent failures must fail closed")
	})

	t.Run("without a consent service nothing is pushed", func(t *testing.T) {
		notifier := &captureNotifier{}

		newNotifyService(notifier, nil).notifyGuardianDevice(context.Background(), testThread())

		assert.Empty(t, notifier.events,
			"an unwired dependency must not turn into consent-free delivery")
	})

	t.Run("a suppressed dispatch is not an error for the caller", func(t *testing.T) {
		notifier := &captureNotifier{err: notifications.ErrDisabled}

		assert.NotPanics(t, func() {
			newNotifyService(notifier, &stubPreferences{optedIn: []int64{42}}).
				notifyGuardianDevice(context.Background(), testThread())
		})
	})
}
