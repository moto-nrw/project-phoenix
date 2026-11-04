package api

import "slices"

// seedEnrollmentCourseBookings are the courses the approved online
// enrollments book besides their care days, by child. The phase then shows
// believable occupancy: nine of twelve places in the Fußball-AG, all eight of
// the Theater-AG, so the parents app keeps a full course whose request lands
// on the waiting list (#3075). Before, the Fußball-AG stood at 0 of 12. The
// holiday care belongs to its own holiday phase (#5139), so a school-year
// enrollment does not book it.
var seedEnrollmentCourseBookings = map[string][]string{
	"Zeynep Çelik":     {"ag-theater"},
	"Emma Klein":       {"ag-theater"},
	"Oskar Wolf":       {"ag-fussball"},
	"Zofia Kowalczyk":  {"ag-fussball", "ag-theater"},
	"Ben Hartmann":     {"ag-fussball"},
	"Clara Neumann":    {"ag-fussball", "ag-theater"},
	"Artem Kovalenko":  {"ag-fussball", "ag-theater"},
	"Omar Al-Hassan":   {"ag-fussball"},
	"Amelie Vogt":      {"ag-theater"},
	"Felix Kovac":      {"ag-fussball"},
	"Hannah Schmitz":   {"ag-theater"},
	"Viktor Reimer":    {"ag-fussball"},
	"Elias Sommerfeld": {"ag-fussball"},
	"Sofia Rossi":      {"ag-theater"},
}

// withSeedCourseBookings adds the child's courses to the offerings an
// enrollment selects. Their days are fixed by the school, so they need no
// day selection.
func withSeedCourseBookings(offerings map[string]int64, childFirstName, childLastName string, offeringIDs []int64) []int64 {
	ids := slices.Clone(offeringIDs)
	for _, key := range seedEnrollmentCourseBookings[childFirstName+" "+childLastName] {
		if id := offerings[key]; id != 0 && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids
}

// seedEnrollmentChildAnswers are a child's answers to the form's questions
// about the child (allergies, swimming), which the decision page lists under
// "Angaben der Kinder". Luka Berisha's open request (product screenshot
// anmeldung-entscheiden) has an allergy to note; the youngest cannot swim yet.
func seedEnrollmentChildAnswers(childFirstName, childLastName string) map[string]any {
	answers := map[string]any{"allergies": "keine", "swimming_permission": true}
	switch childFirstName + " " + childLastName {
	case "Luka Berisha":
		answers["allergies"] = "Laktoseintoleranz. Bitte keine Milchprodukte beim Mittagessen."
	case "Emma Klein", "Anastasia Smirnow":
		answers["swimming_permission"] = false
	}
	return answers
}
