package users_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tagesinformationen (#2180). Geprüft wird, was nur die Datenbank beantwortet:
// die Vorauswahl nach Zeitraum und die Reihenfolge. Wochentag und Wochenmuster
// liegen im Service und sind dort getestet.

func TestStaffNoticeRepository_ListValidOn(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StaffNotice
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "notice-author@test.local")

	day := func(iso string) timezone.Date {
		d, err := timezone.ParseDate(iso)
		require.NoError(t, err)
		return d
	}

	create := func(t *testing.T, title, priority string, from string, until *string) *users.StaffNotice {
		t.Helper()
		notice := &users.StaffNotice{
			Title:     title,
			Body:      "",
			Priority:  priority,
			ValidFrom: day(from),
			Weekdays:  []int16{},
			Active:    true,
			CreatedBy: account.ID,
		}
		if until != nil {
			u := day(*until)
			notice.ValidUntil = &u
		}
		require.NoError(t, repo.Create(ctx, notice))
		return notice
	}

	ended := "2026-08-04"
	inactive := &users.StaffNotice{
		Title:     "Abgeschaltet",
		Priority:  users.StaffNoticePriorityInfo,
		ValidFrom: day("2026-08-01"),
		Weekdays:  []int16{},
		Active:    false,
		CreatedBy: account.ID,
	}
	require.NoError(t, repo.Create(ctx, inactive))

	info := create(t, "Laufender Hinweis", users.StaffNoticePriorityInfo, "2026-08-01", nil)
	important := create(t, "Wichtiger Hinweis", users.StaffNoticePriorityImportant, "2026-08-01", nil)
	create(t, "Abgelaufen", users.StaffNoticePriorityInfo, "2026-08-01", &ended)
	create(t, "Beginnt später", users.StaffNoticePriorityInfo, "2026-09-01", nil)

	rows, err := repo.ListValidOn(ctx, day("2026-08-06"))
	require.NoError(t, err)

	titles := make([]string, 0, len(rows))
	for _, row := range rows {
		titles = append(titles, row.Title)
	}

	assert.Contains(t, titles, info.Title)
	assert.Contains(t, titles, important.Title)
	assert.NotContains(t, titles, "Abgelaufen", "ein beendeter Hinweis gilt nicht mehr")
	assert.NotContains(t, titles, "Beginnt später", "ein künftiger Hinweis gilt noch nicht")
	assert.NotContains(t, titles, "Abgeschaltet", "abgeschaltete Hinweise sieht das Team nicht")

	// Wichtiges zuerst — die Spalte selbst sortiert alphabetisch falsch
	// ('info' vor 'important'), deshalb steht die Reihenfolge hier fest.
	var firstImportant, firstInfo int = -1, -1
	for i, row := range rows {
		if firstImportant < 0 && row.Priority == users.StaffNoticePriorityImportant {
			firstImportant = i
		}
		if firstInfo < 0 && row.Priority == users.StaffNoticePriorityInfo {
			firstInfo = i
		}
	}
	require.GreaterOrEqual(t, firstImportant, 0)
	require.GreaterOrEqual(t, firstInfo, 0)
	assert.Less(t, firstImportant, firstInfo, "wichtige Hinweise stehen oben")
}

func TestStaffNoticeRepository_Acknowledge(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StaffNotice
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "notice-reader@test.local")

	from, err := timezone.ParseDate("2026-08-01")
	require.NoError(t, err)
	notice := &users.StaffNotice{
		Title:                   "Bitte bestätigen",
		Priority:                users.StaffNoticePriorityImportant,
		ValidFrom:               from,
		Weekdays:                []int16{},
		RequiresAcknowledgement: true,
		Active:                  true,
		CreatedBy:               account.ID,
	}
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
