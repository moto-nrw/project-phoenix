package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type noticeRuntime struct{ ports.Runtime }

func (noticeRuntime) TenantID(context.Context) int64 { return 5 }

func noticeConference(accountID int64) *schoolcalendar.ParentConference {
	return &schoolcalendar.ParentConference{ID: 3, ParentConferenceFields: schoolcalendar.ParentConferenceFields{
		OrganizerStaffID: 1, OrganizerAccountID: accountID, SchoolClass: "2a",
		Date: calendar.NewDate(2026, 11, 12), StartMinute: 14 * 60, EndMinute: 16 * 60, SlotMinutes: 15,
	}}
}

// The Lehrkraft hears about a booking in moto schule only with her consent,
// and the lock-screen text names class, day and time, never the child.
func TestNotifyParentConferenceOrganizer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	noDelivery := func(error) bool { return false }

	t.Run("consent decides", func(t *testing.T) {
		notifier := &dispatchNotifier{}
		preferences := &dispatchPreferences{optedIn: []int64{}}
		s := &service{cfg: Config{Notifier: notifier, Preferences: preferences, Runtime: noticeRuntime{}, NoDelivery: noDelivery}}
		require.NoError(t, s.notifyParentConferenceOrganizer(ctx, noticeConference(9), parentConferenceBooked, 14*60))
		assert.Equal(t, portal.NotificationTypeParentConferenceBooking, preferences.askedFor)
		assert.Empty(t, notifier.events)
	})

	t.Run("an agreed Lehrkraft gets the notice in moto schule", func(t *testing.T) {
		notifier := &dispatchNotifier{}
		s := &service{cfg: Config{Notifier: notifier, Preferences: &dispatchPreferences{}, Runtime: noticeRuntime{},
			NoDelivery: noDelivery, Now: func() time.Time { return time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC) }}}
		require.NoError(t, s.notifyParentConferenceOrganizer(ctx, noticeConference(9), parentConferenceCancelled, 14*60+15))
		require.Len(t, notifier.events, 1)
		event := notifier.events[0]
		assert.Equal(t, "staff", event.Audience.Scope)
		assert.Equal(t, []int64{9}, event.Audience.StaffAccountIDs)
		assert.Equal(t, int64(5), event.Audience.TenantID)
		assert.Equal(t, "Elternsprechtag Klasse 2a", event.Title)
		assert.Equal(t, "Eine Familie hat ihren Termin am 12.11. um 14:15 Uhr abgesagt.", event.Body)
		assert.Equal(t, "/school/elternsprechtag?sprechtag=3", event.SchoolDeepLink)
	})

	t.Run("without an account nobody is asked", func(t *testing.T) {
		notifier := &dispatchNotifier{}
		preferences := &dispatchPreferences{err: errors.New("must not be asked")}
		s := &service{cfg: Config{Notifier: notifier, Preferences: preferences, Runtime: noticeRuntime{}, NoDelivery: noDelivery}}
		require.NoError(t, s.notifyParentConferenceOrganizer(ctx, noticeConference(0), parentConferenceMoved, 14*60))
		assert.Empty(t, notifier.events)
	})
}
