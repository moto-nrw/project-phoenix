package application

import (
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
)

func SortDailyParticipants(participants []mealplan.DailyParticipant) {
	sort.SliceStable(participants, func(left, right int) bool {
		a, b := participants[left], participants[right]
		if a.SchoolClass != b.SchoolClass {
			return a.SchoolClass < b.SchoolClass
		}
		if a.LastName != b.LastName {
			return a.LastName < b.LastName
		}
		if a.FirstName != b.FirstName {
			return a.FirstName < b.FirstName
		}
		return a.StudentID < b.StudentID
	})
}
