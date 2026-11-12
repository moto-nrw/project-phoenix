package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func (s *StudentStore) CreateEnrollment(ctx context.Context, input domain.EnrollmentStudent) (peopledirectory.Student, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return peopledirectory.Student{}, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{}
	started := time.Now()
	result, err := s.createEnrollmentStudent(ctx, db, scope.TenantID, input, &stats)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return peopledirectory.Student{}, stats, fmt.Errorf("people directory postgres: create enrollment student: %w", err)
	}
	return result, stats, nil
}

func (s *StudentStore) createEnrollmentStudent(ctx context.Context, db bun.IDB, tenantID int64, input domain.EnrollmentStudent, stats *domain.OperationStats) (peopledirectory.Student, error) {
	var gate *bun.SelectQuery
	var err error
	if s.classGateQuery != nil {
		gate, err = s.classGateQuery(ctx)
	} else {
		err = s.classGate(ctx, false)
		stats.Queries++
		gate = db.NewSelect().ColumnExpr("TRUE")
	}
	if err != nil {
		return peopledirectory.Student{}, err
	}
	record := domain.StudentRecord{PersonID: input.PersonID}
	if input.InitialProfile != nil {
		domain.ApplyEnrollmentPatch(&record, *input.InitialProfile)
	}
	row := studentRow{TenantID: tenantID, PersonID: input.PersonID, SchoolClass: input.SchoolClass, Status: input.Status}
	if row.Status == "" {
		row.Status = "active"
	}
	row.EnrolledFrom, err = optionalDate(input.EnrolledFrom)
	if err != nil {
		return peopledirectory.Student{}, err
	}
	row.EnrolledUntil, err = optionalDate(input.EnrolledUntil)
	if err != nil {
		return peopledirectory.Student{}, err
	}
	stats.Queries++
	err = db.NewRaw(`WITH class_write_gate AS MATERIALIZED (?),
  enrolled_person AS MATERIALIZED (
   SELECT person.id FROM users.persons AS person CROSS JOIN class_write_gate
   WHERE person.id = ? AND person.tenant_id = ? AND person.deleted_at IS NULL
   FOR KEY SHARE OF person
  )
  INSERT INTO users.student_profiles
   (tenant_id, person_id, address_street, address_city, address_postal_code, extra_info,
    photo_consent_given_at, photo_consent_given_by, agb_accepted_at, data_processing_accepted_at, email_contact_accepted_at)
  SELECT ?, id, ?, ?, ?, ?, ?, ?, ?, ?, ? FROM enrolled_person
  RETURNING id, created_at, updated_at`,
		gate, input.PersonID, tenantID, tenantID, record.AddressStreet, record.AddressCity,
		record.AddressPostalCode, record.ExtraInfo, record.PhotoConsentGivenAt, record.PhotoConsentGivenBy,
		record.AGBAcceptedAt, record.DataProcessingAcceptedAt, record.EmailContactAcceptedAt).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return peopledirectory.Student{}, peopledirectory.ErrPersonNotFound
	}
	if err != nil {
		return peopledirectory.Student{}, err
	}
	stats.Rows = 1
	return toStudent(row), nil
}

// enrollmentProfileRow is the person a new profile row was written for.
type enrollmentProfileRow struct {
	ID        int64     `bun:"id"`
	PersonID  int64     `bun:"person_id"`
	CreatedAt time.Time `bun:"created_at"`
	UpdatedAt time.Time `bun:"updated_at"`
}

// CreateEnrollments inserts the profiles of new children in one statement,
// as createEnrollmentStudent inserts one, after the caller took the shared
// class-writes gate (LockEnrollmentClassWrites): each person locked FOR KEY
// SHARE, the same
// columns with the values the initial profile patch gives. A row is assigned
// to its input by person_id (unique per profile), never by RETURNING order;
// a person the school does not have fails the call with ErrPersonNotFound.
func (s *StudentStore) CreateEnrollments(ctx context.Context, inputs []domain.EnrollmentStudent) ([]peopledirectory.Student, domain.OperationStats, error) {
	stats := domain.OperationStats{}
	if len(inputs) == 0 {
		return []peopledirectory.Student{}, stats, nil
	}
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, stats, err
	}
	columns, rows, err := enrollmentProfileColumns(scope.TenantID, inputs)
	if err != nil {
		return nil, stats, err
	}
	var written []enrollmentProfileRow
	stats.Queries++
	started := time.Now()
	err = db.NewRaw(`WITH source AS (
   SELECT k.* FROM unnest(?::bigint[], ?::text[], ?::boolean[], ?::text[], ?::boolean[], ?::text[], ?::boolean[], ?::text[], ?::boolean[],
     ?::timestamptz[], ?::boolean[], ?::bigint[], ?::timestamptz[], ?::boolean[], ?::timestamptz[], ?::boolean[], ?::timestamptz[], ?::boolean[])
   WITH ORDINALITY AS k(person_id, street, street_set, city, city_set, postal, postal_set, extra, extra_set,
     photo_at, photo_at_set, photo_by, agb_at, agb_at_set, processing_at, processing_at_set, contact_at, contact_at_set, ord)
  ),
  enrolled_person AS MATERIALIZED (
   SELECT person.id FROM users.persons AS person
   WHERE person.id IN (SELECT person_id FROM source) AND person.tenant_id = ? AND person.deleted_at IS NULL
   FOR KEY SHARE OF person
  )
  INSERT INTO users.student_profiles
   (tenant_id, person_id, address_street, address_city, address_postal_code, extra_info,
    photo_consent_given_at, photo_consent_given_by, agb_accepted_at, data_processing_accepted_at, email_contact_accepted_at)
  SELECT ?, source.person_id,
   CASE WHEN source.street_set THEN source.street END, CASE WHEN source.city_set THEN source.city END,
   CASE WHEN source.postal_set THEN source.postal END, CASE WHEN source.extra_set THEN source.extra END,
   CASE WHEN source.photo_at_set THEN source.photo_at END, NULLIF(source.photo_by, 0),
   CASE WHEN source.agb_at_set THEN source.agb_at END, CASE WHEN source.processing_at_set THEN source.processing_at END,
   CASE WHEN source.contact_at_set THEN source.contact_at END
  FROM source JOIN enrolled_person ON enrolled_person.id = source.person_id
  ORDER BY source.ord
  RETURNING id, person_id, created_at, updated_at`,
		append(columns, scope.TenantID, scope.TenantID)...).Scan(ctx, &written)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: create enrollment students: %w", err)
	}
	result, err := enrollmentStudents(rows, written)
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(written))
	return result, stats, nil
}

// enrollmentStudents maps the written profiles back onto the input rows by
// person; a person without a profile row was deleted or not in the tenant.
func enrollmentStudents(rows []studentRow, written []enrollmentProfileRow) ([]peopledirectory.Student, error) {
	if len(written) != len(rows) {
		return nil, peopledirectory.ErrPersonNotFound
	}
	byPerson := make(map[int64]enrollmentProfileRow, len(written))
	for _, row := range written {
		byPerson[row.PersonID] = row
	}
	result := make([]peopledirectory.Student, len(rows))
	for i, row := range rows {
		profile, ok := byPerson[row.PersonID]
		if !ok {
			return nil, peopledirectory.ErrPersonNotFound
		}
		row.ID, row.CreatedAt, row.UpdatedAt = profile.ID, profile.CreatedAt, profile.UpdatedAt
		result[i] = toStudent(row)
	}
	return result, nil
}

// enrollmentProfileColumns are the statement arrays of CreateEnrollments,
// one per column with a set flag for nullable ones, and the rows the
// profiles return as, built as createEnrollmentStudent builds its row.
func enrollmentProfileColumns(tenantID int64, inputs []domain.EnrollmentStudent) ([]any, []studentRow, error) {
	n := len(inputs)
	persons, photoBy := make([]int64, n), make([]int64, n)
	texts := [4][]*string{make([]*string, n), make([]*string, n), make([]*string, n), make([]*string, n)}
	times := [4][]*time.Time{make([]*time.Time, n), make([]*time.Time, n), make([]*time.Time, n), make([]*time.Time, n)}
	rows := make([]studentRow, n)
	seen := make(map[int64]bool, n)
	for i, input := range inputs {
		if seen[input.PersonID] {
			return nil, nil, fmt.Errorf("people directory postgres: create enrollment students: person %d twice", input.PersonID)
		}
		seen[input.PersonID] = true
		record := domain.StudentRecord{PersonID: input.PersonID}
		if input.InitialProfile != nil {
			domain.ApplyEnrollmentPatch(&record, *input.InitialProfile)
		}
		row := studentRow{TenantID: tenantID, PersonID: input.PersonID, SchoolClass: input.SchoolClass, Status: input.Status}
		if row.Status == "" {
			row.Status = "active"
		}
		var err error
		if row.EnrolledFrom, err = optionalDate(input.EnrolledFrom); err != nil {
			return nil, nil, err
		}
		if row.EnrolledUntil, err = optionalDate(input.EnrolledUntil); err != nil {
			return nil, nil, err
		}
		rows[i] = row
		persons[i] = input.PersonID
		texts[0][i], texts[1][i], texts[2][i], texts[3][i] = record.AddressStreet, record.AddressCity, record.AddressPostalCode, record.ExtraInfo
		times[0][i], times[1][i], times[2][i], times[3][i] = record.PhotoConsentGivenAt, record.AGBAcceptedAt, record.DataProcessingAcceptedAt, record.EmailContactAcceptedAt
		if record.PhotoConsentGivenBy != nil {
			photoBy[i] = *record.PhotoConsentGivenBy
		}
	}
	args := []any{pgdialect.Array(persons)}
	for _, column := range texts {
		values, set := nullableTexts(column)
		args = append(args, pgdialect.Array(values), pgdialect.Array(set))
	}
	photoAt, photoAtSet := nullableTimes(times[0])
	args = append(args, pgdialect.Array(photoAt), pgdialect.Array(photoAtSet), pgdialect.Array(photoBy))
	for _, column := range times[1:] {
		values, set := nullableTimes(column)
		args = append(args, pgdialect.Array(values), pgdialect.Array(set))
	}
	return args, rows, nil
}

// nullableTimes splits optional instants into values and a set flag each.
func nullableTimes(values []*time.Time) ([]time.Time, []bool) {
	times, set := make([]time.Time, len(values)), make([]bool, len(values))
	for i, value := range values {
		if value != nil {
			times[i], set[i] = *value, true
		}
	}
	return times, set
}
