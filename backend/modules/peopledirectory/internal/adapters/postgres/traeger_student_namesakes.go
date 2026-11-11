package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// NewStudentNamesakes reads on the caller's administrative transaction.
func NewStudentNamesakes(database func(context.Context) (bun.IDB, TenantScope, error)) peopledirectory.StudentNamesakes {
	return studentNamesakes{database: database}
}

type studentNamesakes struct {
	database func(context.Context) (bun.IDB, TenantScope, error)
}

// studentNamesakesSQL takes the first names, the last names and the dates of
// birth of the identities as three arrays of the same length, then the school
// set, for persons and profiles alike. Names compare as
// FindEnrolledMatchCandidates compares them.
const studentNamesakesSQL = `SELECT wanted.ordinal - 1 AS identity, profile.tenant_id, profile.id
FROM unnest(?0::text[], ?1::text[], ?2::date[]) WITH ORDINALITY AS wanted(first_name, last_name, birthday, ordinal)
JOIN users.persons AS person
  ON person.tenant_id = ANY(?3) AND person.birthday = wanted.birthday AND person.deleted_at IS NULL
  AND LOWER(NORMALIZE(TRIM(person.first_name), NFC) COLLATE public.german_name) = LOWER(NORMALIZE(TRIM(wanted.first_name), NFC) COLLATE public.german_name)
  AND LOWER(NORMALIZE(TRIM(person.last_name), NFC) COLLATE public.german_name) = LOWER(NORMALIZE(TRIM(wanted.last_name), NFC) COLLATE public.german_name)
JOIN users.student_profiles AS profile
  ON profile.tenant_id = person.tenant_id AND profile.person_id = person.id
WHERE profile.tenant_id = ANY(?3)
ORDER BY identity, profile.tenant_id, profile.id`

// StudentNamesakesByTenant lists the student profiles of the schools with
// the names and the date of birth of each identity, in one statement.
func (s studentNamesakes) StudentNamesakesByTenant(ctx context.Context, schools []int64, identities []peopledirectory.StudentIdentity) ([]peopledirectory.StudentNamesake, error) {
	result := []peopledirectory.StudentNamesake{}
	if len(schools) == 0 || len(identities) == 0 {
		return result, nil
	}
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	if !scope.AcrossSchools {
		return nil, errors.New("people directory: student namesakes need the administrative transaction")
	}
	firstNames, lastNames, birthdays := make([]string, len(identities)), make([]string, len(identities)), make([]string, len(identities))
	for i, identity := range identities {
		firstNames[i], lastNames[i], birthdays[i] = identity.FirstName, identity.LastName, identity.Birthday.String()
	}
	var rows []struct {
		Identity int   `bun:"identity"`
		TenantID int64 `bun:"tenant_id"`
		ID       int64 `bun:"id"`
	}
	if err := db.NewRaw(studentNamesakesSQL, pgdialect.Array(firstNames), pgdialect.Array(lastNames), pgdialect.Array(birthdays), pgdialect.Array(schools)).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("people directory postgres: student namesakes: %w", err)
	}
	for _, row := range rows {
		result = append(result, peopledirectory.StudentNamesake{Identity: row.Identity, SchoolID: row.TenantID, StudentProfileID: row.ID})
	}
	return result, nil
}
