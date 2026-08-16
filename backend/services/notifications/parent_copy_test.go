package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParentMessageCopyUsesPortalLocale(t *testing.T) {
	title, body := ParentMessageCopy("en")
	assert.Equal(t, "New message from the OGS", title)
	assert.Equal(t, "You have a new message in the parent portal.", body)
}

func TestParentAppointmentCopyUsesPortalLocale(t *testing.T) {
	title, body := ParentAppointmentCopy("sq", ParentAppointmentCancelled)
	assert.Equal(t, "Takimi u anulua", title)
	assert.Equal(t, "Një takim për ju është anuluar.", body)
}

func TestParentAnnouncementCopyUsesPortalLocale(t *testing.T) {
	title, body := ParentAnnouncementCopy("ru", ParentPollReminder)
	assert.Equal(t, "Напоминание: опрос открыт", title)
	assert.Contains(t, body, "ребёнка")
}

func TestParentRequestDecisionCopyUsesPortalLocale(t *testing.T) {
	title, body := ParentRequestDecisionCopy("sq", "master_data", "abgelehnt")
	assert.Equal(t, "Kërkesa u refuzua", title)
	assert.Equal(t, "Kërkesa juaj për të dhënat bazë u refuzua.", body)
}

// parentLocales are the portal locales every push copy must answer in. An
// unknown locale is not an error: it falls back to German.
var parentLocales = []string{"de", "en", "ru", "sq"}

// assertDistinctPerLocale is the check that catches the copy-paste slip a
// spot-check misses: a locale branch that silently returns another language's
// wording, or an empty string.
func assertDistinctPerLocale(t *testing.T, got map[string]string) {
	t.Helper()
	seen := make(map[string]string, len(got))
	for locale, text := range got {
		assert.NotEmpty(t, text, "locale %s has no copy", locale)
		if other, clash := seen[text]; clash {
			t.Errorf("locale %s reuses the copy of %s: %q", locale, other, text)
		}
		seen[text] = locale
	}
}

func TestParentAnnouncementCopyCoversEveryLocaleAndKind(t *testing.T) {
	for _, kind := range []string{ParentPollPublished, ParentPollReminder, ParentAnnouncementPublished} {
		titles := map[string]string{}
		bodies := map[string]string{}
		for _, locale := range parentLocales {
			titles[locale], bodies[locale] = ParentAnnouncementCopy(locale, kind)
		}
		assertDistinctPerLocale(t, titles)
		assertDistinctPerLocale(t, bodies)
	}
}

func TestParentAppointmentCopyCoversEveryLocaleAndKind(t *testing.T) {
	kinds := []string{
		ParentAppointmentPublished,
		ParentAppointmentUpdated,
		ParentAppointmentCancelled,
		ParentAppointmentReminder,
	}
	for _, kind := range kinds {
		titles := map[string]string{}
		bodies := map[string]string{}
		for _, locale := range parentLocales {
			titles[locale], bodies[locale] = ParentAppointmentCopy(locale, kind)
		}
		assertDistinctPerLocale(t, titles)
		assertDistinctPerLocale(t, bodies)
	}
}

func TestParentMessageCopyCoversEveryLocale(t *testing.T) {
	titles := map[string]string{}
	bodies := map[string]string{}
	for _, locale := range parentLocales {
		titles[locale], bodies[locale] = ParentMessageCopy(locale)
	}
	assertDistinctPerLocale(t, titles)
	assertDistinctPerLocale(t, bodies)
}

func TestParentRequestDecisionCopyCoversEveryLocaleAndRequestType(t *testing.T) {
	for _, requestType := range []string{"care_schedule", "master_data", "excused_absence", "unknown"} {
		for _, status := range []string{"abgelehnt", "genehmigt"} {
			bodies := map[string]string{}
			for _, locale := range parentLocales {
				title, body := ParentRequestDecisionCopy(locale, requestType, status)
				assert.NotEmpty(t, title, "locale %s, type %s has no title", locale, requestType)
				bodies[locale] = body
			}
			assertDistinctPerLocale(t, bodies)
		}
	}
}

// An approval and a rejection must never read the same, in any locale.
func TestParentRequestDecisionCopySeparatesApprovalFromRejection(t *testing.T) {
	for _, locale := range parentLocales {
		rejectedTitle, rejectedBody := ParentRequestDecisionCopy(locale, "care_schedule", "abgelehnt")
		approvedTitle, approvedBody := ParentRequestDecisionCopy(locale, "care_schedule", "genehmigt")
		assert.NotEqual(t, approvedTitle, rejectedTitle, "locale %s", locale)
		assert.NotEqual(t, approvedBody, rejectedBody, "locale %s", locale)
	}
}

// An unknown locale must answer in German rather than fall through to empty.
func TestParentCopyFallsBackToGerman(t *testing.T) {
	deTitle, deBody := ParentMessageCopy("de")
	title, body := ParentMessageCopy("fr")
	assert.Equal(t, deTitle, title)
	assert.Equal(t, deBody, body)
}
