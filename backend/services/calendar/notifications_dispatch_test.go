package calendar

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dispatchNotifier struct {
	events []notifications.Event
	err    error
}

func (n *dispatchNotifier) Notify(_ context.Context, event notifications.Event) error {
	if n.err != nil {
		return n.err
	}
	n.events = append(n.events, event)
	return nil
}

func (n *dispatchNotifier) NotifySynchronously(ctx context.Context, event notifications.Event) error {
	return n.Notify(ctx, event)
}

type dispatchPreferences struct {
	notifications.PreferenceService
	optedIn  []int64
	err      error
	askedFor string
}

func (p *dispatchPreferences) FilterOptedIn(_ context.Context, notificationType string, accountIDs []int64) ([]int64, error) {
	p.askedFor = notificationType
	if p.err != nil {
		return nil, p.err
	}
	if p.optedIn != nil {
		return p.optedIn, nil
	}
	return accountIDs, nil
}

func guardianProfile(id int64, accountID *int64) *userModels.GuardianProfile {
	profile := &userModels.GuardianProfile{AccountID: accountID}
	profile.ID = id
	return profile
}

// The audience a notification travels with is a list of account ids. Turning
// guardian profiles into that list is where a profile that cannot sign in, and
// a parent listed twice (once per child), have to fall out.
func TestGuardianAccountIDs(t *testing.T) {
	shared := int64(77)
	other := int64(88)
	unusable := int64(0)

	profiles := map[int64]*userModels.GuardianProfile{
		1: guardianProfile(1, &shared),
		2: guardianProfile(2, &shared),
		3: guardianProfile(3, &other),
		4: guardianProfile(4, nil),
		5: guardianProfile(5, &unusable),
	}

	accountIDs := guardianAccountIDs([]int64{1, 2, 3, 4, 5, 9}, profiles)

	assert.Equal(t, []int64{shared, other}, accountIDs,
		"a parent of two invited children must be addressed once, and an account-less profile not at all")
	assert.Empty(t, guardianAccountIDs(nil, profiles))
}

// dispatchGuardianAccountDevices is the single gate every appointment push goes
// through. Consent narrows the audience the appointment already defined — it
// never widens it, and a missing dependency means "do not push".
func TestDispatchGuardianAccountDevices(t *testing.T) {
	ctx := context.Background()
	accounts := []int64{77}

	t.Run("without a notifier or an audience nothing is dispatched", func(t *testing.T) {
		s := &service{}
		dispatched, err := s.dispatchGuardianAccountDevices(ctx, helperAppointment(), platformModels.EmailKindAppointmentPublished, accounts)
		require.NoError(t, err)
		assert.False(t, dispatched)

		s = &service{cfg: Config{Notifier: &dispatchNotifier{}, Preferences: &dispatchPreferences{}}}
		dispatched, err = s.dispatchGuardianAccountDevices(ctx, helperAppointment(), platformModels.EmailKindAppointmentPublished, nil)
		require.NoError(t, err)
		assert.False(t, dispatched)
	})

	t.Run("an unwired consent service must not become consent-free delivery", func(t *testing.T) {
		notifier := &dispatchNotifier{}
		s := &service{cfg: Config{Notifier: notifier}}

		dispatched, err := s.dispatchGuardianAccountDevices(ctx, helperAppointment(), platformModels.EmailKindAppointmentPublished, accounts)
		require.NoError(t, err)
		assert.False(t, dispatched)
		assert.Empty(t, notifier.events)
	})

	t.Run("an unreadable consent state fails the dispatch", func(t *testing.T) {
		consentErr := errors.New("preferences unavailable")
		s := &service{cfg: Config{
			Notifier:    &dispatchNotifier{},
			Preferences: &dispatchPreferences{err: consentErr},
		}}

		dispatched, err := s.dispatchGuardianAccountDevices(ctx, helperAppointment(), platformModels.EmailKindAppointmentPublished, accounts)
		require.ErrorIs(t, err, consentErr)
		assert.False(t, dispatched)
	})

	t.Run("nobody opted in means nothing to deliver", func(t *testing.T) {
		notifier := &dispatchNotifier{}
		s := &service{cfg: Config{
			Notifier:    notifier,
			Preferences: &dispatchPreferences{optedIn: []int64{}},
		}}

		dispatched, err := s.dispatchGuardianAccountDevices(ctx, helperAppointment(), platformModels.EmailKindAppointmentPublished, accounts)
		require.NoError(t, err)
		assert.False(t, dispatched)
		assert.Empty(t, notifier.events)
	})

	t.Run("a cancellation is urgent, a reminder is its own type", func(t *testing.T) {
		notifier := &dispatchNotifier{}
		prefs := &dispatchPreferences{}
		s := &service{cfg: Config{Notifier: notifier, Preferences: prefs}}

		dispatched, err := s.dispatchGuardianAccountDevices(ctx, helperAppointment(), platformModels.EmailKindAppointmentCancelled, accounts)
		require.NoError(t, err)
		assert.True(t, dispatched)
		require.Len(t, notifier.events, 1)
		assert.Equal(t, notifications.PriorityHigh, notifier.events[0].Priority,
			"a cancelled appointment is the one a parent must not miss")
		assert.Equal(t, notifications.TypeParentAppointment, notifier.events[0].Type)
		assert.Equal(t, parentCalendarDeepLink, notifier.events[0].DeepLink)
		assert.Equal(t, accounts, notifier.events[0].Audience.GuardianAccountIDs)

		dispatched, err = s.dispatchGuardianAccountDevices(ctx, helperAppointment(), platformModels.EmailKindAppointmentReminder, accounts)
		require.NoError(t, err)
		assert.True(t, dispatched)
		require.Len(t, notifier.events, 2)
		assert.Equal(t, notifications.TypeParentAppointmentReminder, notifier.events[1].Type)
		assert.Equal(t, notifications.PriorityNormal, notifier.events[1].Priority)
		assert.Equal(t, notifications.TypeParentAppointmentReminder, prefs.askedFor,
			"consent has to be read for the type actually being sent")
	})

	t.Run("a failed dispatch is reported as not delivered", func(t *testing.T) {
		dispatchErr := errors.New("push service unreachable")
		s := &service{cfg: Config{
			Notifier:    &dispatchNotifier{err: dispatchErr},
			Preferences: &dispatchPreferences{},
		}}

		dispatched, err := s.dispatchGuardianAccountDevices(ctx, helperAppointment(), platformModels.EmailKindAppointmentPublished, accounts)
		require.ErrorIs(t, err, dispatchErr)
		assert.False(t, dispatched)
	})
}

// The reminder push is dispatched synchronously because the caller holds a
// delivery claim it must release when the push was not accepted.
func TestDispatchGuardianAccountReminderDevices(t *testing.T) {
	ctx := context.Background()
	accounts := []int64{77}

	t.Run("without a synchronous notifier or an audience nothing is dispatched", func(t *testing.T) {
		s := &service{}
		dispatched, err := s.dispatchGuardianAccountReminderDevices(ctx, helperAppointment(), accounts)
		require.NoError(t, err)
		assert.False(t, dispatched)

		s = &service{cfg: Config{ReminderNotifier: &dispatchNotifier{}}}
		dispatched, err = s.dispatchGuardianAccountReminderDevices(ctx, helperAppointment(), nil)
		require.NoError(t, err)
		assert.False(t, dispatched)
	})

	t.Run("acceptance is reported back to the claim holder", func(t *testing.T) {
		notifier := &dispatchNotifier{}
		s := &service{cfg: Config{ReminderNotifier: notifier}}

		dispatched, err := s.dispatchGuardianAccountReminderDevices(ctx, helperAppointment(), accounts)
		require.NoError(t, err)
		assert.True(t, dispatched)
		require.Len(t, notifier.events, 1)
		assert.Equal(t, notifications.TypeParentAppointmentReminder, notifier.events[0].Type)
		assert.Equal(t, "Terminerinnerung", notifier.events[0].Title)
	})

	t.Run("a rejected push is not acceptance", func(t *testing.T) {
		dispatchErr := errors.New("push service unreachable")
		s := &service{cfg: Config{ReminderNotifier: &dispatchNotifier{err: dispatchErr}}}

		dispatched, err := s.dispatchGuardianAccountReminderDevices(ctx, helperAppointment(), accounts)
		require.ErrorIs(t, err, dispatchErr)
		assert.False(t, dispatched,
			"reporting a failed push as delivered would drop the reminder permanently")
	})
}

// The nil-safe logger keeps every notification path usable from a bare service
// value; a configured logger has to win.
func TestServiceLoggerFallback(t *testing.T) {
	assert.NotNil(t, (&service{}).logger())

	logger := slog.New(slog.DiscardHandler)
	assert.Same(t, logger, (&service{cfg: Config{Logger: logger}}).logger())
}
