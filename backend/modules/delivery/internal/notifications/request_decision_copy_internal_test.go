package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestDecisionCopy(t *testing.T) {
	t.Parallel()

	t.Run("names the subject area, never the child", func(t *testing.T) {
		cases := map[string]string{
			"care_schedule":   "Betreuungszeiten",
			"pickup_change":   "Abholung",
			"master_data":     "Stammdaten",
			"excused_absence": "Abmeldung",
			"sick_absence":    "Krankmeldung",
		}
		for requestType, expected := range cases {
			title, body := ParentRequestDecisionCopy("de", requestType, "erledigt")
			assert.Equal(t, "Anfrage genehmigt", title)
			assert.Contains(t, body, expected)
		}
	})

	t.Run("an unknown request type still reads as a sentence", func(t *testing.T) {
		title, body := ParentRequestDecisionCopy("de", "something_new", "erledigt")
		assert.Equal(t, "Anfrage genehmigt", title)
		assert.Equal(t, "Ihre Anfrage wurde genehmigt.", body,
			"a future request type must degrade to generic copy, not to an empty push")
	})

	t.Run("a rejection says so", func(t *testing.T) {
		title, body := ParentRequestDecisionCopy("de", "care_schedule", "abgelehnt")
		assert.Equal(t, "Anfrage abgelehnt", title)
		assert.Contains(t, body, "abgelehnt")
	})

	t.Run("uses the guardian locale", func(t *testing.T) {
		title, body := ParentRequestDecisionCopy("en", "care_schedule", "erledigt")
		assert.Equal(t, "Request approved", title)
		assert.Equal(t, "Your care schedule request was approved.", body)
	})
}
