package staffnotice

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

// Reine Logiktests mit Attrappen: geprüft wird, WAS an einem Tag gilt und was
// nicht. Die Datenbank hat daran keinen Anteil — sie grenzt nur den
// Gültigkeitszeitraum ein, den Rest entscheidet dieser Service.

type fakeNoticeRepo struct {
	notices     []*usersModels.StaffNotice
	own         map[int64]time.Time
	counts      map[int64]int
	acked       []int64
	createdWith *usersModels.StaffNotice
}

func (f *fakeNoticeRepo) Create(_ context.Context, n *usersModels.StaffNotice) error {
	f.createdWith = n
	return nil
}
func (f *fakeNoticeRepo) Update(_ context.Context, n *usersModels.StaffNotice) error {
	f.createdWith = n
	return nil
}
func (f *fakeNoticeRepo) FindByID(_ context.Context, id int64) (*usersModels.StaffNotice, error) {
	for _, n := range f.notices {
		if n.ID == id {
			return n, nil
		}
	}
	return nil, nil
}
func (f *fakeNoticeRepo) Delete(context.Context, int64) error { return nil }
func (f *fakeNoticeRepo) List(context.Context, bool) ([]*usersModels.StaffNotice, error) {
	return f.notices, nil
}
func (f *fakeNoticeRepo) ListValidOn(context.Context, timezone.Date) ([]*usersModels.StaffNotice, error) {
	return f.notices, nil
}
func (f *fakeNoticeRepo) Acknowledge(_ context.Context, noticeID, _ int64) error {
	f.acked = append(f.acked, noticeID)
	return nil
}
func (f *fakeNoticeRepo) AcknowledgedAtFor(context.Context, int64, []int64) (map[int64]time.Time, error) {
	if f.own == nil {
		return map[int64]time.Time{}, nil
	}
	return f.own, nil
}
func (f *fakeNoticeRepo) AcknowledgedCounts(context.Context, []int64) (map[int64]int, error) {
	if f.counts == nil {
		return map[int64]int{}, nil
	}
	return f.counts, nil
}

type fakePeriodRepo struct {
	periods []*scheduleModels.CalendarPeriod
	calls   int
}

func (f *fakePeriodRepo) FindActiveByTenantID(context.Context) ([]*scheduleModels.CalendarPeriod, error) {
	f.calls++
	return f.periods, nil
}

func mustDate(t *testing.T, iso string) timezone.Date {
	t.Helper()
	d, err := timezone.ParseDate(iso)
	require.NoError(t, err)
	return d
}

func datePtr(date timezone.Date) *timezone.Date { return &date }

func newNotice(t *testing.T, id int64, weekdays []int16, weekPattern int) *usersModels.StaffNotice {
	t.Helper()
	n := &usersModels.StaffNotice{
		Title:       "Hinweis",
		Priority:    usersModels.StaffNoticePriorityInfo,
		ValidFrom:   mustDate(t, "2026-08-01"),
		Weekdays:    weekdays,
		WeekPattern: weekPattern,
		Active:      true,
	}
	n.ID = id
	return n
}

func TestTodayFiltersByWeekday(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tuesday := mustDate(t, "2026-08-04")

	monTue := newNotice(t, 11, []int16{1, 2}, 0)
	fridayOnly := newNotice(t, 12, []int16{5}, 0)
	everyDay := newNotice(t, 13, nil, 0)

	repo := &fakeNoticeRepo{notices: []*usersModels.StaffNotice{monTue, fridayOnly, everyDay}}
	svc := NewService(ServiceConfig{Repo: repo})

	views, err := svc.Today(ctx, 42, tuesday)
	require.NoError(t, err)

	got := make([]int64, 0, len(views))
	for _, v := range views {
		got = append(got, v.ID)
	}
	assert.Equal(t, []int64{11, 13}, got, "der Freitagshinweis darf am Dienstag nicht erscheinen")
}

func TestTodaySkipsPeriodLookupWithoutWeekPattern(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	periods := &fakePeriodRepo{}
	repo := &fakeNoticeRepo{notices: []*usersModels.StaffNotice{newNotice(t, 21, nil, 0)}}
	svc := NewService(ServiceConfig{Repo: repo, Periods: periods})

	_, err := svc.Today(ctx, 42, mustDate(t, "2026-08-04"))
	require.NoError(t, err)
	assert.Zero(t, periods.calls, "ohne Wochenmuster darf die Startseite keine Zeiträume laden")
}

func TestTodayHonoursWeekPattern(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// Anker Montag 2026-08-03 = Woche A; 2026-08-10 ist damit Woche B.
	anchor := mustDate(t, "2026-08-03")
	period := &scheduleModels.CalendarPeriod{
		Name:            "Schuljahr",
		StartDate:       mustDate(t, "2026-08-01"),
		EndDate:         mustDate(t, "2027-07-31"),
		WeekCycleLength: 2,
		WeekCycleAnchor: &anchor,
		IsActive:        true,
	}
	periods := &fakePeriodRepo{periods: []*scheduleModels.CalendarPeriod{period}}

	weekA := newNotice(t, 31, nil, scheduleModels.WeekPatternA)
	weekB := newNotice(t, 32, nil, scheduleModels.WeekPatternB)
	repo := &fakeNoticeRepo{notices: []*usersModels.StaffNotice{weekA, weekB}}
	svc := NewService(ServiceConfig{Repo: repo, Periods: periods})

	inWeekA, err := svc.Today(ctx, 42, mustDate(t, "2026-08-05"))
	require.NoError(t, err)
	require.Len(t, inWeekA, 1)
	assert.Equal(t, int64(31), inWeekA[0].ID)

	inWeekB, err := svc.Today(ctx, 42, mustDate(t, "2026-08-12"))
	require.NoError(t, err)
	require.Len(t, inWeekB, 1)
	assert.Equal(t, int64(32), inWeekB[0].ID)
}

func TestTodayKeepsNoticeWithoutWeekCycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// Schule ohne A/B-Rhythmus: ein Hinweis mit Muster verschwindet nicht
	// stillschweigend, er gilt jede Woche (Richtung von
	// ShouldMaterializeWeekPattern).
	periods := &fakePeriodRepo{periods: []*scheduleModels.CalendarPeriod{}}
	repo := &fakeNoticeRepo{notices: []*usersModels.StaffNotice{
		newNotice(t, 41, nil, scheduleModels.WeekPatternB),
	}}
	svc := NewService(ServiceConfig{Repo: repo, Periods: periods})

	views, err := svc.Today(ctx, 42, mustDate(t, "2026-08-05"))
	require.NoError(t, err)
	assert.Len(t, views, 1)
}

func TestTodayAttachesOwnAcknowledgement(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	stamp := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	repo := &fakeNoticeRepo{
		notices: []*usersModels.StaffNotice{newNotice(t, 51, nil, 0)},
		own:     map[int64]time.Time{51: stamp},
		counts:  map[int64]int{51: 4},
	}
	svc := NewService(ServiceConfig{Repo: repo})

	views, err := svc.Today(ctx, 42, mustDate(t, "2026-08-04"))
	require.NoError(t, err)
	require.Len(t, views, 1)
	require.NotNil(t, views[0].AcknowledgedAt)
	assert.Equal(t, stamp, *views[0].AcknowledgedAt)
	assert.Equal(t, 4, views[0].AcknowledgedCount)
}

func TestAcknowledgeRejectsNoticeThatDoesNotAskForIt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	notice := newNotice(t, 61, nil, 0)
	repo := &fakeNoticeRepo{notices: []*usersModels.StaffNotice{notice}}
	svc := NewService(ServiceConfig{Repo: repo})

	err := svc.Acknowledge(ctx, 61, 42)
	assert.ErrorIs(t, err, ErrInvalid)
	assert.Empty(t, repo.acked)

	notice.RequiresAcknowledgement = true
	require.NoError(t, svc.Acknowledge(ctx, 61, 42))
	assert.Equal(t, []int64{61}, repo.acked)
}

func TestAcknowledgeUnknownNotice(t *testing.T) {
	t.Parallel()
	svc := NewService(ServiceConfig{Repo: &fakeNoticeRepo{}})
	assert.ErrorIs(t, svc.Acknowledge(context.Background(), 999, 42), ErrNotFound)
}

func TestAcknowledgeRejectsNoticeThatDoesNotApplyToday(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	today := mustDate(t, "2026-08-05")
	otherWeekday := int16((int(today.Weekday())+6)%7 + 1)
	otherWeekday = otherWeekday%7 + 1

	tests := []struct {
		name    string
		notice  *usersModels.StaffNotice
		periods *fakePeriodRepo
	}{
		{
			name: "inactive",
			notice: &usersModels.StaffNotice{
				Title:                   "Abgeschaltet",
				ValidFrom:               today.AddDays(-1),
				RequiresAcknowledgement: true,
			},
		},
		{
			name: "future",
			notice: &usersModels.StaffNotice{
				Title:                   "Zukünftig",
				ValidFrom:               today.AddDays(1),
				RequiresAcknowledgement: true,
				Active:                  true,
			},
		},
		{
			name: "expired",
			notice: &usersModels.StaffNotice{
				Title:                   "Abgelaufen",
				ValidFrom:               today.AddDays(-2),
				ValidUntil:              datePtr(today.AddDays(-1)),
				RequiresAcknowledgement: true,
				Active:                  true,
			},
		},
		{
			name: "different weekday",
			notice: &usersModels.StaffNotice{
				Title:                   "Anderer Wochentag",
				ValidFrom:               today.AddDays(-1),
				Weekdays:                []int16{otherWeekday},
				RequiresAcknowledgement: true,
				Active:                  true,
			},
		},
		{
			name: "different week pattern",
			notice: &usersModels.StaffNotice{
				Title:                   "Woche B",
				ValidFrom:               today.AddDays(-1),
				WeekPattern:             scheduleModels.WeekPatternB,
				RequiresAcknowledgement: true,
				Active:                  true,
			},
			periods: &fakePeriodRepo{periods: []*scheduleModels.CalendarPeriod{{
				StartDate:       today.AddDays(-7),
				EndDate:         today.AddDays(7),
				WeekCycleLength: 2,
				WeekCycleAnchor: &today,
				IsActive:        true,
			}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.notice.ID = 71
			repo := &fakeNoticeRepo{notices: []*usersModels.StaffNotice{tt.notice}}
			svc := NewService(ServiceConfig{
				Repo:        repo,
				Periods:     tt.periods,
				CurrentDate: func() timezone.Date { return today },
			})

			err := svc.Acknowledge(ctx, tt.notice.ID, 42)
			assert.ErrorIs(t, err, ErrInvalid)
			assert.Empty(t, repo.acked)
		})
	}
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &fakeNoticeRepo{}
	svc := NewService(ServiceConfig{Repo: repo})

	_, err := svc.Create(ctx, 42, Input{
		Title:     "   ",
		ValidFrom: mustDate(t, "2026-08-01"),
		Active:    true,
	})
	assert.ErrorIs(t, err, ErrInvalid)

	_, err = svc.Create(ctx, 42, Input{Title: "Ohne Beginn", Active: true})
	assert.ErrorIs(t, err, ErrInvalid, "ohne Startdatum gäbe es keinen Zeitraum")
}

func TestCreateNormalizesWeekdays(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &fakeNoticeRepo{}
	svc := NewService(ServiceConfig{Repo: repo})

	_, err := svc.Create(ctx, 42, Input{
		Title:     "Turnhalle",
		ValidFrom: mustDate(t, "2026-08-01"),
		Weekdays:  []int16{3, 1, 3},
		Active:    true,
	})
	require.NoError(t, err)
	require.NotNil(t, repo.createdWith)
	assert.Equal(t, []int16{1, 3}, repo.createdWith.Weekdays)
	assert.Equal(t, usersModels.StaffNoticePriorityInfo, repo.createdWith.Priority,
		"ohne Angabe ist ein Hinweis eine Information, keine Warnung")
}

func TestCreateRejectsUnknownWeekday(t *testing.T) {
	t.Parallel()
	svc := NewService(ServiceConfig{Repo: &fakeNoticeRepo{}})
	_, err := svc.Create(context.Background(), 42, Input{
		Title:     "Kaputt",
		ValidFrom: mustDate(t, "2026-08-01"),
		Weekdays:  []int16{9},
		Active:    true,
	})
	assert.ErrorIs(t, err, ErrInvalid, "ein unbekannter Wochentag darf nicht stumm verschwinden")
}
