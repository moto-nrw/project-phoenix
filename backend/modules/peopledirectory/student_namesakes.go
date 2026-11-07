package peopledirectory

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"
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

// NameAndClass is one namesake question: a child's names and class.
type NameAndClass struct {
	FirstName, LastName, SchoolClass string
}

// EnrolledStudentIDsByNamesAndClasses answers EnrolledStudentIDsByNameAndClass
// for every key; result[i] answers keys[i] in the order the single-key lookup
// returns. Keys spelled in ASCII share one name read over all their pairs and
// are told apart here: for ASCII text the owner's LOWER/NFC/BTRIM equality is
// plain ASCII case folding after trimming spaces. Every other key keeps its
// own exact-name read, because Go cannot reproduce the database's Unicode
// equality. One student read hydrates all matched persons. The answers thus
// share READ COMMITTED snapshots instead of taking one per key.
func EnrolledStudentIDsByNamesAndClasses(ctx context.Context, directory NamesakeDirectory, keys []NameAndClass) ([][]int64, error) {
	matches, err := namesakePersons(ctx, directory, keys)
	if err != nil {
		return nil, fmt.Errorf("class list entry student lookup: search persons: %w", err)
	}
	result := make([][]int64, len(keys))
	personIDs := distinctPersonIDs(matches)
	if len(personIDs) == 0 {
		return result, nil
	}
	students, err := directory.ListStudentsByPersonID(ctx, personIDs)
	if err != nil {
		return nil, fmt.Errorf("class list entry student lookup: list students: %w", err)
	}
	for i, key := range keys {
		if len(matches[i]) > 0 {
			result[i] = enrolledNamesakeIDs(matches[i], students, key.SchoolClass)
		}
	}
	return result, nil
}

// namesakePersons returns the persons per key that SearchPersons with the
// key's exact names would return.
func namesakePersons(ctx context.Context, directory NamesakeDirectory, keys []NameAndClass) ([][]Person, error) {
	matches := make([][]Person, len(keys))
	shared := []int{}
	scalar := []int{}
	filter := PersonFilter{}
	for i, key := range keys {
		first, last := strings.TrimSpace(key.FirstName), strings.TrimSpace(key.LastName)
		switch {
		case first == "" || last == "":
		case isASCII(first) && isASCII(last):
			shared = append(shared, i)
			filter.Names = append(filter.Names, PersonName{FirstName: first, LastName: last})
		default:
			scalar = append(scalar, i)
		}
	}
	if _, err := scalarNamesakePersons(ctx, directory, keys, scalar, matches); err != nil {
		return nil, err
	}
	if len(shared) == 0 {
		return matches, nil
	}
	persons, err := directory.SearchPersons(ctx, filter)
	if err != nil {
		return nil, err
	}
	if !allASCIINames(persons) {
		// A non-ASCII spelling the database folds onto an ASCII key: let the
		// scalar reads decide instead of guessing the Unicode rules.
		return scalarNamesakePersons(ctx, directory, keys, shared, matches)
	}
	for _, i := range shared {
		matches[i] = asciiNamesakes(persons, keys[i])
	}
	return matches, nil
}

func allASCIINames(persons []Person) bool {
	for _, person := range persons {
		if !isASCII(person.FirstName) || !isASCII(person.LastName) {
			return false
		}
	}
	return true
}

// asciiNamesakes keeps the persons of the shared read that carry the key's
// names, in the read's order.
func asciiNamesakes(persons []Person, key NameAndClass) []Person {
	// The shared read sends the trimmed key, as the single-key lookup does.
	first, last := asciiNameKey(strings.TrimSpace(key.FirstName)), asciiNameKey(strings.TrimSpace(key.LastName))
	var result []Person
	for _, person := range persons {
		if asciiNameKey(person.FirstName) == first && asciiNameKey(person.LastName) == last {
			result = append(result, person)
		}
	}
	return result
}

func scalarNamesakePersons(ctx context.Context, directory NamesakeDirectory, keys []NameAndClass, indexes []int, matches [][]Person) ([][]Person, error) {
	for _, i := range indexes {
		persons, err := directory.SearchPersons(ctx, PersonFilter{
			FirstNameEquals: strings.TrimSpace(keys[i].FirstName), LastNameEquals: strings.TrimSpace(keys[i].LastName),
		})
		if err != nil {
			return nil, err
		}
		matches[i] = persons
	}
	return matches, nil
}

// asciiNameKey is the owner's name equality restricted to ASCII text:
// BTRIM removes spaces only, and LOWER folds A-Z.
func asciiNameKey(value string) string {
	return strings.ToLower(strings.Trim(value, " "))
}

func isASCII(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func distinctPersonIDs(matches [][]Person) []int64 {
	ids := []int64{}
	seen := map[int64]bool{}
	for _, persons := range matches {
		for _, person := range persons {
			if !seen[person.ID] {
				seen[person.ID] = true
				ids = append(ids, person.ID)
			}
		}
	}
	return ids
}

// enrolledNamesakeIDs keeps the hydrated students of the matched persons that
// are enrolled in the class, in hydration order, as the single-key lookup does.
func enrolledNamesakeIDs(persons []Person, students []Student, schoolClass string) []int64 {
	matched := make(map[int64]bool, len(persons))
	for _, person := range persons {
		matched[person.ID] = true
	}
	class := strings.TrimSpace(schoolClass)
	ids := []int64{}
	for _, student := range students {
		if matched[student.PersonID] && !student.IsAlumnus() && strings.EqualFold(strings.TrimSpace(student.SchoolClass), class) {
			ids = append(ids, student.ID)
		}
	}
	return ids
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
