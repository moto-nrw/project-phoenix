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
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// In-memory identities. None of these are database rows.
const (
	absenceTenant    int64 = 5
	absenceStudentA  int64 = 71
	absenceStudentB  int64 = 72
	absenceGroupA    int64 = 81
	absenceStaffA    int64 = 91
	absenceAccountA  int64 = 101
	absenceAdmin     int64 = 102
	absenceActorAcct int64 = 103
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
}

func (f *fakeStaffAccountReader) ListAccountIDsByStaffIDs(_ context.Context, staffIDs []int64) (map[int64]int64, error) {
	if f.err != nil {
		return nil, f.err
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
		staff:  &fakeStaffAccountReader{accounts: map[int64]int64{absenceStaffA: absenceAccountA}},
		admins: &fakeAdminReader{ids: []int64{absenceAdmin}},
	}
	producer := notifications.NewAbsenceNotifier(
		w.notifier, w.consent, w.students, w.groups, w.staff, w.admins, nil, nil)
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
	producer, w := newAbsenceWorld()

	producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA))

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

func TestAbsenceNotifierWording(t *testing.T) {
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
			wantBody: "Eine Familie hat ein Kind aus Ihrer Gruppe für heute krankgemeldet.",
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
			producer.NotifyAbsenceReported(context.Background(), tc.report())

			require.Len(t, w.notifier.events, 1)
			assert.Equal(t, tc.wantBody, w.notifier.events[0].Body)
			assert.Equal(t, tc.wantLink, w.notifier.events[0].DeepLink)
		})
	}
}

func TestAbsenceNotifierSilentCases(t *testing.T) {
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
			producer.NotifyAbsenceReported(context.Background(), tc.report)
			assert.Empty(t, w.notifier.events, tc.why)
		})
	}
}

func TestAbsenceNotifierExcludesTheActor(t *testing.T) {
	producer, w := newAbsenceWorld()
	w.consent.allowed[absenceActorAcct] = struct{}{}
	w.staff.accounts[absenceStaffA] = absenceActorAcct

	report := sickToday(absenceStudentA)
	report.ActorAccountID = absenceActorAcct
	producer.NotifyAbsenceReported(context.Background(), report)

	require.Len(t, w.notifier.events, 1)
	assert.NotContains(t, w.notifier.events[0].Audience.StaffAccountIDs, absenceActorAcct,
		"nobody needs a push about their own keystroke")
}

func TestAbsenceNotifierRespectsConsent(t *testing.T) {
	t.Run("only those who agreed are addressed", func(t *testing.T) {
		producer, w := newAbsenceWorld()
		w.consent.allowed = map[int64]struct{}{absenceAdmin: {}}

		producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA))

		require.Len(t, w.notifier.events, 1)
		assert.Equal(t, []int64{absenceAdmin}, w.notifier.events[0].Audience.StaffAccountIDs)
		assert.ElementsMatch(t, []int64{absenceAccountA, absenceAdmin}, w.consent.asked,
			"consent narrows the relation-derived set, it never replaces it")
	})

	t.Run("nobody agreed means no event", func(t *testing.T) {
		producer, w := newAbsenceWorld()
		w.consent.allowed = nil

		producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA))
		assert.Empty(t, w.notifier.events)
	})

	t.Run("a child without a group still reaches the office", func(t *testing.T) {
		producer, w := newAbsenceWorld()
		w.students.byID[absenceStudentA] = &userModel.Student{}

		producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA))

		require.Len(t, w.notifier.events, 1)
		assert.Equal(t, []int64{absenceAdmin}, w.notifier.events[0].Audience.StaffAccountIDs)
	})
}

// Every failure is swallowed by contract: the producer runs from after-commit
// hooks, and a notification that cannot be built must never undo an absence
// that was already recorded.
func TestAbsenceNotifierSwallowsFailures(t *testing.T) {
	cases := map[string]func(*absenceWorld){
		"student read fails": func(w *absenceWorld) { w.students.err = errors.New("boom") },
		"group read fails":   func(w *absenceWorld) { w.groups.err = errors.New("boom") },
		"staff read fails":   func(w *absenceWorld) { w.staff.err = errors.New("boom") },
		"admin read fails":   func(w *absenceWorld) { w.admins.err = errors.New("boom") },
		"consent fails":      func(w *absenceWorld) { w.consent.err = errors.New("boom") },
		"dispatch fails":     func(w *absenceWorld) { w.notifier.err = errors.New("boom") },
		"notifications off":  func(w *absenceWorld) { w.notifier.err = notifications.ErrDisabled },
		"outside the window": func(w *absenceWorld) { w.notifier.err = notifications.ErrOutsideActiveWindow },
	}

	for name, break_ := range cases {
		t.Run(name, func(t *testing.T) {
			producer, w := newAbsenceWorld()
			break_(w)

			assert.NotPanics(t, func() {
				producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA))
			})
			assert.Empty(t, w.notifier.events)
		})
	}
}

// A partially constructed producer stays quiet rather than addressing a
// fallback audience.
func TestAbsenceNotifierWithoutDependencies(t *testing.T) {
	notifier := &captureNotifier{}
	producer := notifications.NewAbsenceNotifier(nil, nil, nil, nil, nil, nil, nil, nil)
	assert.NotPanics(t, func() {
		producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA))
	})
	assert.Empty(t, notifier.events)
}
