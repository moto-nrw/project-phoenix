// Guardian push for staff replies (#1671). In-memory: only the notifier and the
// consent service are faked, and notifyGuardianDevice is driven directly — the
// send paths that call it are covered by the messaging tests around them.
package messaging

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type captureNotifier struct {
	events []communication.NotificationEvent
	err    error
}

func (c *captureNotifier) Notify(_ context.Context, event communication.NotificationEvent) error {
	c.events = append(c.events, event)
	return c.err
}

type sequencedNotifier struct {
	events []communication.NotificationEvent
	errs   []error
}

func (c *sequencedNotifier) Notify(_ context.Context, event communication.NotificationEvent) error {
	c.events = append(c.events, event)
	if len(c.errs) == 0 {
		return nil
	}
	err := c.errs[0]
	c.errs = c.errs[1:]
	return err
}

type stubPreferences struct {
	communication.NotificationPreferences
	optedIn []int64
	err     error
	gotType string
	asked   []int64
}

type stubGuardianProfiles struct {
	profile *communication.ParentMessageGuardianContact
}

func (s stubGuardianProfiles) FindMessageGuardianProfile(context.Context, int64) (*communication.ParentMessageGuardianContact, error) {
	return s.profile, nil
}

func (s *stubPreferences) FilterOptedIn(_ context.Context, notificationType string, accountIDs []int64) ([]int64, error) {
	s.gotType = notificationType
	s.asked = append([]int64(nil), accountIDs...)
	return s.optedIn, s.err
}

// sessionlessRequest is a request without an authenticated account; the push
// paths under test read none.
type sessionlessRequest struct{}

func (sessionlessRequest) AccountID(context.Context) int64 { return 0 }

func (sessionlessRequest) Permissions(context.Context) []string { return nil }

func testThread() *ParentMessageThread {
	thread := &ParentMessageThread{
		StudentID:         55,
		GuardianAccountID: 42,
	}
	thread.ID = 9
	thread.SetTenantID(7)
	return thread
}

func newNotifyService(notifier communication.NotificationSender, prefs communication.NotificationPreferences) *Service {
	return NewService(Config{
		Session:     sessionlessRequest{},
		Logger:      slog.Default(),
		Notifier:    notifier,
		Preferences: prefs,
	}, testpkg.TenantTransactionRuntime{}, devices.ParentDeliveryPolicy{})
}

func TestNotifyGuardianDevice(t *testing.T) {
	t.Parallel()

	t.Run("addresses the thread's guardian and nobody else", func(t *testing.T) {
		notifier := &captureNotifier{}
		prefs := &stubPreferences{optedIn: []int64{42}}

		newNotifyService(notifier, prefs).notifyGuardianDevice(context.Background(), testThread(), 123)

		require.Len(t, notifier.events, 1)
		event := notifier.events[0]
		assert.Equal(t, communication.TypeParentMessage, event.Type)
		assert.Equal(t, communication.ScopeGuardian, event.Audience.Scope)
		assert.Equal(t, []int64{42}, event.Audience.GuardianAccountIDs)
		assert.Equal(t, int64(7), event.Audience.TenantID)
		assert.Equal(t, "/messages", event.DeepLink)

		assert.Equal(t, communication.TypeParentMessage, prefs.gotType,
			"consent has to be asked for this type, not for another producer's")
		assert.Equal(t, []int64{42}, prefs.asked)
	})

	t.Run("carries the child so the delivery re-checks access to it", func(t *testing.T) {
		notifier := &captureNotifier{}

		newNotifyService(notifier, &stubPreferences{optedIn: []int64{42}}).
			notifyGuardianDevice(context.Background(), testThread(), 123)

		require.Len(t, notifier.events, 1)
		// Not payload: the device lookup runs in a later transaction and keeps
		// only recipients who still hold parent_portal.access for this child.
		assert.Equal(t, []int64{55}, notifier.events[0].Audience.StudentIDs)
	})

	t.Run("says nothing about the child or the sender", func(t *testing.T) {
		notifier := &captureNotifier{}
		newNotifyService(notifier, &stubPreferences{optedIn: []int64{42}}).
			notifyGuardianDevice(context.Background(), testThread(), 123)

		require.Len(t, notifier.events, 1)
		// A push is rendered on a lock screen. Everything identifying sits behind
		// the authenticated deep link.
		assert.NotContains(t, notifier.events[0].Body, "55")
		assert.NotEmpty(t, notifier.events[0].Title)
	})

	t.Run("uses the guardian portal locale", func(t *testing.T) {
		notifier := &captureNotifier{}
		locale := "en"
		s := NewService(Config{
			Session: sessionlessRequest{}, Logger: slog.Default(), Notifier: notifier,
			Preferences:      &stubPreferences{optedIn: []int64{42}},
			GuardianProfiles: stubGuardianProfiles{profile: &communication.ParentMessageGuardianContact{PortalLocale: &locale}},
		}, testpkg.TenantTransactionRuntime{}, devices.ParentDeliveryPolicy{})

		s.notifyGuardianDevice(context.Background(), testThread(), 123)

		require.Len(t, notifier.events, 1)
		assert.Equal(t, "New message from the OGS", notifier.events[0].Title)
	})

	t.Run("a guardian who did not agree is not pushed to", func(t *testing.T) {
		notifier := &captureNotifier{}

		newNotifyService(notifier, &stubPreferences{optedIn: nil}).
			notifyGuardianDevice(context.Background(), testThread(), 123)

		assert.Empty(t, notifier.events, "no row means off")
	})

	t.Run("an unreadable consent state pushes to nobody", func(t *testing.T) {
		notifier := &captureNotifier{}

		newNotifyService(notifier, &stubPreferences{err: errors.New("db down")}).
			notifyGuardianDevice(context.Background(), testThread(), 123)

		assert.Empty(t, notifier.events, "consent failures must fail closed")
	})

	t.Run("without a consent service nothing is pushed", func(t *testing.T) {
		notifier := &captureNotifier{}

		newNotifyService(notifier, nil).notifyGuardianDevice(context.Background(), testThread(), 123)

		assert.Empty(t, notifier.events,
			"an unwired dependency must not turn into consent-free delivery")
	})

	t.Run("a suppressed dispatch is not an error for the caller", func(t *testing.T) {
		notifier := &captureNotifier{err: devices.ErrNotificationsDisabled}

		assert.NotPanics(t, func() {
			newNotifyService(notifier, &stubPreferences{optedIn: []int64{42}}).
				notifyGuardianDevice(context.Background(), testThread(), 123)
		})
	})

	t.Run("a failed dispatch is logged, never raised to the sender", func(t *testing.T) {
		// The staff member's message is already stored and broadcast. A push
		// that could not be delivered must not turn their send into an error.
		notifier := &captureNotifier{err: errors.New("push service unreachable")}

		assert.NotPanics(t, func() {
			newNotifyService(notifier, &stubPreferences{optedIn: []int64{42}}).
				notifyGuardianDevice(context.Background(), testThread(), 123)
		})
	})

	t.Run("a thread without a guardian account has nobody to push to", func(t *testing.T) {
		notifier := &captureNotifier{}
		prefs := &stubPreferences{optedIn: []int64{42}}
		thread := testThread()
		thread.GuardianAccountID = 0

		newNotifyService(notifier, prefs).notifyGuardianDevice(context.Background(), thread, 123)

		assert.Empty(t, notifier.events)
		assert.Nil(t, prefs.asked, "consent must not even be asked for a missing account")
	})

	t.Run("no thread is a no-op", func(t *testing.T) {
		notifier := &captureNotifier{}

		assert.NotPanics(t, func() {
			newNotifyService(notifier, &stubPreferences{optedIn: []int64{42}}).
				notifyGuardianDevice(context.Background(), nil, 123)
		})
		assert.Empty(t, notifier.events)
	})
}

func TestPushGroupMessage_ContinuesAfterSuppressedRecipient(t *testing.T) {
	t.Parallel()

	notifier := &sequencedNotifier{errs: []error{devices.ErrNotificationsDisabled, nil}}
	thread := &communication.ParentGroupThread{ID: 9, TenantID: 7}
	newNotifyService(notifier, &stubPreferences{optedIn: []int64{42, 43}}).pushGroupMessage(
		context.Background(),
		thread,
		123,
		[]int64{42, 43},
		map[int64][]int64{42: {55}, 43: {56}},
	)

	require.Len(t, notifier.events, 2, "a suppressed push for one guardian must not skip later opted-in guardians")
	assert.Equal(t, []int64{42}, notifier.events[0].Audience.GuardianAccountIDs)
	assert.Equal(t, []int64{43}, notifier.events[1].Audience.GuardianAccountIDs)
}
