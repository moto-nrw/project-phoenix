package calendar

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
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
	t.Parallel()

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

// The student scope is what lets the delivery transaction ask the access
// question again. It must cover exactly the children the addressed accounts
// were let through by — anything more would ask about a child the audience was
// never based on, anything less would drop a legitimate recipient.
func TestGuardianStudentIDs(t *testing.T) {
	t.Parallel()

	addressed := int64(77)
	elsewhere := int64(88)

	recipients := guardianRecipients{
		guardianIDs: []int64{1, 2, 3, 4},
		profiles: map[int64]*userModels.GuardianProfile{
			1: guardianProfile(1, &addressed),
			2: guardianProfile(2, &addressed),
			3: guardianProfile(3, &elsewhere),
			4: guardianProfile(4, nil),
		},
		studentsByGuardian: map[int64][]int64{
			1: {10, 11},
			2: {11},
			3: {12},
			4: {13},
		},
	}

	assert.Equal(t, []int64{10, 11}, guardianStudentIDs(recipients, []int64{addressed}),
		"siblings of the addressed account count once each; a child of another recipient must not widen the scope")
	assert.Equal(t, []int64{10, 11, 12}, guardianStudentIDs(recipients, []int64{addressed, elsewhere}))
	assert.Empty(t, guardianStudentIDs(recipients, nil))
}

// The lifecycle path addresses many accounts at once, so the scope it hands to
// the delivery recheck must not be the appointment-wide union: a parent who
// lost access to their own invited child would otherwise ride through on a
// child of a different recipient they happen to be a guardian of.
func TestGuardianStudentGroups(t *testing.T) {
	t.Parallel()

	first := int64(77)
	second := int64(88)
	third := int64(99)

	recipients := guardianRecipients{
		guardianIDs: []int64{1, 2, 3, 4},
		profiles: map[int64]*userModels.GuardianProfile{
			1: guardianProfile(1, &first),
			2: guardianProfile(2, &second),
			3: guardianProfile(3, &third),
			4: guardianProfile(4, nil),
		},
		studentsByGuardian: map[int64][]int64{
			1: {10},
			2: {12},
			3: {12},
			4: {13},
		},
	}

	groups := guardianStudentGroups(recipients, []int64{first, second, third})

	require.Len(t, groups, 2, "accounts let through by the same child share one event")
	assert.Equal(t, []int64{first}, groups[0].accountIDs)
	assert.Equal(t, []int64{10}, groups[0].studentIDs,
		"the account invited for one child must not carry another recipient's child into the recheck")
	assert.Equal(t, []int64{second, third}, groups[1].accountIDs)
	assert.Equal(t, []int64{12}, groups[1].studentIDs)

	assert.Empty(t, guardianStudentGroups(recipients, nil))
	assert.Empty(t, guardianStudentGroups(recipients, []int64{int64(1234)}),
		"an account no child let through is not addressable at all")
}

// dispatchGuardianAccountDevices is the single gate every appointment push goes
// through. Consent narrows the audience the appointment already defined — it
// never widens it, and a missing dependency means "do not push".
func TestDispatchGuardianAccountDevices(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	accounts := []int64{77}
	students := []int64{4711}

	t.Run("without a notifier or an audience nothing is dispatched", func(t *testing.T) {
		s := &service{}
		dispatched, err := s.dispatchGuardianAccountDevicesLocalized(ctx, helperAppointment(), platformModels.EmailKindAppointmentPublished, accounts, students, "")
		require.NoError(t, err)
		assert.False(t, dispatched)

		s = &service{cfg: Config{Notifier: &dispatchNotifier{}, Preferences: &dispatchPreferences{}}}
		dispatched, err = s.dispatchGuardianAccountDevicesLocalized(ctx, helperAppointment(), platformModels.EmailKindAppointmentPublished, nil, students, "")
		require.NoError(t, err)
		assert.False(t, dispatched)
	})

	t.Run("an unwired consent service must not become consent-free delivery", func(t *testing.T) {
		notifier := &dispatchNotifier{}
		s := &service{cfg: Config{Notifier: notifier}}

		dispatched, err := s.dispatchGuardianAccountDevicesLocalized(ctx, helperAppointment(), platformModels.EmailKindAppointmentPublished, accounts, students, "")
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

		dispatched, err := s.dispatchGuardianAccountDevicesLocalized(ctx, helperAppointment(), platformModels.EmailKindAppointmentPublished, accounts, students, "")
		require.ErrorIs(t, err, consentErr)
		assert.False(t, dispatched)
	})

	t.Run("nobody opted in means nothing to deliver", func(t *testing.T) {
		notifier := &dispatchNotifier{}
		s := &service{cfg: Config{
			Notifier:    notifier,
			Preferences: &dispatchPreferences{optedIn: []int64{}},
		}}

		dispatched, err := s.dispatchGuardianAccountDevicesLocalized(ctx, helperAppointment(), platformModels.EmailKindAppointmentPublished, accounts, students, "")
		require.NoError(t, err)
		assert.False(t, dispatched)
		assert.Empty(t, notifier.events)
	})

	t.Run("a cancellation is urgent, a reminder is its own type", func(t *testing.T) {
		notifier := &dispatchNotifier{}
		prefs := &dispatchPreferences{}
		s := &service{cfg: Config{Notifier: notifier, Preferences: prefs}}

		dispatched, err := s.dispatchGuardianAccountDevicesLocalized(ctx, helperAppointment(), platformModels.EmailKindAppointmentCancelled, accounts, students, "")
		require.NoError(t, err)
		assert.True(t, dispatched)
		require.Len(t, notifier.events, 1)
		assert.Equal(t, notifications.PriorityHigh, notifier.events[0].Priority,
			"a cancelled appointment is the one a parent must not miss")
		assert.Equal(t, notifications.TypeParentAppointment, notifier.events[0].Type)
		assert.Equal(t, parentCalendarDeepLink, notifier.events[0].DeepLink)
		assert.Equal(t, accounts, notifier.events[0].Audience.GuardianAccountIDs)
		assert.Equal(t, students, notifier.events[0].Audience.StudentIDs,
			"the children the recipients were let through by must reach the delivery transaction, which rechecks their access")

		dispatched, err = s.dispatchGuardianAccountDevicesLocalized(ctx, helperAppointment(), platformModels.EmailKindAppointmentReminder, accounts, students, "")
		require.NoError(t, err)
		assert.True(t, dispatched)
		require.Len(t, notifier.events, 2)
		assert.Equal(t, notifications.TypeParentAppointmentReminder, notifier.events[1].Type)
		assert.Equal(t, notifications.PriorityNormal, notifier.events[1].Priority)
		assert.Equal(t, notifications.TypeParentAppointmentReminder, prefs.askedFor,
			"consent has to be read for the type actually being sent")
	})

	t.Run("uses the guardian portal locale", func(t *testing.T) {
		notifier := &dispatchNotifier{}
		s := &service{cfg: Config{Notifier: notifier, Preferences: &dispatchPreferences{}}}

		dispatched, err := s.dispatchGuardianAccountDevicesLocalized(ctx, helperAppointment(), platformModels.EmailKindAppointmentCancelled, accounts, students, "en")
		require.NoError(t, err)
		assert.True(t, dispatched)
		require.Len(t, notifier.events, 1)
		assert.Equal(t, "Appointment cancelled", notifier.events[0].Title)
	})

	t.Run("a failed dispatch is reported as not delivered", func(t *testing.T) {
		dispatchErr := errors.New("push service unreachable")
		s := &service{cfg: Config{
			Notifier:    &dispatchNotifier{err: dispatchErr},
			Preferences: &dispatchPreferences{},
		}}

		dispatched, err := s.dispatchGuardianAccountDevicesLocalized(ctx, helperAppointment(), platformModels.EmailKindAppointmentPublished, accounts, students, "")
		require.ErrorIs(t, err, dispatchErr)
		assert.False(t, dispatched)
	})
}

// The reminder push is dispatched synchronously because the caller holds a
// delivery claim it must release when the push was not accepted.
func TestDispatchGuardianAccountReminderDevices(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	accounts := []int64{77}
	students := []int64{4711}

	t.Run("without a synchronous notifier or an audience nothing is dispatched", func(t *testing.T) {
		s := &service{}
		dispatched, err := s.dispatchGuardianAccountReminderDevicesLocalized(ctx, helperAppointment(), accounts, students, "")
		require.NoError(t, err)
		assert.False(t, dispatched)

		s = &service{cfg: Config{ReminderNotifier: &dispatchNotifier{}}}
		dispatched, err = s.dispatchGuardianAccountReminderDevicesLocalized(ctx, helperAppointment(), nil, students, "")
		require.NoError(t, err)
		assert.False(t, dispatched)
	})

	t.Run("acceptance is reported back to the claim holder", func(t *testing.T) {
		notifier := &dispatchNotifier{}
		s := &service{cfg: Config{ReminderNotifier: notifier}}

		dispatched, err := s.dispatchGuardianAccountReminderDevicesLocalized(ctx, helperAppointment(), accounts, students, "")
		require.NoError(t, err)
		assert.True(t, dispatched)
		require.Len(t, notifier.events, 1)
		assert.Equal(t, notifications.TypeParentAppointmentReminder, notifier.events[0].Type)
		assert.Equal(t, "Terminerinnerung", notifier.events[0].Title)
		assert.Equal(t, students, notifier.events[0].Audience.StudentIDs,
			"the reminder is about the same children and carries the same delivery-time access recheck")
	})

	t.Run("a rejected push is not acceptance", func(t *testing.T) {
		dispatchErr := errors.New("push service unreachable")
		s := &service{cfg: Config{ReminderNotifier: &dispatchNotifier{err: dispatchErr}}}

		dispatched, err := s.dispatchGuardianAccountReminderDevicesLocalized(ctx, helperAppointment(), accounts, students, "")
		require.ErrorIs(t, err, dispatchErr)
		assert.False(t, dispatched,
			"reporting a failed push as delivered would drop the reminder permanently")
	})
}

// The nil-safe logger keeps every notification path usable from a bare service
// value; a configured logger has to win.
func TestServiceLoggerFallback(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, (&service{}).logger())

	logger := slog.New(slog.DiscardHandler)
	assert.Same(t, logger, (&service{cfg: Config{Logger: logger}}).logger())
}
