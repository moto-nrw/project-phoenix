// Package displayorder is the School Membership adapter onto the shared
// German collation. A class list is read by humans, so its order is the one
// the frontend's localeCompare(..., "de") produces and the grade leads it
// numerically — neither can be expressed in SQL, so the listing is ordered
// here after the rows are fetched.
package displayorder

import "github.com/moto-nrw/project-phoenix/internal/collation"

// ClassListEntry is the part of an entry the order reads. The package keeps
// its own shape because an adapter of this owner may not reach into the
// module's domain values.
type ClassListEntry struct {
	SchoolClass string
	LastName    string
	FirstName   string
	ID          int64
}

// CompareClassListEntry is the class-list display order: school class
// (grade-aware, so "2a" precedes "10a"), then last and first name in German
// dictionary order, then the ID so equal names never swap between two
// listings. Negative when a sorts before b.
func CompareClassListEntry(a, b ClassListEntry) int {
	if r := collation.CompareSchoolClasses(a.SchoolClass, b.SchoolClass); r != 0 {
		return r
	}
	if r := collation.CompareGermanNames(a.LastName, a.FirstName, b.LastName, b.FirstName); r != 0 {
		return r
	}
	switch {
	case a.ID < b.ID:
		return -1
	case a.ID > b.ID:
		return 1
	}
	return 0
}
