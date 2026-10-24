package peopledirectory

// ClassListEntry is one class-list-only child (#2382) as this resource reads
// it: a name and a free-text class, nothing else exists.
type StudentClassListEntry struct {
	ID          int64
	FirstName   string
	LastName    string
	SchoolClass string
}
