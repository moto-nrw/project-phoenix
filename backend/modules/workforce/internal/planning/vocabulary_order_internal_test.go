package planning

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// The Dienstplan overview lists staff in German dictionary order (#4949).
func TestSortOverviewStaff_UsesGermanDictionaryOrder(t *testing.T) {
	t.Parallel()
	staff := []*workforce.PlanStaff{
		{ID: 1, Person: &workforce.PlanStaffPerson{FirstName: "Eva", LastName: "Zimmermann"}},
		{ID: 2, Person: &workforce.PlanStaffPerson{FirstName: "Can", LastName: "Özdemir"}},
		{ID: 3, Person: &workforce.PlanStaffPerson{FirstName: "Ida", LastName: "adler"}},
		{ID: 4},
		{ID: 5, Person: &workforce.PlanStaffPerson{FirstName: "Ana", LastName: "Ábel"}},
	}
	sortOverviewStaff(staff)
	ids := make([]int64, len(staff))
	for i, member := range staff {
		ids[i] = member.ID
	}
	assert.Equal(t, []int64{4, 5, 3, 2, 1}, ids)
}
