package notifications

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pickupCaptureNotifier struct {
	events []Event
	err    error
}

func (c *pickupCaptureNotifier) Notify(_ context.Context, event Event) error {
	if c.err != nil {
		return c.err
	}
	c.events = append(c.events, event)
	return nil
}

type pickupRecipientStub struct {
	scopes   []StaffRecipientScope
	requests []StaffRecipientRequest
}

func (s *pickupRecipientStub) Resolve(_ context.Context, request StaffRecipientRequest) ([]StaffRecipientScope, error) {
	s.requests = append(s.requests, request)
	return s.scopes, nil
}

// A Thursday, fixed so the copy never depends on the wall clock.
var pickupToday = calendar.NewDate(2026, 11, 12)

func newPickupWorld(scopes ...StaffRecipientScope) (*absenceNotifier, *pickupCaptureNotifier, *pickupRecipientStub) {
	notifier := &pickupCaptureNotifier{}
	recipients := &pickupRecipientStub{scopes: scopes}
	producer := NewAbsenceNotifier(notifier, recipients, nil, nil).(*absenceNotifier)
	producer.today = func() calendar.Date { return pickupToday }
	return producer, notifier, recipients
}

func TestPickupChangeNotifierAnnouncesChangeToResponsibleStaff(t *testing.T) {
	t.Parallel()
	producer, notifier, recipients := newPickupWorld(
		StaffRecipientScope{AccountID: 11, StudentIDs: []int64{7}},
		StaffRecipientScope{AccountID: 12, StudentIDs: []int64{7}},
	)

	require.NoError(t, producer.NotifyPickupChanged(context.Background(), PickupChangeReport{
		TenantID: 3, StudentID: 7, Date: pickupToday, PickupTime: "14:30", ActorAccountID: 99, Revision: 1,
	}))

	require.Len(t, recipients.requests, 1)
	assert.Equal(t, StaffRecipientRequest{
		StudentIDs: []int64{7}, NotificationType: TypeStudentPickupChanged, ExcludedAccountIDs: []int64{99},
	}, recipients.requests[0], "the shared resolver scopes recipients to the child's responsible staff")
	require.Len(t, notifier.events, 1)
	event := notifier.events[0]
	assert.Equal(t, TypeStudentPickupChanged, event.Type)
	assert.Equal(t, Audience{TenantID: 3, Scope: ScopeStaff, StaffAccountIDs: []int64{11, 12}}, event.Audience)
	assert.Equal(t, "Abholzeit geändert", event.Title)
	assert.Equal(t, "Eltern haben die Abholzeit für ein Kind aus Ihrer Gruppe geändert. Neu für heute: 14:30 Uhr.", event.Body)
	assert.Equal(t, "/students/7", event.DeepLink, "tapping opens the child")
}

func TestPickupChangeNotifierNamesTheDay(t *testing.T) {
	t.Parallel()
	cases := []struct {
		date calendar.Date
		want string
	}{
		{pickupToday.AddDays(1), "Neu für morgen: 15:00 Uhr."},
		{calendar.NewDate(2026, 11, 20), "Neu für Fr. 20.11.: 15:00 Uhr."},
	}
	for _, tc := range cases {
		producer, notifier, _ := newPickupWorld(StaffRecipientScope{AccountID: 11, StudentIDs: []int64{7}})
		require.NoError(t, producer.NotifyPickupChanged(context.Background(), PickupChangeReport{
			TenantID: 3, StudentID: 7, Date: tc.date, PickupTime: "15:00",
		}))
		require.Len(t, notifier.events, 1)
		assert.Contains(t, notifier.events[0].Body, tc.want)
	}
}

func TestPickupChangeNotifierAnnouncesWithdrawal(t *testing.T) {
	t.Parallel()
	producer, notifier, _ := newPickupWorld(StaffRecipientScope{AccountID: 11, StudentIDs: []int64{7}})

	require.NoError(t, producer.NotifyPickupChanged(context.Background(), PickupChangeReport{
		TenantID: 3, StudentID: 7, Date: pickupToday.AddDays(1), Withdrawn: true, PickupTime: "14:30",
	}))

	require.Len(t, notifier.events, 1)
	assert.Equal(t, "Abholzeit zurückgenommen", notifier.events[0].Title)
	assert.Equal(t, "Die geänderte Abholzeit für ein Kind aus Ihrer Gruppe gilt nicht mehr. Für morgen gilt wieder die normale Abholzeit.", notifier.events[0].Body)
	assert.NotContains(t, notifier.events[0].Body, "14:30", "a withdrawn time is not shown as current")
}

// Repeated changes of the same day are separate notices: the key carries
// the revision, so delivery deduplication never swallows the latest one.
func TestPickupChangeNotifierKeysEveryRevision(t *testing.T) {
	t.Parallel()
	producer, notifier, _ := newPickupWorld(StaffRecipientScope{AccountID: 11, StudentIDs: []int64{7}})
	for revision, at := range []string{"14:30", "15:00", "14:30"} {
		require.NoError(t, producer.NotifyPickupChanged(context.Background(), PickupChangeReport{
			TenantID: 3, StudentID: 7, Date: pickupToday, PickupTime: at, Revision: int64(revision),
		}))
	}
	require.Len(t, notifier.events, 3)
	assert.NotEqual(t, notifier.events[0].IdempotencyKey, notifier.events[2].IdempotencyKey)
}

func TestPickupChangeNotifierSkipsWhatIsNoNews(t *testing.T) {
	t.Parallel()
	cases := map[string]PickupChangeReport{
		"past day":        {TenantID: 3, StudentID: 7, Date: pickupToday.AddDays(-1), PickupTime: "14:30"},
		"no time":         {TenantID: 3, StudentID: 7, Date: pickupToday},
		"no school":       {StudentID: 7, Date: pickupToday, PickupTime: "14:30"},
		"no child":        {TenantID: 3, Date: pickupToday, PickupTime: "14:30"},
		"no date":         {TenantID: 3, StudentID: 7, PickupTime: "14:30"},
		"nobody to reach": {TenantID: 3, StudentID: 8, Date: pickupToday, PickupTime: "14:30"},
	}
	for name, report := range cases {
		var scopes []StaffRecipientScope
		producer, notifier, _ := newPickupWorld(scopes...)
		require.NoError(t, producer.NotifyPickupChanged(context.Background(), report), name)
		assert.Empty(t, notifier.events, name)
	}
}

func TestPickupChangeNotifierQuietWhenSchoolSwitchedNoticesOff(t *testing.T) {
	t.Parallel()
	for _, quiet := range []error{ErrDisabled, ErrOutsideActiveWindow} {
		producer, notifier, _ := newPickupWorld(StaffRecipientScope{AccountID: 11, StudentIDs: []int64{7}})
		notifier.err = quiet
		require.NoError(t, producer.NotifyPickupChanged(context.Background(), PickupChangeReport{
			TenantID: 3, StudentID: 7, Date: pickupToday, PickupTime: "14:30",
		}))
	}
	producer, notifier, _ := newPickupWorld(StaffRecipientScope{AccountID: 11, StudentIDs: []int64{7}})
	notifier.err = errors.New("outbox down")
	require.Error(t, producer.NotifyPickupChanged(context.Background(), PickupChangeReport{
		TenantID: 3, StudentID: 7, Date: pickupToday, PickupTime: "14:30",
	}), "a failed enqueue is not reported as delivered")
}
