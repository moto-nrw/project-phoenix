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
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

func studentWithCareEnd(id int64, lastCareDay *timezone.Date) *userModels.Student {
	student := &userModels.Student{SchoolClass: "3a", EnrolledUntil: lastCareDay}
	student.ID = id
	return student
}

func TestFilterCareRunningOn(t *testing.T) {
	t.Parallel()

	day := timezone.NewDate(2026, 9, 15)
	before := day.AddDays(-1)
	after := day.AddDays(1)

	running := studentWithCareEnd(1, nil)
	lastDayIsToday := studentWithCareEnd(2, &day)
	endsLater := studentWithCareEnd(3, &after)
	endedYesterday := studentWithCareEnd(4, &before)

	kept := filterCareRunningOn(
		[]*userModels.Student{running, lastDayIsToday, endsLater, endedYesterday, nil},
		day,
	)

	ids := make([]int64, 0, len(kept))
	for _, student := range kept {
		ids = append(ids, student.ID)
	}

	assert.Equal(t,
		[]int64{running.ID, lastDayIsToday.ID, endsLater.ID}, ids,
		"a child stays on the class roster up to and including their last care day")
	assert.NotContains(t, ids, endedYesterday.ID,
		"and drops off the day after")
}

func TestClassRosterFiltersCareDate(t *testing.T) {
	t.Parallel()

	t.Run("defaults to today", func(t *testing.T) {
		assert.Equal(t, timezone.TodayDate(), ClassRosterFilters{}.careDate())
	})

	t.Run("follows the day the class view is paged to", func(t *testing.T) {
		// The Lehrkraft class day view pages through the week. A sheet for
		// last Tuesday must show who was in care THEN, not who is today.
		paged := timezone.NewDate(2026, 5, 12)
		assert.Equal(t, paged, ClassRosterFilters{OfferingDate: &paged}.careDate())
	})
}
