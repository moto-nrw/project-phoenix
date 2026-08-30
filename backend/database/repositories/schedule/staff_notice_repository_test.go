package schedule_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
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

	repo := repositories.NewFactory(db).StaffNotice
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "notice-author@test.local")

	day := func(iso string) timezone.Date {
		d, err := timezone.ParseDate(iso)
		require.NoError(t, err)
		return d
	}

	ended := day("2026-08-04")
	inactive := testpkg.NewTestStaffNotice(t, "Abgeschaltet", day("2026-08-01"), account.ID, testpkg.StaffNoticeOpts{Inactive: true})
	info := testpkg.NewTestStaffNotice(t, "Laufender Hinweis", day("2026-08-01"), account.ID, testpkg.StaffNoticeOpts{})
	important := testpkg.NewTestStaffNotice(t, "Wichtiger Hinweis", day("2026-08-01"), account.ID, testpkg.StaffNoticeOpts{Important: true})
	expired := testpkg.NewTestStaffNotice(t, "Abgelaufen", day("2026-08-01"), account.ID, testpkg.StaffNoticeOpts{ValidUntil: &ended})
	future := testpkg.NewTestStaffNotice(t, "Beginnt später", day("2026-09-01"), account.ID, testpkg.StaffNoticeOpts{})
	require.NoError(t, repo.Create(ctx, inactive))
	require.NoError(t, repo.Create(ctx, info))
	require.NoError(t, repo.Create(ctx, important))
	require.NoError(t, repo.Create(ctx, expired))
	require.NoError(t, repo.Create(ctx, future))

	rows, err := repo.ListValidOn(ctx, day("2026-08-06"))
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

	repo := repositories.NewFactory(db).StaffNotice
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "notice-reader@test.local")

	from, err := timezone.ParseDate("2026-08-01")
	require.NoError(t, err)
	notice := testpkg.NewTestStaffNotice(t, "Bitte bestätigen", from, account.ID, testpkg.StaffNoticeOpts{
		Important:               true,
		RequiresAcknowledgement: true,
	})
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
