// Klassen-Rosters nach dem Ende einer Betreuung (#2487).
//
// FindBySchoolClass filtert nur Abgänger. Ohne den Filter hier stünde ein
// Kind, dessen Betreuung beendet ist, weiter auf dem Tagesblatt der Lehrkraft
// (Schulportal, #1772) und auf jedem Klassenlisten-Export — genau die beiden
// Listen, die die Akzeptanzkriterien namentlich nennen.
package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

func TestClassRosterFiltersCareDate(t *testing.T) {
	t.Parallel()

	t.Run("defaults to today", func(t *testing.T) {
		today := timezone.NewDate(2026, 8, 24)
		assert.Equal(t, today, ClassRosterFilters{}.careDate(today))
	})

	t.Run("follows the day the class view is paged to", func(t *testing.T) {
		// The Lehrkraft class day view pages through the week. A sheet for
		// last Tuesday must show who was in care THEN, not who is today.
		paged := timezone.NewDate(2026, 5, 12)
		assert.Equal(t, paged, ClassRosterFilters{OfferingDate: &paged}.careDate(timezone.NewDate(2026, 8, 24)))
	})
}
