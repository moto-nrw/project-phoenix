// Absence producer tests against fakes. The producer is pure orchestration —
// relation, then consent, then one event — so every branch is reachable without
// a database.
package notifications_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// In-memory identities. None of these are database rows.
const (
	absenceTenant     int64 = 5
	absenceStudentA   int64 = 71
	absenceStudentB   int64 = 72
	absenceGroupA     int64 = 81
	absenceGroupB     int64 = 82
	absenceStaffA     int64 = 91
	absenceStaffB     int64 = 92
	absenceAdminStaff int64 = 93
	absenceAccountA   int64 = 101
	absenceAccountB   int64 = 104
	absenceAdmin      int64 = 102
	absenceActorAcct  int64 = 103
)

type fakeStudentReader struct {
	userModel.StudentRepository
	byID map[int64]*userModel.Student
	err  error
}

func (f *fakeStudentReader) FindReadScopeByIDs(_ context.Context, ids []int64) (map[int64]*userModel.Student, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make(map[int64]*userModel.Student, len(ids))
	for _, id := range ids {
		if s, ok := f.byID[id]; ok {
			out[id] = s
		}
	}
	return out, nil
}

type fakeGroupReader struct {
	educationModel.GroupRepository
	staffByGroup map[int64][]int64
	err          error
}

func (f *fakeGroupReader) ListStaffIDsByEducationGroupIDs(_ context.Context, groupIDs []int64, _ timezone.Date) ([]educationModel.StaffGroupID, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []educationModel.StaffGroupID
	for _, groupID := range groupIDs {
		for _, staffID := range f.staffByGroup[groupID] {
			out = append(out, educationModel.StaffGroupID{StaffID: staffID, GroupID: groupID})
		}
	}
	return out, nil
}

type fakeStaffAccountReader struct {
	userModel.StaffRepository
	accounts map[int64]int64
	err      error
	dutyErr  error
	calls    int
}

func (f *fakeStaffAccountReader) ListAccountIDsByStaffIDs(_ context.Context, staffIDs []int64) (map[int64]int64, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if f.dutyErr != nil && f.calls > 1 {
		return nil, f.dutyErr
	}
	out := make(map[int64]int64, len(staffIDs))
	for _, staffID := range staffIDs {
		if accountID, ok := f.accounts[staffID]; ok {
			out[staffID] = accountID
		}
	}
	return out, nil
}

type fakeAdminReader struct {
	ids []int64
	err error
}

func (f *fakeAdminReader) ListEffectiveAdminAccountIDs(context.Context) ([]int64, error) {
	return f.ids, f.err
}

type fakeOnDutySetting struct {
	enabled bool
	err     error
	asked   string
}

func (f *fakeOnDutySetting) ResolveBool(_ context.Context, key string) (bool, error) {
	f.asked = key
	return f.enabled, f.err
}

type fakeDutyReader struct {
	presence map[int64]string
	err      error
	called   bool
}

func (f *fakeDutyReader) GetTodayPresenceMap(context.Context) (map[int64]string, error) {
	f.called = true
	return f.presence, f.err
}

// captureNotifier records what the producer decided to send.
type captureNotifier struct {
	events []notifications.Event
	err    error
}

func (c *captureNotifier) Notify(_ context.Context, event notifications.Event) error {
	if c.err != nil {
		return c.err
	}
	c.events = append(c.events, event)
	return nil
}

// stubConsent answers the consent question without a repository.
type stubConsent struct {
	notifications.PreferenceService
	allowed map[int64]struct{}
	err     error
	asked   []int64
}

func (s *stubConsent) FilterOptedIn(_ context.Context, _ string, accountIDs []int64) ([]int64, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.asked = append(s.asked, accountIDs...)
	out := make([]int64, 0, len(accountIDs))
	for _, id := range accountIDs {
		if _, ok := s.allowed[id]; ok {
			out = append(out, id)
		}
	}
	return out, nil
}

type absenceWorld struct {
	notifier *captureNotifier
	consent  *stubConsent
	students *fakeStudentReader
	groups   *fakeGroupReader
	staff    *fakeStaffAccountReader
	admins   *fakeAdminReader
	settings *fakeOnDutySetting
	duty     *fakeDutyReader
}

// newAbsenceWorld wires one child in one group, supervised by one person, plus
// one admin. Both the supervisor and the admin have agreed.
func newAbsenceWorld() (notifications.AbsenceNotifier, *absenceWorld) {
	groupID := absenceGroupA
	w := &absenceWorld{
		notifier: &captureNotifier{},
		consent:  &stubConsent{allowed: map[int64]struct{}{absenceAccountA: {}, absenceAdmin: {}}},
		students: &fakeStudentReader{byID: map[int64]*userModel.Student{
			absenceStudentA: {GroupID: &groupID},
			absenceStudentB: {GroupID: &groupID},
		}},
		groups: &fakeGroupReader{staffByGroup: map[int64][]int64{absenceGroupA: {absenceStaffA}}},
		staff: &fakeStaffAccountReader{accounts: map[int64]int64{
			absenceStaffA:     absenceAccountA,
			absenceAdminStaff: absenceAdmin,
		}},
		admins:   &fakeAdminReader{ids: []int64{absenceAdmin}},
		settings: &fakeOnDutySetting{enabled: true},
		duty: &fakeDutyReader{presence: map[int64]string{
			absenceStaffA:     activeModel.WorkSessionStatusPresent,
			absenceAdminStaff: activeModel.WorkSessionStatusPresent,
		}},
	}
	recipients := notifications.NewStaffRecipientResolver(
		w.consent, w.students, w.groups, w.staff, w.admins, w.settings, w.duty)
	producer := notifications.NewAbsenceNotifier(w.notifier, recipients, nil, nil)
	return producer, w
}

func sickToday(studentIDs ...int64) notifications.AbsenceReport {
	return notifications.AbsenceReport{
		TenantID:   absenceTenant,
		StudentIDs: studentIDs,
		Status:     activeModel.StudentStatusDaySick,
		Dates:      []timezone.Date{timezone.TodayDate()},
		FromParent: true,
	}
}

func TestAbsenceNotifierReachesGroupAndOffice(t *testing.T) {
	t.Parallel()

	producer, w := newAbsenceWorld()

	require.NoError(t, producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA)))

	require.Len(t, w.notifier.events, 1)
	event := w.notifier.events[0]
	assert.Equal(t, notifications.TypeStudentAbsenceReported, event.Type)
	assert.Equal(t, notifications.ScopeStaff, event.Audience.Scope)
	assert.Equal(t, absenceTenant, event.Audience.TenantID)
	assert.ElementsMatch(t, []int64{absenceAccountA, absenceAdmin}, event.Audience.StaffAccountIDs,
		"the supervising person and the office, both of whom agreed")
	assert.Equal(t, "Krankmeldung", event.Title)
	assert.Contains(t, event.Body, "Eine Familie")
	assert.Equal(t, "/students/71", event.DeepLink)
	assert.NotContains(t, event.Body, "Felix", "no child name leaves the app")
}

func TestAbsenceNotifierPartitionsCountsByRecipientScope(t *testing.T) {
	t.Parallel()

	producer, w := newAbsenceWorld()
	groupB := absenceGroupB
	w.students.byID[absenceStudentB] = &userModel.Student{GroupID: &groupB}
	w.groups.staffByGroup[absenceGroupB] = []int64{absenceStaffB}
	w.staff.accounts[absenceStaffB] = absenceAccountB
	w.consent.allowed[absenceAccountB] = struct{}{}
	w.duty.presence[absenceStaffB] = activeModel.WorkSessionStatusPresent

	require.NoError(t, producer.NotifyAbsenceReported(
		context.Background(),
		sickToday(absenceStudentA, absenceStudentB),
	))

	require.Len(t, w.notifier.events, 3)
	byRecipient := make(map[int64]notifications.Event)
	for _, event := range w.notifier.events {
		require.Len(t, event.Audience.StaffAccountIDs, 1)
		byRecipient[event.Audience.StaffAccountIDs[0]] = event
	}

	assert.Equal(t, "Eine Familie hat ein Kind aus Ihrer Gruppe für heute krankgemeldet.", byRecipient[absenceAccountA].Body)
	assert.Equal(t, "/students/71", byRecipient[absenceAccountA].DeepLink)
	assert.Equal(t, "Eine Familie hat ein Kind aus Ihrer Gruppe für heute krankgemeldet.", byRecipient[absenceAccountB].Body)
	assert.Equal(t, "/students/72", byRecipient[absenceAccountB].DeepLink)
	assert.Equal(t, "2 Kinder wurden für heute krankgemeldet.", byRecipient[absenceAdmin].Body)
	assert.Equal(t, "/students", byRecipient[absenceAdmin].DeepLink)
}

func TestAbsenceNotifierWording(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		report   func() notifications.AbsenceReport
		wantBody string
		wantLink string
	}{
		{
			name: "reported by staff",
			report: func() notifications.AbsenceReport {
				r := sickToday(absenceStudentA)
				r.FromParent = false
				return r
			},
			wantBody: "Für ein Kind aus Ihrer Gruppe wurde heute eine Krankmeldung eingetragen.",
			wantLink: "/students/71",
		},
		{
			name: "an excuse reads differently",
			report: func() notifications.AbsenceReport {
				r := sickToday(absenceStudentA)
				r.Status = activeModel.StudentStatusDayExcused
				return r
			},
			wantBody: "Für ein Kind aus Ihrer Gruppe wurde heute eine Entschuldigung eingetragen.",
			wantLink: "/students/71",
		},
		{
			name: "two children point at the list",
			report: func() notifications.AbsenceReport {
				return sickToday(absenceStudentA, absenceStudentB)
			},
			wantBody: "2 Kinder wurden für heute krankgemeldet.",
			wantLink: "/students",
		},
		{
			name: "three excused children use plural wording",
			report: func() notifications.AbsenceReport {
				r := sickToday(71, 72, 73)
				r.Status = activeModel.StudentStatusDayExcused
				return r
			},
			wantBody: "3 Kinder wurden für heute entschuldigt.",
			wantLink: "/students",
		},
		{
			name: "many children collapse into a count without a link",
			report: func() notifications.AbsenceReport {
				return sickToday(71, 72, 73, 74)
			},
			wantBody: "4 Kinder wurden für heute krankgemeldet.",
			wantLink: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			producer, w := newAbsenceWorld()
			require.NoError(t, producer.NotifyAbsenceReported(context.Background(), tc.report()))

			event := eventForAbsenceRecipient(w.notifier.events, absenceAdmin)
			require.NotNil(t, event)
			assert.Equal(t, tc.wantBody, event.Body)
			assert.Equal(t, tc.wantLink, event.DeepLink)
		})
	}
}

func eventForAbsenceRecipient(events []notifications.Event, accountID int64) *notifications.Event {
	for i := range events {
		for _, recipientID := range events[i].Audience.StaffAccountIDs {
			if recipientID == accountID {
				return &events[i]
			}
		}
	}
	return nil
}

func TestAbsenceNotifierSilentCases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		report notifications.AbsenceReport
		why    string
	}{
		{
			name: "a class trip is not news",
			report: func() notifications.AbsenceReport {
				r := sickToday(absenceStudentA)
				r.Status = activeModel.StudentStatusDayClassTrip
				return r
			}(),
			why: "only sick and excused are reported",
		},
		{
			name: "a note for another day is planning data",
			report: func() notifications.AbsenceReport {
				r := sickToday(absenceStudentA)
				r.Dates = []timezone.Date{timezone.TodayDate().AddDays(3)}
				return r
			}(),
			why: "a future note must not interrupt anybody today",
		},
		{
			name:   "no children",
			report: sickToday(),
			why:    "nothing was reported",
		},
		{
			name: "no tenant",
			report: func() notifications.AbsenceReport {
				r := sickToday(absenceStudentA)
				r.TenantID = 0
				return r
			}(),
			why: "an event without a school cannot be delivered",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			producer, w := newAbsenceWorld()
			require.NoError(t, producer.NotifyAbsenceReported(context.Background(), tc.report))
			assert.Empty(t, w.notifier.events, tc.why)
		})
	}
}

func TestAbsenceNotifierExcludesTheActor(t *testing.T) {
	t.Parallel()

	producer, w := newAbsenceWorld()
	w.consent.allowed[absenceActorAcct] = struct{}{}
	w.staff.accounts[absenceStaffA] = absenceActorAcct

	report := sickToday(absenceStudentA)
	report.ActorAccountID = absenceActorAcct
	require.NoError(t, producer.NotifyAbsenceReported(context.Background(), report))

	require.Len(t, w.notifier.events, 1)
	assert.NotContains(t, w.notifier.events[0].Audience.StaffAccountIDs, absenceActorAcct,
		"nobody needs a push about their own keystroke")
}

func TestAbsenceNotifierExcludesAdditionalOriginators(t *testing.T) {
	t.Parallel()

	producer, w := newAbsenceWorld()

	report := sickToday(absenceStudentA)
	report.ExcludedAccountIDs = []int64{absenceAccountA}
	require.NoError(t, producer.NotifyAbsenceReported(context.Background(), report))

	require.Len(t, w.notifier.events, 1)
	assert.Equal(t, []int64{absenceAdmin}, w.notifier.events[0].Audience.StaffAccountIDs,
		"a guardian with a staff role must not receive their own approved report")
}

func TestAbsenceNotifierRespectsConsent(t *testing.T) {
	t.Parallel()

	t.Run("only those who agreed are addressed", func(t *testing.T) {
		producer, w := newAbsenceWorld()
		w.consent.allowed = map[int64]struct{}{absenceAdmin: {}}

		require.NoError(t, producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA)))

		require.Len(t, w.notifier.events, 1)
		assert.Equal(t, []int64{absenceAdmin}, w.notifier.events[0].Audience.StaffAccountIDs)
		assert.ElementsMatch(t, []int64{absenceAccountA, absenceAdmin}, w.consent.asked,
			"consent narrows the relation-derived set, it never replaces it")
	})

	t.Run("nobody agreed means no event", func(t *testing.T) {
		producer, w := newAbsenceWorld()
		w.consent.allowed = nil

		require.NoError(t, producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA)))
		assert.Empty(t, w.notifier.events)
	})

	t.Run("a child without a group still reaches the office", func(t *testing.T) {
		producer, w := newAbsenceWorld()
		w.students.byID[absenceStudentA] = &userModel.Student{}

		require.NoError(t, producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA)))

		require.Len(t, w.notifier.events, 1)
		assert.Equal(t, []int64{absenceAdmin}, w.notifier.events[0].Audience.StaffAccountIDs)
	})
}

func TestAbsenceNotifierRespectsOnDutySetting(t *testing.T) {
	t.Parallel()

	t.Run("only checked-in staff are addressed", func(t *testing.T) {
		producer, w := newAbsenceWorld()
		w.duty.presence[absenceStaffA] = "checked_out"

		require.NoError(t, producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA)))

		require.Len(t, w.notifier.events, 1)
		assert.Equal(t, []int64{absenceAdmin}, w.notifier.events[0].Audience.StaffAccountIDs)
		assert.Equal(t, configModel.KeyNotificationsOnDutyOnly, w.settings.asked)
	})

	t.Run("home office counts as on duty", func(t *testing.T) {
		producer, w := newAbsenceWorld()
		w.duty.presence[absenceStaffA] = activeModel.WorkSessionStatusHomeOffice
		w.duty.presence[absenceAdminStaff] = "checked_out"

		require.NoError(t, producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA)))

		require.Len(t, w.notifier.events, 1)
		assert.Equal(t, []int64{absenceAccountA}, w.notifier.events[0].Audience.StaffAccountIDs)
	})

	t.Run("nobody checked in fails closed", func(t *testing.T) {
		producer, w := newAbsenceWorld()
		w.duty.presence = nil

		require.NoError(t, producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA)))

		assert.Empty(t, w.notifier.events)
	})

	t.Run("staffless admins are excluded", func(t *testing.T) {
		producer, w := newAbsenceWorld()
		delete(w.staff.accounts, absenceAdminStaff)

		require.NoError(t, producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA)))

		require.Len(t, w.notifier.events, 1)
		assert.Equal(t, []int64{absenceAccountA}, w.notifier.events[0].Audience.StaffAccountIDs)
	})

	t.Run("disabled setting leaves consent as the final filter", func(t *testing.T) {
		producer, w := newAbsenceWorld()
		w.settings.enabled = false
		w.duty.err = errors.New("must not be read")

		require.NoError(t, producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA)))

		require.Len(t, w.notifier.events, 1)
		assert.ElementsMatch(t, []int64{absenceAccountA, absenceAdmin}, w.notifier.events[0].Audience.StaffAccountIDs)
		assert.False(t, w.duty.called)
	})
}

// Durable producer failures are returned so the surrounding domain transaction
// can roll back instead of committing without its delivery intent. Tenant
// gates remain intentional no-ops.
func TestAbsenceNotifierPropagatesDurableDeliveryFailures(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		breakWorld func(*absenceWorld)
		wantError  bool
	}{
		"student read fails":  {func(w *absenceWorld) { w.students.err = errors.New("boom") }, true},
		"group read fails":    {func(w *absenceWorld) { w.groups.err = errors.New("boom") }, true},
		"staff read fails":    {func(w *absenceWorld) { w.staff.err = errors.New("boom") }, true},
		"admin read fails":    {func(w *absenceWorld) { w.admins.err = errors.New("boom") }, true},
		"consent fails":       {func(w *absenceWorld) { w.consent.err = errors.New("boom") }, true},
		"setting read fails":  {func(w *absenceWorld) { w.settings.err = errors.New("boom") }, true},
		"presence read fails": {func(w *absenceWorld) { w.duty.err = errors.New("boom") }, true},
		"duty mapping fails":  {func(w *absenceWorld) { w.staff.dutyErr = errors.New("boom") }, true},
		"dispatch fails":      {func(w *absenceWorld) { w.notifier.err = errors.New("boom") }, true},
		"notifications off":   {func(w *absenceWorld) { w.notifier.err = notifications.ErrDisabled }, false},
		"outside the window":  {func(w *absenceWorld) { w.notifier.err = notifications.ErrOutsideActiveWindow }, false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			producer, w := newAbsenceWorld()
			tc.breakWorld(w)

			err := producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA))
			if tc.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Empty(t, w.notifier.events)
		})
	}
}

// A partially constructed producer stays quiet rather than addressing a
// fallback audience.
func TestAbsenceNotifierWithoutDependencies(t *testing.T) {
	t.Parallel()

	notifier := &captureNotifier{}
	producer := notifications.NewAbsenceNotifier(nil, nil, nil, nil)
	assert.NotPanics(t, func() {
		require.NoError(t, producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA)))
	})
	assert.Empty(t, notifier.events)
}
