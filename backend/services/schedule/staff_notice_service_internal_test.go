package schedule

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
// Gültigkeitszeitraum ein, den Rest entscheidet dieser StaffNoticeService.

type fakeNoticeRepo struct {
	notices     []*usersModels.StaffNotice
	own         map[int64]time.Time
	counts      map[int64]int
	countsCalls int
	acked       []int64
	ackedBy     []int64
	ackRows     []*usersModels.StaffNoticeAck
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
func (f *fakeNoticeRepo) ListValidOn(context.Context, timezone.Date, string) ([]*usersModels.StaffNotice, error) {
	// Bewusst ohne Zielgruppenfilter: der Service muss die Leserart selbst
	// prüfen, siehe TestStaffNoticeTodayFiltersByReader.
	return f.notices, nil
}
func (f *fakeNoticeRepo) Acknowledge(_ context.Context, noticeID, accountID int64) error {
	f.acked = append(f.acked, noticeID)
	f.ackedBy = append(f.ackedBy, accountID)
	return nil
}
func (f *fakeNoticeRepo) AcknowledgedAtFor(context.Context, int64, []int64) (map[int64]time.Time, error) {
	if f.own == nil {
		return map[int64]time.Time{}, nil
	}
	return f.own, nil
}
func (f *fakeNoticeRepo) AcknowledgedCounts(context.Context, []int64) (map[int64]int, error) {
	f.countsCalls++
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

func noticeMustDate(t *testing.T, iso string) timezone.Date {
	t.Helper()
	d, err := timezone.ParseDate(iso)
	require.NoError(t, err)
	return d
}

func noticeDatePtr(date timezone.Date) *timezone.Date { return &date }

func newNotice(t *testing.T, id int64, weekdays []int16, weekPattern int) *usersModels.StaffNotice {
	t.Helper()
	n := &usersModels.StaffNotice{
		Title:       "Hinweis",
		Audience:    usersModels.StaffNoticeAudienceAll,
		Priority:    usersModels.StaffNoticePriorityInfo,
		ValidFrom:   noticeMustDate(t, "2026-08-01"),
		Weekdays:    weekdays,
		WeekPattern: weekPattern,
		Active:      true,
	}
	n.ID = id
	return n
}

func TestStaffNoticeTodayFiltersByWeekday(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tuesday := noticeMustDate(t, "2026-08-04")

	monTue := newNotice(t, 11, []int16{1, 2}, 0)
	fridayOnly := newNotice(t, 12, []int16{5}, 0)
	everyDay := newNotice(t, 13, nil, 0)

	repo := &fakeNoticeRepo{notices: []*usersModels.StaffNotice{monTue, fridayOnly, everyDay}}
	svc := NewStaffNoticeService(StaffNoticeServiceConfig{Repo: repo})

	views, err := svc.Today(ctx, 42, tuesday, usersModels.StaffNoticeAudienceStaff)
	require.NoError(t, err)

	got := make([]int64, 0, len(views))
	for _, v := range views {
		got = append(got, v.ID)
	}
	assert.Equal(t, []int64{11, 13}, got, "der Freitagshinweis darf am Dienstag nicht erscheinen")
}

func TestStaffNoticeTodaySkipsPeriodLookupWithoutWeekPattern(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	periods := &fakePeriodRepo{}
	repo := &fakeNoticeRepo{notices: []*usersModels.StaffNotice{newNotice(t, 21, nil, 0)}}
	svc := NewStaffNoticeService(StaffNoticeServiceConfig{Repo: repo, Periods: periods})

	_, err := svc.Today(ctx, 42, noticeMustDate(t, "2026-08-04"), usersModels.StaffNoticeAudienceStaff)
	require.NoError(t, err)
	assert.Zero(t, periods.calls, "ohne Wochenmuster darf die Startseite keine Zeiträume laden")
}

func TestStaffNoticeTodayHonoursWeekPattern(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// Anker Montag 2026-08-03 = Woche A; 2026-08-10 ist damit Woche B.
	anchor := scheduleModels.Date(noticeMustDate(t, "2026-08-03"))
	period := &scheduleModels.CalendarPeriod{
		Name:            "Schuljahr",
		PeriodType:      scheduleModels.PeriodTypeSchoolYear,
		StartDate:       scheduleModels.Date(noticeMustDate(t, "2026-08-01")),
		EndDate:         scheduleModels.Date(noticeMustDate(t, "2027-07-31")),
		WeekCycleLength: 2,
		WeekCycleAnchor: &anchor,
		IsActive:        true,
	}
	holidayAnchor := scheduleModels.Date(noticeMustDate(t, "2026-08-10"))
	holiday := &scheduleModels.CalendarPeriod{
		Name:            "Ferien",
		PeriodType:      scheduleModels.PeriodTypeHoliday,
		StartDate:       scheduleModels.Date(noticeMustDate(t, "2026-08-01")),
		EndDate:         scheduleModels.Date(noticeMustDate(t, "2026-08-31")),
		WeekCycleLength: 2,
		WeekCycleAnchor: &holidayAnchor,
		IsActive:        true,
	}
	periods := &fakePeriodRepo{periods: []*scheduleModels.CalendarPeriod{holiday, period}}

	weekA := newNotice(t, 31, nil, scheduleModels.WeekPatternA)
	weekB := newNotice(t, 32, nil, scheduleModels.WeekPatternB)
	repo := &fakeNoticeRepo{notices: []*usersModels.StaffNotice{weekA, weekB}}
	svc := NewStaffNoticeService(StaffNoticeServiceConfig{Repo: repo, Periods: periods})

	inWeekA, err := svc.Today(ctx, 42, noticeMustDate(t, "2026-08-05"), usersModels.StaffNoticeAudienceStaff)
	require.NoError(t, err)
	require.Len(t, inWeekA, 1)
	assert.Equal(t, int64(31), inWeekA[0].ID)

	inWeekB, err := svc.Today(ctx, 42, noticeMustDate(t, "2026-08-12"), usersModels.StaffNoticeAudienceStaff)
	require.NoError(t, err)
	require.Len(t, inWeekB, 1)
	assert.Equal(t, int64(32), inWeekB[0].ID)
}

func TestStaffNoticeTodayKeepsNoticeWithoutWeekCycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// Schule ohne A/B-Rhythmus: ein Hinweis mit Muster verschwindet nicht
	// stillschweigend, er gilt jede Woche (Richtung von
	// ShouldMaterializeWeekPattern).
	periods := &fakePeriodRepo{periods: []*scheduleModels.CalendarPeriod{}}
	repo := &fakeNoticeRepo{notices: []*usersModels.StaffNotice{
		newNotice(t, 41, nil, scheduleModels.WeekPatternB),
	}}
	svc := NewStaffNoticeService(StaffNoticeServiceConfig{Repo: repo, Periods: periods})

	views, err := svc.Today(ctx, 42, noticeMustDate(t, "2026-08-05"), usersModels.StaffNoticeAudienceStaff)
	require.NoError(t, err)
	assert.Len(t, views, 1)
}

func TestStaffNoticeTodayAttachesOwnAcknowledgement(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	stamp := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	repo := &fakeNoticeRepo{
		notices: []*usersModels.StaffNotice{newNotice(t, 51, nil, 0)},
		own:     map[int64]time.Time{51: stamp},
		counts:  map[int64]int{51: 4},
	}
	svc := NewStaffNoticeService(StaffNoticeServiceConfig{Repo: repo})

	views, err := svc.Today(ctx, 42, noticeMustDate(t, "2026-08-04"), usersModels.StaffNoticeAudienceStaff)
	require.NoError(t, err)
	require.Len(t, views, 1)
	require.NotNil(t, views[0].AcknowledgedAt)
	assert.Equal(t, stamp, *views[0].AcknowledgedAt)
	assert.Zero(t, views[0].AcknowledgedCount)
	assert.Zero(t, repo.countsCalls, "die Teamansicht darf keine Kenntnisnahmen anderer laden")
}

func TestStaffNoticeListAttachesAcknowledgementCounts(t *testing.T) {
	t.Parallel()
	repo := &fakeNoticeRepo{
		notices: []*usersModels.StaffNotice{newNotice(t, 52, nil, 0)},
		counts:  map[int64]int{52: 4},
	}
	svc := NewStaffNoticeService(StaffNoticeServiceConfig{Repo: repo})

	views, err := svc.List(context.Background(), 42, true)
	require.NoError(t, err)
	require.Len(t, views, 1)
	assert.Equal(t, 4, views[0].AcknowledgedCount)
	assert.Equal(t, 1, repo.countsCalls)
}

func TestStaffNoticeAcknowledgeRejectsNoticeThatDoesNotAskForIt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	notice := newNotice(t, 61, nil, 0)
	repo := &fakeNoticeRepo{notices: []*usersModels.StaffNotice{notice}}
	svc := NewStaffNoticeService(StaffNoticeServiceConfig{Repo: repo})

	err := svc.Acknowledge(ctx, 61, 42, usersModels.StaffNoticeAudienceStaff)
	assert.ErrorIs(t, err, ErrStaffNoticeInvalid)
	assert.Empty(t, repo.acked)

	notice.RequiresAcknowledgement = true
	require.NoError(t, svc.Acknowledge(ctx, 61, 42, usersModels.StaffNoticeAudienceStaff))
	assert.Equal(t, []int64{61}, repo.acked)
}

func TestStaffNoticeAcknowledgeUnknownNotice(t *testing.T) {
	t.Parallel()
	svc := NewStaffNoticeService(StaffNoticeServiceConfig{Repo: &fakeNoticeRepo{}})
	assert.ErrorIs(t, svc.Acknowledge(context.Background(), 999, 42, usersModels.StaffNoticeAudienceStaff), ErrStaffNoticeNotFound)
}

func TestStaffNoticeAcknowledgeRejectsNoticeThatDoesNotApplyToday(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	today := noticeMustDate(t, "2026-08-05")
	scheduleToday := scheduleModels.Date(today)
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
				Audience:                usersModels.StaffNoticeAudienceAll,
				ValidFrom:               today.AddDays(-1),
				RequiresAcknowledgement: true,
			},
		},
		{
			name: "future",
			notice: &usersModels.StaffNotice{
				Title:                   "Zukünftig",
				Audience:                usersModels.StaffNoticeAudienceAll,
				ValidFrom:               today.AddDays(1),
				RequiresAcknowledgement: true,
				Active:                  true,
			},
		},
		{
			name: "expired",
			notice: &usersModels.StaffNotice{
				Title:                   "Abgelaufen",
				Audience:                usersModels.StaffNoticeAudienceAll,
				ValidFrom:               today.AddDays(-2),
				ValidUntil:              noticeDatePtr(today.AddDays(-1)),
				RequiresAcknowledgement: true,
				Active:                  true,
			},
		},
		{
			name: "different weekday",
			notice: &usersModels.StaffNotice{
				Title:                   "Anderer Wochentag",
				Audience:                usersModels.StaffNoticeAudienceAll,
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
				Audience:                usersModels.StaffNoticeAudienceAll,
				ValidFrom:               today.AddDays(-1),
				WeekPattern:             scheduleModels.WeekPatternB,
				RequiresAcknowledgement: true,
				Active:                  true,
			},
			periods: &fakePeriodRepo{periods: []*scheduleModels.CalendarPeriod{{
				PeriodType:      scheduleModels.PeriodTypeSchoolYear,
				StartDate:       scheduleModels.Date(today.AddDays(-7)),
				EndDate:         scheduleModels.Date(today.AddDays(7)),
				WeekCycleLength: 2,
				WeekCycleAnchor: &scheduleToday,
				IsActive:        true,
			}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.notice.ID = 71
			repo := &fakeNoticeRepo{notices: []*usersModels.StaffNotice{tt.notice}}
			svc := NewStaffNoticeService(StaffNoticeServiceConfig{
				Repo:        repo,
				Periods:     tt.periods,
				CurrentDate: func() timezone.Date { return today },
			})

			err := svc.Acknowledge(ctx, tt.notice.ID, 42, usersModels.StaffNoticeAudienceStaff)
			assert.ErrorIs(t, err, ErrStaffNoticeInvalid)
			assert.Empty(t, repo.acked)
		})
	}
}

func TestStaffNoticeCreateRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &fakeNoticeRepo{}
	svc := NewStaffNoticeService(StaffNoticeServiceConfig{Repo: repo})

	_, err := svc.Create(ctx, 42, StaffNoticeInput{
		Title:     "   ",
		ValidFrom: noticeMustDate(t, "2026-08-01"),
		Active:    true,
	})
	assert.ErrorIs(t, err, ErrStaffNoticeInvalid)

	_, err = svc.Create(ctx, 42, StaffNoticeInput{Title: "Ohne Beginn", Active: true})
	assert.ErrorIs(t, err, ErrStaffNoticeInvalid, "ohne Startdatum gäbe es keinen Zeitraum")
}

func TestStaffNoticeCreateNormalizesWeekdays(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &fakeNoticeRepo{}
	svc := NewStaffNoticeService(StaffNoticeServiceConfig{Repo: repo})

	_, err := svc.Create(ctx, 42, StaffNoticeInput{
		Title:     "Turnhalle",
		ValidFrom: noticeMustDate(t, "2026-08-01"),
		Weekdays:  []int16{3, 1, 3},
		Active:    true,
	})
	require.NoError(t, err)
	require.NotNil(t, repo.createdWith)
	assert.Equal(t, []int16{1, 3}, repo.createdWith.Weekdays)
	assert.Equal(t, usersModels.StaffNoticePriorityInfo, repo.createdWith.Priority,
		"ohne Angabe ist ein Hinweis eine Information, keine Warnung")
}

func TestStaffNoticeCreateRejectsUnknownWeekday(t *testing.T) {
	t.Parallel()
	svc := NewStaffNoticeService(StaffNoticeServiceConfig{Repo: &fakeNoticeRepo{}})
	_, err := svc.Create(context.Background(), 42, StaffNoticeInput{
		Title:     "Kaputt",
		ValidFrom: noticeMustDate(t, "2026-08-01"),
		Weekdays:  []int16{9},
		Active:    true,
	})
	assert.ErrorIs(t, err, ErrStaffNoticeInvalid, "ein unbekannter Wochentag darf nicht stumm verschwinden")
}

// Wer den Hinweis schreibt, kennt ihn: sonst fragt die eigene
// Tagesinformation die Leitung nach einer Kenntnisnahme und zählt so lange im
// Badge mit.
func TestStaffNoticeCreateAcknowledgesForTheAuthor(t *testing.T) {
	t.Parallel()
	repo := &fakeNoticeRepo{}
	svc := NewStaffNoticeService(StaffNoticeServiceConfig{Repo: repo})

	_, err := svc.Create(context.Background(), 42, StaffNoticeInput{
		Title:                   "Räumungsübung",
		ValidFrom:               noticeMustDate(t, "2026-08-01"),
		RequiresAcknowledgement: true,
		Active:                  true,
	})
	require.NoError(t, err)
	assert.Equal(t, []int64{42}, repo.ackedBy)
}

// Ohne verlangte Kenntnisnahme gibt es nichts zu stempeln.
func TestStaffNoticeCreateWithoutAcknowledgementStampsNothing(t *testing.T) {
	t.Parallel()
	repo := &fakeNoticeRepo{}
	svc := NewStaffNoticeService(StaffNoticeServiceConfig{Repo: repo})

	_, err := svc.Create(context.Background(), 42, StaffNoticeInput{
		Title:     "Nur zur Information",
		ValidFrom: noticeMustDate(t, "2026-08-01"),
		Active:    true,
	})
	require.NoError(t, err)
	assert.Empty(t, repo.ackedBy)
}

// Wird die Kenntnisnahme erst nachträglich verlangt, gilt dasselbe.
func TestStaffNoticeUpdateAcknowledgesForTheAuthor(t *testing.T) {
	t.Parallel()
	notice := newNotice(t, 61, nil, 0)
	notice.CreatedBy = 7
	repo := &fakeNoticeRepo{notices: []*usersModels.StaffNotice{notice}}
	svc := NewStaffNoticeService(StaffNoticeServiceConfig{Repo: repo})

	_, err := svc.Update(context.Background(), 61, StaffNoticeInput{
		Title:                   "Hinweis",
		ValidFrom:               noticeMustDate(t, "2026-08-01"),
		RequiresAcknowledgement: true,
		Active:                  true,
	})
	require.NoError(t, err)
	assert.Equal(t, []int64{7}, repo.ackedBy)
}

// --- Zielgruppe und Bestätigungsliste (#2208) ---

func (f *fakeNoticeRepo) Acknowledgements(_ context.Context, noticeID int64) ([]*usersModels.StaffNoticeAck, error) {
	out := make([]*usersModels.StaffNoticeAck, 0, len(f.ackRows))
	for _, ack := range f.ackRows {
		if ack.NoticeID == noticeID {
			out = append(out, ack)
		}
	}
	return out, nil
}

type fakeNoticeNames struct {
	names map[int64]string
	calls int
	asked []int64
}

func (f *fakeNoticeNames) ListPersonNamesByAccount(_ context.Context, accountIDs []int64) (map[int64]string, error) {
	f.calls++
	f.asked = append(f.asked, accountIDs...)
	if f.names == nil {
		return map[int64]string{}, nil
	}
	return f.names, nil
}

func newNoticeFor(t *testing.T, id int64, audience string) *usersModels.StaffNotice {
	t.Helper()
	n := newNotice(t, id, nil, 0)
	n.Audience = audience
	return n
}

// Die Datenbank grenzt die Zielgruppe schon ein; der Service prüft sie
// trotzdem noch einmal, damit eine Attrappe ohne Filter (wie hier) nichts
// durchlässt, was das Portal nicht erreichen darf.
func TestStaffNoticeTodayFiltersByReader(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	day := noticeMustDate(t, "2026-08-04")
	repo := &fakeNoticeRepo{notices: []*usersModels.StaffNotice{
		newNoticeFor(t, 71, usersModels.StaffNoticeAudienceAll),
		newNoticeFor(t, 72, usersModels.StaffNoticeAudienceStaff),
		newNoticeFor(t, 73, usersModels.StaffNoticeAudienceLehrkraft),
	}}
	svc := NewStaffNoticeService(StaffNoticeServiceConfig{Repo: repo})

	ids := func(views []*usersModels.StaffNoticeView) []int64 {
		out := make([]int64, 0, len(views))
		for _, v := range views {
			out = append(out, v.ID)
		}
		return out
	}

	staffViews, err := svc.Today(ctx, 42, day, usersModels.StaffNoticeAudienceStaff)
	require.NoError(t, err)
	assert.Equal(t, []int64{71, 72}, ids(staffViews), "die Betreuung sieht 'alle' und 'nur Betreuung'")

	teacherViews, err := svc.Today(ctx, 42, day, usersModels.StaffNoticeAudienceLehrkraft)
	require.NoError(t, err)
	assert.Equal(t, []int64{71, 73}, ids(teacherViews), "eine Lehrkraft sieht 'alle' und 'nur Lehrkräfte'")

	none, err := svc.Today(ctx, 42, day, usersModels.StaffNoticeAudienceAll)
	require.NoError(t, err)
	assert.Empty(t, none, "'all' ist Zielgruppe, keine Leserart — fail-closed")
}

func TestStaffNoticeAcknowledgeRejectsForeignAudience(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	notice := newNoticeFor(t, 81, usersModels.StaffNoticeAudienceLehrkraft)
	notice.RequiresAcknowledgement = true
	repo := &fakeNoticeRepo{notices: []*usersModels.StaffNotice{notice}}
	svc := NewStaffNoticeService(StaffNoticeServiceConfig{
		Repo:        repo,
		CurrentDate: func() timezone.Date { return noticeMustDate(t, "2026-08-04") },
	})

	err := svc.Acknowledge(ctx, 81, 42, usersModels.StaffNoticeAudienceStaff)
	assert.ErrorIs(t, err, ErrStaffNoticeNotFound, "im OGS-Portal existiert ein Lehrkräfte-Hinweis nicht")
	assert.Empty(t, repo.acked)

	require.NoError(t, svc.Acknowledge(ctx, 81, 42, usersModels.StaffNoticeAudienceLehrkraft))
	assert.Equal(t, []int64{81}, repo.acked)
}

func TestStaffNoticeAcknowledgersResolvesNames(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	later := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	earlier := later.Add(-time.Hour)
	repo := &fakeNoticeRepo{
		notices: []*usersModels.StaffNotice{newNoticeFor(t, 91, usersModels.StaffNoticeAudienceAll)},
		ackRows: []*usersModels.StaffNoticeAck{
			{NoticeID: 91, AccountID: 7, AcknowledgedAt: later},
			{NoticeID: 91, AccountID: 8, AcknowledgedAt: earlier},
			{NoticeID: 92, AccountID: 9, AcknowledgedAt: earlier},
		},
	}
	names := &fakeNoticeNames{names: map[int64]string{7: "Anna Beispiel"}}
	svc := NewStaffNoticeService(StaffNoticeServiceConfig{Repo: repo, Names: names})

	rows, err := svc.Acknowledgers(ctx, 91)
	require.NoError(t, err)
	require.Len(t, rows, 2, "die Kenntnisnahme des anderen Hinweises zählt nicht mit")
	assert.Equal(t, int64(7), rows[0].AccountID)
	assert.Equal(t, "Anna Beispiel", rows[0].Name)
	assert.Equal(t, later, rows[0].AcknowledgedAt)
	assert.Equal(t, int64(8), rows[1].AccountID)
	assert.Equal(t, StaffNoticeUnknownAcknowledgerName, rows[1].Name,
		"ein Konto ohne Person bleibt in der Liste, mit Platzhalter statt Lücke")

	assert.Equal(t, 1, names.calls, "eine Namensabfrage für die ganze Liste, kein N+1")
	assert.ElementsMatch(t, []int64{7, 8}, names.asked)

	_, err = svc.Acknowledgers(ctx, 999)
	assert.ErrorIs(t, err, ErrStaffNoticeNotFound)
}

func TestStaffNoticeAcknowledgersWithoutNameLookup(t *testing.T) {
	t.Parallel()
	repo := &fakeNoticeRepo{
		notices: []*usersModels.StaffNotice{newNoticeFor(t, 93, usersModels.StaffNoticeAudienceAll)},
		ackRows: []*usersModels.StaffNoticeAck{{NoticeID: 93, AccountID: 7, AcknowledgedAt: time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)}},
	}
	svc := NewStaffNoticeService(StaffNoticeServiceConfig{Repo: repo})

	rows, err := svc.Acknowledgers(context.Background(), 93)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, StaffNoticeUnknownAcknowledgerName, rows[0].Name)
}

func TestStaffNoticeCreateAudience(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &fakeNoticeRepo{}
	svc := NewStaffNoticeService(StaffNoticeServiceConfig{Repo: repo})

	created, err := svc.Create(ctx, 42, StaffNoticeInput{
		Title: "Ohne Zielgruppe", ValidFrom: noticeMustDate(t, "2026-08-01"), Active: true,
	})
	require.NoError(t, err)
	assert.Equal(t, usersModels.StaffNoticeAudienceAll, created.Audience, "leer heißt 'alle', wie vor der Zielgruppe")

	created, err = svc.Create(ctx, 42, StaffNoticeInput{
		Title: "Nur Lehrkräfte", Audience: usersModels.StaffNoticeAudienceLehrkraft,
		ValidFrom: noticeMustDate(t, "2026-08-01"), Active: true,
	})
	require.NoError(t, err)
	assert.Equal(t, usersModels.StaffNoticeAudienceLehrkraft, created.Audience)

	_, err = svc.Create(ctx, 42, StaffNoticeInput{
		Title: "Falsche Zielgruppe", Audience: "eltern",
		ValidFrom: noticeMustDate(t, "2026-08-01"), Active: true,
	})
	assert.ErrorIs(t, err, ErrStaffNoticeInvalid)
}
