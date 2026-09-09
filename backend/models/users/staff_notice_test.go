package users

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ein Hinweis existiert einmal und wird beim Lesen gegen das Datum geprüft.
// Diese Tests halten genau diese Ableitung fest — ohne Datenbank, weil daran
// nichts mandantenabhängig ist.

func date(t *testing.T, iso string) timezone.Date {
	t.Helper()
	d, err := timezone.ParseDate(iso)
	require.NoError(t, err)
	return d
}

func TestStaffNoticeAppliesOn(t *testing.T) {
	t.Parallel()
	// 2026-08-03 ist ein Montag, 2026-08-04 ein Dienstag.
	monday := date(t, "2026-08-03")
	tuesday := date(t, "2026-08-04")
	sunday := date(t, "2026-08-09")

	base := func() *StaffNotice {
		return &StaffNotice{
			Title:     "Turnhalle belegt",
			Priority:  StaffNoticePriorityInfo,
			ValidFrom: date(t, "2026-08-01"),
			Active:    true,
		}
	}

	t.Run("ohne Wochentage gilt der Hinweis an jedem Tag des Zeitraums", func(t *testing.T) {
		notice := base()
		assert.True(t, notice.AppliesOn(monday))
		assert.True(t, notice.AppliesOn(sunday))
	})

	t.Run("mit Wochentagen gilt er nur dort", func(t *testing.T) {
		notice := base()
		notice.Weekdays = []int16{2} // Dienstag
		assert.False(t, notice.AppliesOn(monday))
		assert.True(t, notice.AppliesOn(tuesday))
	})

	t.Run("Sonntag zählt als ISO-Tag 7, nicht als 0", func(t *testing.T) {
		notice := base()
		notice.Weekdays = []int16{7}
		assert.True(t, notice.AppliesOn(sunday))
		assert.False(t, notice.AppliesOn(monday))
	})

	t.Run("vor Beginn und nach Ende gilt er nicht", func(t *testing.T) {
		notice := base()
		notice.ValidFrom = date(t, "2026-08-04")
		until := date(t, "2026-08-05")
		notice.ValidUntil = &until

		assert.False(t, notice.AppliesOn(monday))
		assert.True(t, notice.AppliesOn(tuesday))
		assert.False(t, notice.AppliesOn(date(t, "2026-08-06")))
	})

	t.Run("das Ende ist einschließend", func(t *testing.T) {
		notice := base()
		until := tuesday
		notice.ValidUntil = &until
		assert.True(t, notice.AppliesOn(tuesday))
	})

	t.Run("ohne Ende läuft er weiter", func(t *testing.T) {
		notice := base()
		assert.True(t, notice.AppliesOn(date(t, "2027-03-01")))
	})

	t.Run("abgeschaltet gilt er nie", func(t *testing.T) {
		notice := base()
		notice.Active = false
		assert.False(t, notice.AppliesOn(monday))
	})
}

func TestStaffNoticeValidate(t *testing.T) {
	t.Parallel()
	valid := func() *StaffNotice {
		return &StaffNotice{
			Title:     "Hinweis",
			Priority:  StaffNoticePriorityInfo,
			Audience:  StaffNoticeAudienceAll,
			ValidFrom: date(t, "2026-08-01"),
			Active:    true,
		}
	}

	t.Run("gültiger Hinweis", func(t *testing.T) {
		require.NoError(t, valid().Validate())
	})

	t.Run("unbekannte Zielgruppe wird abgelehnt", func(t *testing.T) {
		notice := valid()
		notice.Audience = "eltern"
		assert.Error(t, notice.Validate())

		// Leer ist keine Zielgruppe: der Service setzt "all", das Modell
		// verlässt sich nicht darauf.
		notice.Audience = ""
		assert.Error(t, notice.Validate())
	})

	t.Run("Titel darf nicht leer sein", func(t *testing.T) {
		notice := valid()
		notice.Title = "   "
		assert.Error(t, notice.Validate())
	})

	t.Run("unbekannte Wichtigkeit wird abgelehnt", func(t *testing.T) {
		notice := valid()
		notice.Priority = "dringend"
		assert.Error(t, notice.Validate())
	})

	t.Run("Ende vor Beginn wird abgelehnt", func(t *testing.T) {
		notice := valid()
		before := date(t, "2026-07-31")
		notice.ValidUntil = &before
		assert.Error(t, notice.Validate())
	})

	t.Run("Wochentage außerhalb 1..7 werden abgelehnt", func(t *testing.T) {
		notice := valid()
		notice.Weekdays = []int16{0}
		assert.Error(t, notice.Validate())

		notice.Weekdays = []int16{8}
		assert.Error(t, notice.Validate())
	})

	t.Run("Wochenmuster außerhalb 0..2 wird abgelehnt", func(t *testing.T) {
		notice := valid()
		notice.WeekPattern = 3
		assert.Error(t, notice.Validate())
	})
}

func TestStaffNoticeAppliesTo(t *testing.T) {
	t.Parallel()
	// Die Leserart ist das Portal: Betreuung liest als "staff", eine
	// Lehrkraft als "lehrkraft". "all" ist Zielgruppe, nie Leserart.
	withAudience := func(audience string) *StaffNotice {
		return &StaffNotice{Title: "Hinweis", Audience: audience}
	}

	assert.True(t, withAudience(StaffNoticeAudienceAll).AppliesTo(StaffNoticeAudienceStaff))
	assert.True(t, withAudience(StaffNoticeAudienceAll).AppliesTo(StaffNoticeAudienceLehrkraft))

	assert.True(t, withAudience(StaffNoticeAudienceStaff).AppliesTo(StaffNoticeAudienceStaff))
	assert.False(t, withAudience(StaffNoticeAudienceStaff).AppliesTo(StaffNoticeAudienceLehrkraft),
		"ein Hinweis nur für die Betreuung erreicht keine Lehrkraft")

	assert.True(t, withAudience(StaffNoticeAudienceLehrkraft).AppliesTo(StaffNoticeAudienceLehrkraft))
	assert.False(t, withAudience(StaffNoticeAudienceLehrkraft).AppliesTo(StaffNoticeAudienceStaff),
		"ein Hinweis nur für Lehrkräfte erreicht keine Betreuungskraft")

	// Fail-closed: weder "all" noch Unsinn ist eine Leserart, die etwas sieht.
	assert.False(t, withAudience(StaffNoticeAudienceAll).AppliesTo(StaffNoticeAudienceAll))
	assert.False(t, withAudience(StaffNoticeAudienceAll).AppliesTo(""))
	assert.False(t, withAudience(StaffNoticeAudienceAll).AppliesTo("eltern"))
}
