package compose

import (
	"testing"

	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tagesinformationen (#2180). Geprüft wird, was nur die Datenbank beantwortet:
// die Vorauswahl nach Zeitraum und die Reihenfolge. Wochentag und Wochenmuster
// liegen im Service und sind dort getestet.

func TestStaffNoticeRepository_ListValidOn(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repo := NewStaffNoticeRepository(db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "notice-author@test.local")

	day := func(iso string) calendar.Date {
		d, err := calendar.ParseDate(iso)
		require.NoError(t, err)
		return d
	}

	ended := day("2026-08-04")
	inactive := newTestStaffNotice(t, "Abgeschaltet", day("2026-08-01"), account.ID, staffNoticeOpts{Inactive: true})
	info := newTestStaffNotice(t, "Laufender Hinweis", day("2026-08-01"), account.ID, staffNoticeOpts{})
	important := newTestStaffNotice(t, "Wichtiger Hinweis", day("2026-08-01"), account.ID, staffNoticeOpts{Important: true})
	expired := newTestStaffNotice(t, "Abgelaufen", day("2026-08-01"), account.ID, staffNoticeOpts{ValidUntil: &ended})
	future := newTestStaffNotice(t, "Beginnt später", day("2026-09-01"), account.ID, staffNoticeOpts{})
	inactive.Audience = "all"
	info.Audience = "all"
	important.Audience = "all"
	expired.Audience = "all"
	future.Audience = "all"
	require.NoError(t, repo.Create(ctx, inactive))
	require.NoError(t, repo.Create(ctx, info))
	require.NoError(t, repo.Create(ctx, important))
	require.NoError(t, repo.Create(ctx, expired))
	require.NoError(t, repo.Create(ctx, future))

	rows, err := repo.ListValidOn(ctx, day("2026-08-06"), "staff") // Leserart "staff" = OGS-Portal; Behavior-Test ohne Modell-Import
	require.NoError(t, err)

	titles := make([]string, 0, len(rows))
	for _, row := range rows {
		titles = append(titles, row.Title)
	}

	assert.Contains(t, titles, info.Title)
	assert.Contains(t, titles, important.Title)
	assert.NotContains(t, titles, expired.Title, "ein beendeter Hinweis gilt nicht mehr")
	assert.NotContains(t, titles, future.Title, "ein künftiger Hinweis gilt noch nicht")
	assert.NotContains(t, titles, inactive.Title, "abgeschaltete Hinweise sieht das Team nicht")

	// Wichtiges zuerst — die Spalte selbst sortiert alphabetisch falsch
	// ('info' vor 'important'), deshalb steht die Reihenfolge hier fest.
	firstImportant, firstInfo := -1, -1
	for i, row := range rows {
		if firstImportant < 0 && row.Priority == important.Priority {
			firstImportant = i
		}
		if firstInfo < 0 && row.Priority == info.Priority {
			firstInfo = i
		}
	}
	require.GreaterOrEqual(t, firstImportant, 0)
	require.GreaterOrEqual(t, firstInfo, 0)
	assert.Less(t, firstImportant, firstInfo, "wichtige Hinweise stehen oben")
}

func TestStaffNoticeRepository_Acknowledge(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repo := NewStaffNoticeRepository(db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "notice-reader@test.local")
	// Verfasserin und Leserin sind zwei Personen: die Kenntnisnahme der
	// Verfasserin zählt seit #2180 nicht als Leserin mit, und dieser Test
	// prüft die Entprellung ("eine Person zählt einmal"), nicht das.
	author := testpkg.CreateTestAccount(t, db, "notice-author@test.local")

	from, err := calendar.ParseDate("2026-08-01")
	require.NoError(t, err)
	notice := newTestStaffNotice(t, "Bitte bestätigen", from, author.ID, staffNoticeOpts{
		Important:               true,
		RequiresAcknowledgement: true,
	})
	notice.Audience = "all"
	require.NoError(t, repo.Create(ctx, notice))

	require.NoError(t, repo.Acknowledge(ctx, notice.ID, account.ID))

	own, err := repo.AcknowledgedAtFor(ctx, account.ID, []int64{notice.ID})
	require.NoError(t, err)
	first, ok := own[notice.ID]
	require.True(t, ok, "die eigene Kenntnisnahme muss auffindbar sein")

	counts, err := repo.AcknowledgedCounts(ctx, []int64{notice.ID})
	require.NoError(t, err)
	assert.Equal(t, 1, counts[notice.ID])

	// Ein zweiter Klick ändert nichts: der erste Zeitpunkt ist der ehrliche.
	require.NoError(t, repo.Acknowledge(ctx, notice.ID, account.ID))
	own, err = repo.AcknowledgedAtFor(ctx, account.ID, []int64{notice.ID})
	require.NoError(t, err)
	assert.Equal(t, first, own[notice.ID])

	counts, err = repo.AcknowledgedCounts(ctx, []int64{notice.ID})
	require.NoError(t, err)
	assert.Equal(t, 1, counts[notice.ID], "eine Person zählt einmal")
}

// Die Verfasserin bekommt ihre Kenntnisnahme beim Anlegen gestempelt, damit der
// eigene Hinweis sie nicht danach fragt. Als Leserin ist sie damit nicht
// gemeint: sonst stünde bei einem frisch geschriebenen Hinweis „1 Person hat
// bestätigt", und die Leitung liest darin ein Teammitglied.
func TestStaffNoticeRepository_AcknowledgedCountsExcludesTheAuthor(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repo := NewStaffNoticeRepository(db)
	ctx := testpkg.Ctx(t)

	author := testpkg.CreateTestAccount(t, db, "count-author@test.local")
	reader := testpkg.CreateTestAccount(t, db, "count-reader@test.local")

	from, err := calendar.ParseDate("2026-08-01")
	require.NoError(t, err)
	notice := newTestStaffNotice(t, "Bitte bestätigen", from, author.ID, staffNoticeOpts{
		RequiresAcknowledgement: true,
	})
	notice.Audience = "all"
	require.NoError(t, repo.Create(ctx, notice))

	require.NoError(t, repo.Acknowledge(ctx, notice.ID, author.ID))
	counts, err := repo.AcknowledgedCounts(ctx, []int64{notice.ID})
	require.NoError(t, err)
	assert.Zero(t, counts[notice.ID], "die eigene Kenntnisnahme der Verfasserin zählt nicht")

	require.NoError(t, repo.Acknowledge(ctx, notice.ID, reader.ID))
	counts, err = repo.AcknowledgedCounts(ctx, []int64{notice.ID})
	require.NoError(t, err)
	assert.Equal(t, 1, counts[notice.ID], "eine Person aus dem Team hat bestätigt")

	// Die Bestätigungsliste der Leitung (#2208) folgt derselben Regel.
	rows, err := repo.Acknowledgements(ctx, notice.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1, "die Verfasserin steht nicht in der Bestätigungsliste")
	assert.Equal(t, reader.ID, rows[0].AccountID)
}

// Zielgruppe (#2208): die Datenbank grenzt auf "alle" plus die Leserart ein.
// Leserarten stehen hier als Zeichenketten ("staff" = OGS-Portal, "lehrkraft"
// = moto schule), weil dieser Behavior-Test die Modelle nicht importiert.
func TestStaffNoticeRepository_ListValidOnFiltersAudience(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repo := NewStaffNoticeRepository(db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "notice-audience@test.local")
	from, err := calendar.ParseDate("2026-08-01")
	require.NoError(t, err)

	forAll := newTestStaffNotice(t, "Für alle", from, account.ID, staffNoticeOpts{})
	forStaff := newTestStaffNotice(t, "Nur Betreuung", from, account.ID, staffNoticeOpts{})
	forTeachers := newTestStaffNotice(t, "Nur Lehrkräfte", from, account.ID, staffNoticeOpts{})
	forAll.Audience = "all"
	forStaff.Audience = "staff"
	forTeachers.Audience = "lehrkraft"
	require.NoError(t, repo.Create(ctx, forAll))
	require.NoError(t, repo.Create(ctx, forStaff))
	require.NoError(t, repo.Create(ctx, forTeachers))

	titles := func(reader string) []string {
		rows, err := repo.ListValidOn(ctx, from, reader)
		require.NoError(t, err)
		out := make([]string, 0, len(rows))
		for _, row := range rows {
			out = append(out, row.Title)
		}
		return out
	}

	staffView := titles("staff")
	assert.Contains(t, staffView, forAll.Title)
	assert.Contains(t, staffView, forStaff.Title)
	assert.NotContains(t, staffView, forTeachers.Title, "die Betreuung sieht keinen Hinweis nur für Lehrkräfte")

	teacherView := titles("lehrkraft")
	assert.Contains(t, teacherView, forAll.Title)
	assert.Contains(t, teacherView, forTeachers.Title)
	assert.NotContains(t, teacherView, forStaff.Title, "eine Lehrkraft sieht keinen Hinweis nur für die Betreuung")

	// Fail-closed: weder "all" noch Unsinn ist eine Leserart.
	assert.Empty(t, titles("all"))
	assert.Empty(t, titles(""))
}

// Bestätigungsliste (#2208): alle Kenntnisnahmen eines Hinweises, neueste
// zuerst, nur aus dem eigenen Mandanten.
func TestStaffNoticeRepository_Acknowledgements(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repo := NewStaffNoticeRepository(db)
	ctx := testpkg.Ctx(t)

	author := testpkg.CreateTestAccount(t, db, "notice-ack-author@test.local")
	first := testpkg.CreateTestAccount(t, db, "notice-ack-first@test.local")
	second := testpkg.CreateTestAccount(t, db, "notice-ack-second@test.local")

	from, err := calendar.ParseDate("2026-08-01")
	require.NoError(t, err)
	notice := newTestStaffNotice(t, "Bitte bestätigen", from, author.ID, staffNoticeOpts{RequiresAcknowledgement: true})
	other := newTestStaffNotice(t, "Anderer Hinweis", from, author.ID, staffNoticeOpts{RequiresAcknowledgement: true})
	notice.Audience = "all"
	other.Audience = "all"
	require.NoError(t, repo.Create(ctx, notice))
	require.NoError(t, repo.Create(ctx, other))

	rows, err := repo.Acknowledgements(ctx, notice.ID)
	require.NoError(t, err)
	assert.Empty(t, rows, "ohne Kenntnisnahme ist die Liste leer, nicht nil")

	require.NoError(t, repo.Acknowledge(ctx, notice.ID, first.ID))
	require.NoError(t, repo.Acknowledge(ctx, notice.ID, second.ID))
	require.NoError(t, repo.Acknowledge(ctx, other.ID, first.ID))

	rows, err = repo.Acknowledgements(ctx, notice.ID)
	require.NoError(t, err)
	require.Len(t, rows, 2, "die Kenntnisnahme des anderen Hinweises zählt nicht mit")
	assert.False(t, rows[0].AcknowledgedAt.Before(rows[1].AcknowledgedAt), "neueste zuerst")
	got := []int64{rows[0].AccountID, rows[1].AccountID}
	assert.ElementsMatch(t, []int64{first.ID, second.ID}, got)
}

// staffNoticeOpts controls optional fields for newTestStaffNotice.
type staffNoticeOpts struct {
	Important               bool // default priority is "info"
	ValidUntil              *calendar.Date
	Inactive                bool
	RequiresAcknowledgement bool
}

// newTestStaffNotice builds an unsaved Tagesinformation (#2180) with the
// fields a repository test needs: every day of the week, no week pattern,
// active unless opts.Inactive. It does not touch the database — the
// repository under test persists it, so the fixture stays out of that path.
func newTestStaffNotice(tb testing.TB, title string, validFrom calendar.Date, createdBy int64, opts staffNoticeOpts) *scheduleModels.StaffNotice {
	tb.Helper()

	priority := scheduleModels.StaffNoticePriorityInfo
	if opts.Important {
		priority = scheduleModels.StaffNoticePriorityImportant
	}
	return &scheduleModels.StaffNotice{
		Title:                   title,
		Priority:                priority,
		ValidFrom:               validFrom,
		ValidUntil:              opts.ValidUntil,
		Weekdays:                []int16{},
		RequiresAcknowledgement: opts.RequiresAcknowledgement,
		Active:                  !opts.Inactive,
		CreatedBy:               createdBy,
	}
}
