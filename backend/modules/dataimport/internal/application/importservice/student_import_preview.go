package importpkg

import (
	"context"
	"strings"
	"unicode/utf8"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/dataimport/ports"
)

// PreviewConfig builds a read-only batch view through the existing directory
// ports. It is never installed on the shared config or used by write phases.
// Failed bulk reads roll back before the original row-by-row path runs, so an
// optimization cannot turn one lookup failure into errors on unrelated rows.
func (c *StudentImportConfig) PreviewConfig(ctx context.Context, rows []importModels.StudentImportRow) ports.ImportConfig[importModels.StudentImportRow] {
	if c.Transactions == nil || len(rows) < 2 {
		return c
	}
	filter, names := studentPreviewNames(rows)
	if len(filter.Names) == 0 {
		return c
	}
	persons, students, err := c.readPreviewMatches(ctx, filter)
	if err != nil {
		return c
	}
	// Both owner predicates use database LOWER with NFC normalization, not
	// accent-insensitive search or Go's Unicode folding. Keep scalar matching
	// for non-ASCII results rather than reimplementing the database equality.
	// An empty bulk result proves absence because it uses the scalar predicate.
	if len(persons) > 0 && !asciiPreviewNames(filter.Names, persons) {
		return c
	}
	for _, person := range persons {
		// BTRIM in the owner removes spaces, not arbitrary Unicode whitespace.
		name := previewName(strings.Trim(person.FirstName, " "), strings.Trim(person.LastName, " "))
		names[name] = append(names[name], person)
	}
	clone := *c
	clone.Persons = previewPersons{PersonDirectory: c.Persons, names: names}
	personIDs := make(map[int64]bool, len(persons))
	for _, person := range persons {
		personIDs[person.ID] = true
	}
	clone.Students = previewStudents{StudentDirectory: c.Students, personIDs: personIDs, students: students}
	return &clone
}

func studentPreviewNames(rows []importModels.StudentImportRow) (ports.PersonFilter, map[ports.PersonName][]ports.Person) {
	names := make(map[ports.PersonName][]ports.Person, len(rows))
	seen := make(map[ports.PersonName]bool, len(rows))
	filter := ports.PersonFilter{}
	for _, row := range rows {
		name := ports.PersonName{FirstName: strings.TrimSpace(row.FirstName), LastName: strings.TrimSpace(row.LastName)}
		if name.FirstName == "" || name.LastName == "" || seen[name] {
			continue
		}
		seen[name] = true
		names[previewName(name.FirstName, name.LastName)] = nil
		filter.Names = append(filter.Names, name)
	}
	return filter, names
}

func (c *StudentImportConfig) readPreviewMatches(ctx context.Context, filter ports.PersonFilter) ([]ports.Person, []ports.Student, error) {
	var persons []ports.Person
	var students []ports.Student
	err := c.Transactions.Savepoint(ctx, func(ctx context.Context) error {
		var err error
		persons, err = c.Persons.SearchPersons(ctx, filter)
		if err != nil {
			return err
		}
		ids := make([]int64, 0, len(persons))
		for _, person := range persons {
			ids = append(ids, person.ID)
		}
		if len(ids) > 0 {
			students, err = c.Students.ListStudentsByPersonID(ctx, ids)
		}
		return err
	})
	return persons, students, err
}

func previewName(first, last string) ports.PersonName {
	return ports.PersonName{FirstName: strings.ToLower(first), LastName: strings.ToLower(last)}
}

func asciiPreviewNames(names []ports.PersonName, persons []ports.Person) bool {
	ascii := func(value string) bool {
		for _, r := range value {
			if r >= utf8.RuneSelf {
				return false
			}
		}
		return true
	}
	for _, name := range names {
		if !ascii(name.FirstName) || !ascii(name.LastName) {
			return false
		}
	}
	for _, person := range persons {
		if !ascii(person.FirstName) || !ascii(person.LastName) {
			return false
		}
	}
	return true
}

type previewPersons struct {
	ports.PersonDirectory
	names map[ports.PersonName][]ports.Person
}

func (p previewPersons) SearchPersons(ctx context.Context, filter ports.PersonFilter) ([]ports.Person, error) {
	if ctx.Err() != nil {
		return p.PersonDirectory.SearchPersons(ctx, filter)
	}

	if filter.FirstNameEquals != "" && filter.LastNameEquals != "" &&
		filter.FirstNamePrefix == "" && filter.LastNamePrefix == "" && filter.FullNameContains == "" &&
		filter.TagID == "" && len(filter.AccountIDs) == 0 && len(filter.Names) == 0 && filter.Page == 0 && filter.PageSize == 0 {
		name := previewName(strings.TrimSpace(filter.FirstNameEquals), strings.TrimSpace(filter.LastNameEquals))
		if persons, found := p.names[name]; found {
			return persons, nil
		}
	}
	return p.PersonDirectory.SearchPersons(ctx, filter)
}

type previewStudents struct {
	ports.StudentDirectory
	personIDs map[int64]bool
	students  []ports.Student
}

func (s previewStudents) ListStudentsByPersonID(ctx context.Context, ids []int64) ([]ports.Student, error) {
	if ctx.Err() != nil {
		return s.StudentDirectory.ListStudentsByPersonID(ctx, ids)
	}

	wanted := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if !s.personIDs[id] {
			return s.StudentDirectory.ListStudentsByPersonID(ctx, ids)
		}
		wanted[id] = true
	}
	result := make([]ports.Student, 0)
	for _, student := range s.students {
		if wanted[student.PersonID] {
			result = append(result, student)
		}
	}
	return result, nil
}
