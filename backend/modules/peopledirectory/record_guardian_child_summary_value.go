package peopledirectory

type GuardianChildSummary struct {
	StudentID   int64
	FirstName   string
	LastName    string
	SchoolClass string
	// EnrollmentSubmit reports whether THIS guardian relationship grants
	// parent_portal.enrollment.submit for THIS child. Portal visibility
	// (parent_portal.access) is a weaker fact: a parent may see a child in
	// the picker yet lack submit rights on that specific relationship, so the
	// enrollment form filters the reuse candidates by this flag rather than
	// letting the parent complete the form and only then hit a 403 (#1663).
	EnrollmentSubmit bool
	// Status is the student's lifecycle status (users.students.status). The
	// loader lists every non-alumnus child, but only active/pending students
	// count as "enrolled" for the existing_students audience — so the reuse
	// picker must be able to drop an inactive child instead of offering a
	// candidate the submit gate is guaranteed to reject (#1663).
	Status string
}
