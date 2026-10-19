package workforce

import "strings"

// AbsenceRequestsIncludeOffboardedStaff is the staff lookup of the absence
// request listing: subjects and deciders are resolved among offboarded staff
// too, so a request stays readable after its subject left the school.
const AbsenceRequestsIncludeOffboardedStaff = true

// AbsenceSubjectSearch is the subject name the absence request listing looks
// up in the People Directory: the trimmed free text, and only while the
// listing names no subject persons itself. "" is no name search.
func AbsenceSubjectSearch(search string, subjectPersonIDs []int64) string {
	if subjectPersonIDs != nil {
		return ""
	}
	return strings.TrimSpace(search)
}

// AbsenceSubjectStaffIDs narrows the absence request listing to the staff IDs
// behind the subject persons; staffOf maps persons to their staff IDs. Nil
// persons is no subject filter. An explicitly empty person set, or one
// without staff members, matches nobody: it never falls through to the
// unfiltered listing.
func AbsenceSubjectStaffIDs(personIDs []int64, staffOf func([]int64) ([]int64, error)) (staffIDs []int64, matchesNobody bool, err error) {
	if personIDs == nil {
		return nil, false, nil
	}
	if len(personIDs) == 0 {
		return nil, true, nil
	}
	staffIDs, err = staffOf(personIDs)
	if err != nil {
		return nil, false, err
	}
	if len(staffIDs) == 0 {
		return nil, true, nil
	}
	return staffIDs, false, nil
}
