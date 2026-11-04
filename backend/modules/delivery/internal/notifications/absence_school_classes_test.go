// The moto schule notice of an absence (#4792) against fakes: subscribers
// first, then the class and its Lehrkräfte, the Schulverwaltung by
// permission, consent, and one event per child.
package notifications_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/notifications"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// In-memory identities of the school world. None of these are database rows.
const (
	schoolClass2a         = "2a"
	schoolClass3b         = "3b"
	schoolTeacher2a int64 = 201
	schoolTeacher3b int64 = 202
	schoolOffice    int64 = 203
)

// fakeSchoolClassReach names each child's class and the Lehrkräfte of each
// class.
type fakeSchoolClassReach struct {
	classes  map[int64]string
	teachers map[string][]int64
	err      error
	calls    int
}

func (f *fakeSchoolClassReach) StudentClassTeachers(_ context.Context, ids []int64) (map[int64]contract.StudentClassTeachers, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	out := make(map[int64]contract.StudentClassTeachers, len(ids))
	for _, id := range ids {
		if schoolClass, ok := f.classes[id]; ok {
			out[id] = contract.StudentClassTeachers{SchoolClass: schoolClass, AccountIDs: f.teachers[schoolClass]}
		}
	}
	return out, nil
}

// fakeSchoolSubscribers lists the accounts that switched the type on, read
// from the same consent stub the producer filters with.
type fakeSchoolSubscribers struct {
	consent *stubConsent
	err     error
}

func (f *fakeSchoolSubscribers) ListOptedIn(context.Context, string) ([]int64, error) {
	if f.err != nil {
		return nil, f.err
	}
	ids := make([]int64, 0, len(f.consent.allowed))
	for id := range f.consent.allowed {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids, nil
}

// fakeSchoolAccounts answers active membership and permission names.
type fakeSchoolAccounts struct {
	inactive    map[int64]bool
	permissions map[int64][]string
	err         error
}

func (f *fakeSchoolAccounts) ListActiveAccountIDsForTenant(_ context.Context, _ int64, ids []int64) ([]int64, error) {
	if f.err != nil {
		return nil, f.err
	}
	active := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !f.inactive[id] {
			active = append(active, id)
		}
	}
	return active, nil
}

func (f *fakeSchoolAccounts) FindEffectivePermissionNamesByAccountIDsForTenant(_ context.Context, ids []int64, _ int64) (map[int64][]string, error) {
	out := make(map[int64][]string, len(ids))
	for _, id := range ids {
		out[id] = f.permissions[id]
	}
	return out, nil
}

type schoolAbsenceWorld struct {
	notifier    *captureNotifier
	consent     *stubConsent
	reach       *fakeSchoolClassReach
	subscribers *fakeSchoolSubscribers
	accounts    *fakeSchoolAccounts
}

// newSchoolAbsenceWorld wires child A and B in class 2a, a teacher of 2a, a
// teacher of 3b and the office, which holds class_day:all_classes. Everyone
// agreed. The OGS side finds nobody: no child has a group and there is no
// admin, so every event is a school one.
func newSchoolAbsenceWorld() (notifications.AbsenceNotifier, *schoolAbsenceWorld) {
	consent := &stubConsent{allowed: map[int64]struct{}{
		schoolTeacher2a: {}, schoolTeacher3b: {}, schoolOffice: {},
	}}
	w := &schoolAbsenceWorld{
		notifier: &captureNotifier{},
		consent:  consent,
		reach: &fakeSchoolClassReach{
			classes: map[int64]string{absenceStudentA: schoolClass2a, absenceStudentB: schoolClass2a},
			teachers: map[string][]int64{
				schoolClass2a: {schoolTeacher2a},
				schoolClass3b: {schoolTeacher3b},
			},
		},
		subscribers: &fakeSchoolSubscribers{consent: consent},
		accounts: &fakeSchoolAccounts{permissions: map[int64][]string{
			schoolOffice: {permissions.ClassDayAllClasses},
		}},
	}
	recipients := notifications.NewStaffRecipientResolver(
		w.consent, &fakeStudentReader{}, &fakeGroupReader{}, &fakeStaffAccountReader{},
		&fakeAdminReader{}, &fakeOnDutySetting{}, &fakeDutyReader{})
	producer := notifications.NewAbsenceNotifier(w.notifier, recipients, nil, nil,
		notifications.WithSchoolClassRecipients(contract.SchoolClassRecipients{
			Reach: w.reach, Consent: w.consent, Subscribers: w.subscribers,
			Accounts: w.accounts, Permissions: securityruntime.HasPermission,
		}))
	return producer, w
}

func TestSchoolClassAbsenceReachesClassTeacherAndOffice(t *testing.T) {
	t.Parallel()

	producer, w := newSchoolAbsenceWorld()

	require.NoError(t, producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA)))

	require.Len(t, w.notifier.events, 1)
	event := w.notifier.events[0]
	assert.Equal(t, notifications.TypeSchoolClassAbsenceReported, event.Type)
	assert.Equal(t, notifications.ScopeStaff, event.Audience.Scope)
	assert.Equal(t, absenceTenant, event.Audience.TenantID)
	assert.Equal(t, []int64{schoolTeacher2a, schoolOffice}, event.Audience.StaffAccountIDs,
		"the teacher of the class and the Schulverwaltung, both of whom agreed")
	assert.Equal(t, "Krankmeldung", event.Title)
	assert.Equal(t, "Neue Krankmeldung in Klasse 2a", event.Body)
	assert.Equal(t, "/school/klasse?klasse=2a", event.SchoolDeepLink)
	assert.Empty(t, event.DeepLink, "the notice has no place in the OGS portal")
	assert.Equal(t, absenceStudentA, event.RelatedID)
}

func TestSchoolClassAbsenceSkipsTeachersOfOtherClasses(t *testing.T) {
	t.Parallel()

	producer, w := newSchoolAbsenceWorld()

	require.NoError(t, producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA)))

	require.Len(t, w.notifier.events, 1)
	assert.NotContains(t, w.notifier.events[0].Audience.StaffAccountIDs, schoolTeacher3b,
		"a teacher without an assignment to 2a hears nothing about a child of 2a")
	assert.NotContains(t, w.consent.asked, schoolTeacher3b,
		"the class relation decides the candidates before consent is asked")
}

func TestSchoolClassAbsenceReachesTheOfficeOnlyByPermission(t *testing.T) {
	t.Parallel()

	t.Run("without class_day:all_classes", func(t *testing.T) {
		producer, w := newSchoolAbsenceWorld()
		w.accounts.permissions = nil

		require.NoError(t, producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA)))

		require.Len(t, w.notifier.events, 1)
		assert.Equal(t, []int64{schoolTeacher2a}, w.notifier.events[0].Audience.StaffAccountIDs,
			"switching the type on alone reaches no class")
	})

	t.Run("an inactive account", func(t *testing.T) {
		producer, w := newSchoolAbsenceWorld()
		w.accounts.inactive = map[int64]bool{schoolOffice: true}

		require.NoError(t, producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA)))

		require.Len(t, w.notifier.events, 1)
		assert.Equal(t, []int64{schoolTeacher2a}, w.notifier.events[0].Audience.StaffAccountIDs)
	})
}

func TestSchoolClassAbsenceSendsOneNoticePerChild(t *testing.T) {
	t.Parallel()

	producer, w := newSchoolAbsenceWorld()

	require.NoError(t, producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA, absenceStudentB)))

	require.Len(t, w.notifier.events, 2, "one push per sick note, never a bundle")
	assert.Equal(t, absenceStudentA, w.notifier.events[0].RelatedID)
	assert.Equal(t, absenceStudentB, w.notifier.events[1].RelatedID)
	assert.NotEqual(t, w.notifier.events[0].IdempotencyKey, w.notifier.events[1].IdempotencyKey)
	for _, event := range w.notifier.events {
		assert.Equal(t, "Neue Krankmeldung in Klasse 2a", event.Body)
	}
	assert.Equal(t, 1, w.reach.calls, "one report resolves its classes once")
}

func TestSchoolClassAbsenceWording(t *testing.T) {
	t.Parallel()

	producer, w := newSchoolAbsenceWorld()
	report := sickToday(absenceStudentA)
	report.Status = notifications.AbsenceStatusExcused

	require.NoError(t, producer.NotifyAbsenceReported(context.Background(), report))

	require.Len(t, w.notifier.events, 1)
	assert.Equal(t, "Entschuldigung", w.notifier.events[0].Title)
	assert.Equal(t, "Neue Entschuldigung in Klasse 2a", w.notifier.events[0].Body)
}

func TestSchoolClassAbsenceNamesTheClassOnce(t *testing.T) {
	t.Parallel()

	producer, w := newSchoolAbsenceWorld()
	w.reach.classes[absenceStudentA] = "Klasse 1a"
	w.reach.teachers["Klasse 1a"] = []int64{schoolTeacher2a}

	require.NoError(t, producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA)))

	require.Len(t, w.notifier.events, 1)
	assert.Equal(t, "Neue Krankmeldung in Klasse 1a", w.notifier.events[0].Body,
		"a school that stores \"Klasse 1a\" must not read \"Klasse Klasse 1a\"")
	assert.Equal(t, "/school/klasse?klasse=Klasse+1a", w.notifier.events[0].SchoolDeepLink)
}

func TestSchoolClassAbsenceEncodesTheClassInTheLink(t *testing.T) {
	t.Parallel()

	producer, w := newSchoolAbsenceWorld()
	w.reach.classes[absenceStudentA] = "5 b/c"
	w.reach.teachers["5 b/c"] = []int64{schoolTeacher2a}

	require.NoError(t, producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA)))

	require.Len(t, w.notifier.events, 1)
	assert.Equal(t, "Neue Krankmeldung in Klasse 5 b/c", w.notifier.events[0].Body)
	assert.Equal(t, "/school/klasse?klasse=5+b%2Fc", w.notifier.events[0].SchoolDeepLink)
}

func TestSchoolClassAbsenceAsksNothingWhenNobodySubscribed(t *testing.T) {
	t.Parallel()

	producer, w := newSchoolAbsenceWorld()
	w.consent.allowed = nil

	require.NoError(t, producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA)))

	assert.Empty(t, w.notifier.events, "the type is off until a person switches it on")
	assert.Zero(t, w.reach.calls, "a school where nobody switched the type on pays no class lookup")
}

func TestSchoolClassAbsenceSilentCases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		mutate func(*schoolAbsenceWorld, *notifications.AbsenceReport)
		why    string
	}{
		{
			name: "a note for another day",
			mutate: func(_ *schoolAbsenceWorld, r *notifications.AbsenceReport) {
				r.Dates = []calendar.Date{calendar.TodayDate().AddDays(1)}
			},
			why: "a later day is no reason to interrupt anybody today",
		},
		{
			name: "a child without a class",
			mutate: func(w *schoolAbsenceWorld, _ *notifications.AbsenceReport) {
				delete(w.reach.classes, absenceStudentA)
			},
			why: "no class means no class teacher and no class view to open",
		},
		{
			name: "only the actor reaches the class",
			mutate: func(w *schoolAbsenceWorld, r *notifications.AbsenceReport) {
				w.reach.teachers[schoolClass2a] = nil
				r.ActorAccountID = schoolOffice
			},
			why: "nobody needs a push about their own keystroke",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			producer, w := newSchoolAbsenceWorld()
			report := sickToday(absenceStudentA)
			tc.mutate(w, &report)
			require.NoError(t, producer.NotifyAbsenceReported(context.Background(), report))
			assert.Empty(t, w.notifier.events, tc.why)
		})
	}
}

func TestSchoolClassAbsenceExcludesOriginators(t *testing.T) {
	t.Parallel()

	producer, w := newSchoolAbsenceWorld()
	report := sickToday(absenceStudentA)
	report.ActorAccountID = schoolTeacher2a

	require.NoError(t, producer.NotifyAbsenceReported(context.Background(), report))

	require.Len(t, w.notifier.events, 1)
	assert.Equal(t, []int64{schoolOffice}, w.notifier.events[0].Audience.StaffAccountIDs)
}

func TestSchoolClassAbsencePropagatesLookupFailures(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*schoolAbsenceWorld){
		"subscriber list": func(w *schoolAbsenceWorld) { w.subscribers.err = errors.New("consent unavailable") },
		"class lookup":    func(w *schoolAbsenceWorld) { w.reach.err = errors.New("people directory unavailable") },
		"account lookup":  func(w *schoolAbsenceWorld) { w.accounts.err = errors.New("identity unavailable") },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			producer, w := newSchoolAbsenceWorld()
			mutate(w)
			require.Error(t, producer.NotifyAbsenceReported(context.Background(), sickToday(absenceStudentA)))
			assert.Empty(t, w.notifier.events)
		})
	}
}
