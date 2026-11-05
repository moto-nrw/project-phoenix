package timetable_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// The staff pool lists staff in German dictionary order (#4949); a staff
// member without a person sorts first, ties fall back to the ID.
func TestSortStaffByName_UsesGermanDictionaryOrder(t *testing.T) {
	t.Parallel()
	staff := []*timetable.StaffMember{
		{ID: 1, Person: &timetable.Person{FirstName: "Eva", LastName: "Zimmermann"}},
		{ID: 2, Person: &timetable.Person{FirstName: "Can", LastName: "Özdemir"}},
		{ID: 3, Person: &timetable.Person{FirstName: "Ida", LastName: "adler"}},
		{ID: 4},
		{ID: 6, Person: &timetable.Person{FirstName: "Ana", LastName: "Ábel"}},
		{ID: 5, Person: &timetable.Person{FirstName: "Ana", LastName: "Ábel"}},
	}
	timetable.SortStaffByName(staff)
	ids := make([]int64, len(staff))
	for i, member := range staff {
		ids[i] = member.ID
	}
	assert.Equal(t, []int64{4, 5, 6, 3, 2, 1}, ids)
}
