package importpkg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/dataimport/ports"
)

// FindExisting resolves the row to an existing student (duplicate detection
// in create mode, match key in update mode). Keys, in order: the RFID card
// (survives a class change), first + last name + class, and first + last name
// + birthday (the class-change case without a card).
func (c *StudentImportConfig) FindExisting(ctx context.Context, row importModels.StudentImportRow) (*int64, error) {
	if row.TagID != "" {
		id, err := c.findStudentByTag(ctx, row.TagID)
		if err != nil || id != nil {
			return id, err
		}
	}
	return c.findStudentWithoutTag(ctx, row)
}

func (c *StudentImportConfig) findStudentWithoutTag(ctx context.Context, row importModels.StudentImportRow) (*int64, error) {
	students, err := findStudentsByNameAndClass(ctx, c.Persons, c.Students, row.FirstName, row.LastName, row.SchoolClass)
	if err != nil {
		return nil, err
	}
	if len(students) == 1 {
		return &students[0].ID, nil
	}
	if len(students) > 1 {
		return nil, fmt.Errorf("mehrere Kinder gefunden mit Name '%s %s' in Klasse '%s'",
			row.FirstName, row.LastName, row.SchoolClass)
	}
	if importModeFromContext(ctx) == importModels.ImportModeCreate {
		return nil, nil
	}
	return c.findStudentByNameAndBirthday(ctx, row)
}

// findStudentByTag returns the student wearing the card, if any.
func (c *StudentImportConfig) findStudentByTag(ctx context.Context, tagID string) (*int64, error) {
	wearer, err := c.Persons.FindPersonByTag(ctx, tagID)
	if err != nil {
		if errors.Is(err, ports.ErrPersonNotFound) {
			return nil, nil
		}
		return nil, err
	}
	students, err := c.Students.ListStudentsByPersonID(ctx, []int64{wearer.ID})
	if err != nil {
		return nil, err
	}
	if len(students) == 0 {
		return nil, nil
	}
	return &students[0].ID, nil
}

// findStudentByNameAndBirthday matches a child that changed class: same
// name, same birthday, exactly one hit.
func (c *StudentImportConfig) findStudentByNameAndBirthday(ctx context.Context, row importModels.StudentImportRow) (*int64, error) {
	birthday, err := parseOptionalDate(row.Birthday)
	if err != nil || birthday == nil {
		return nil, nil
	}
	persons, err := findPersonsByExactName(ctx, c.Persons, row.FirstName, row.LastName)
	if err != nil {
		return nil, err
	}
	var personIDs []int64
	for _, person := range persons {
		if person.Birthday == birthday.String() {
			personIDs = append(personIDs, person.ID)
		}
	}
	if len(personIDs) == 0 {
		return nil, nil
	}
	students, err := c.Students.ListStudentsByPersonID(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	switch len(students) {
	case 0:
		return nil, nil
	case 1:
		return &students[0].ID, nil
	default:
		return nil, fmt.Errorf("mehrere Kinder gefunden mit Name '%s %s' und Geburtstag %s", row.FirstName, row.LastName, row.Birthday)
	}
}

// findPersonsByExactName returns the tenant's persons whose first and last
// name equal the given ones, ignoring case and surrounding whitespace.
func findPersonsByExactName(ctx context.Context, persons ports.PersonDirectory, firstName, lastName string) ([]ports.Person, error) {
	first := strings.TrimSpace(firstName)
	last := strings.TrimSpace(lastName)
	if first == "" || last == "" {
		return nil, nil
	}
	// Unpaged on purpose: absence or ambiguity is decided over every namesake,
	// not over the first page of a sorted listing.
	return persons.SearchPersons(ctx, ports.PersonFilter{FirstNameEquals: first, LastNameEquals: last})
}

// findStudentsByNameAndClass returns the non-alumni students of the class
// that carry the given name, matching like the legacy repository lookup:
// case-insensitive and trimmed on all three keys.
func findStudentsByNameAndClass(ctx context.Context, persons ports.PersonDirectory, students ports.StudentDirectory, firstName, lastName, schoolClass string) ([]ports.Student, error) {
	matches, err := findPersonsByExactName(ctx, persons, firstName, lastName)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, nil
	}
	personIDs := make([]int64, 0, len(matches))
	for _, person := range matches {
		personIDs = append(personIDs, person.ID)
	}
	rows, err := students.ListStudentsByPersonID(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	class := strings.TrimSpace(schoolClass)
	result := make([]ports.Student, 0, len(rows))
	for _, student := range rows {
		if student.IsAlumnus() || !strings.EqualFold(strings.TrimSpace(student.SchoolClass), class) {
			continue
		}
		result = append(result, student)
	}
	return result, nil
}
