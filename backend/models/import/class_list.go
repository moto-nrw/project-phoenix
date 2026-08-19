package importpkg

// ClassListEntryImportRow is one row of the class-list entry bulk import
// (#2382): the minimal Klassenlisteneintrag — name and school class, nothing
// else. Deliberately no guardians, birthday, group or schedule columns; a
// child needing any of those is a regular student import.
type ClassListEntryImportRow struct {
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	SchoolClass string `json:"school_class"`
}
