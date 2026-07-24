package users

// GuardianProfileWithChildren bundles a guardian profile, the
// guardian's primary phone (when set), and the active students linked
// to the profile via users.students_guardians. Loader returns nil
// without error when no profile is linked to the given account
// (caller falls through to claims-derived defaults).
type GuardianProfileWithChildren struct {
	Profile      *GuardianProfile
	PrimaryPhone string
	Children     []GuardianChildSummary
}

// GuardianChildSummary is the minimal student shape the parent
// enrollment form needs to prefill a child slot. Mirrors the fields
// the form's "use existing child" picker renders.
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
}
