package peopledirectory

import (
	"context"
	"fmt"
	"strings"
)

// NamesakeDirectory is the slice of the People Directory the class list
// lookups read.
type NamesakeDirectory interface {
	SearchPersons(context.Context, PersonFilter) ([]Person, error)
	ListStudentsByPersonID(context.Context, []int64) ([]Student, error)
	ListStudentsByID(context.Context, []int64) ([]Student, error)
}

// EnrolledStudentIDsByNameAndClass lists the still-enrolled students of the
// class carrying exactly that name: names and class are trimmed, the class
// compares without case, and alumni are left out. A blank name matches
// nobody. Every namesake is read, unpaged: absence is decided over all of
// them, not over the first page of a sorted listing.
func EnrolledStudentIDsByNameAndClass(ctx context.Context, directory NamesakeDirectory, firstName, lastName, schoolClass string) ([]int64, error) {
	first, last := strings.TrimSpace(firstName), strings.TrimSpace(lastName)
	if first == "" || last == "" {
		return nil, nil
	}
	matches, err := directory.SearchPersons(ctx, PersonFilter{FirstNameEquals: first, LastNameEquals: last})
	if err != nil {
		return nil, fmt.Errorf("class list entry student lookup: search persons: %w", err)
	}
	if len(matches) == 0 {
		return nil, nil
	}
	personIDs := make([]int64, 0, len(matches))
	for _, person := range matches {
		personIDs = append(personIDs, person.ID)
	}
	students, err := directory.ListStudentsByPersonID(ctx, personIDs)
	if err != nil {
		return nil, fmt.Errorf("class list entry student lookup: list students: %w", err)
	}
	class := strings.TrimSpace(schoolClass)
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		if student.IsAlumnus() || !strings.EqualFold(strings.TrimSpace(student.SchoolClass), class) {
			continue
		}
		ids = append(ids, student.ID)
	}
	return ids, nil
}

// IsEnrolledStudent reports whether the ID names a student that has not
// graduated. A missing or graduated child counts as not enrolled.
func IsEnrolledStudent(ctx context.Context, directory NamesakeDirectory, studentID int64) (bool, error) {
	students, err := directory.ListStudentsByID(ctx, []int64{studentID})
	if err != nil {
		return false, fmt.Errorf("class list entry student lookup: find student: %w", err)
	}
	for _, student := range students {
		if student.ID == studentID {
			return !student.IsAlumnus(), nil
		}
	}
	return false, nil
}
