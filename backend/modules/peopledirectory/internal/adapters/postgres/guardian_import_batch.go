package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// The bundled writes of the data import's new guardians (#5552, ADR 0067
// addendum of 2026-11-08, step 5c). Each writes the columns its single
// counterpart writes and leaves the rest to the column defaults: a profile
// starts with childless_since = now() and no account, as CreateGuardian
// leaves it, until LinkNewGuardians clears the mark as LinkGuardianToStudent
// does.

// nullableTexts splits optional texts into values and a set flag per value,
// so a NULL stays NULL and an empty text stays empty.
func nullableTexts(values []*string) ([]string, []bool) {
	texts, set := make([]string, len(values)), make([]bool, len(values))
	for i, value := range values {
		if value != nil {
			texts[i], set[i] = *value, true
		}
	}
	return texts, set
}

type ordinalID struct {
	Ord int64 `bun:"ord"`
	ID  int64 `bun:"id"`
}

// InsertNewGuardians inserts the profiles in one statement and assigns each
// id to its input position (MERGE … ON false … RETURNING the source
// position; the ID contract of the addendum, never an assumed RETURNING
// order).
func (s *GuardianStore) InsertNewGuardians(ctx context.Context, profiles []domain.NewGuardianProfile) ([]int64, domain.OperationStats, error) {
	if len(profiles) == 0 {
		return []int64{}, domain.OperationStats{}, nil
	}
	db, scope, err := s.tenantDB(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	n := len(profiles)
	firsts, lasts, methods, languages := make([]string, n), make([]string, n), make([]string, n), make([]string, n)
	streets, cities, postals, notes := make([]*string, n), make([]*string, n), make([]*string, n), make([]*string, n)
	for i, profile := range profiles {
		firsts[i], lasts[i], methods[i], languages[i] = profile.FirstName, profile.LastName, profile.PreferredContactMethod, profile.LanguagePreference
		streets[i], cities[i], postals[i], notes[i] = profile.AddressStreet, profile.AddressCity, profile.PostalCode, profile.Notes
	}
	street, streetSet := nullableTexts(streets)
	city, citySet := nullableTexts(cities)
	postal, postalSet := nullableTexts(postals)
	note, noteSet := nullableTexts(notes)
	var rows []ordinalID
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`MERGE INTO users.guardian_profiles AS g
		USING (SELECT k.* FROM unnest(?::text[], ?::text[], ?::text[], ?::boolean[], ?::text[], ?::boolean[], ?::text[], ?::boolean[], ?::text[], ?::text[], ?::text[], ?::boolean[])
			WITH ORDINALITY AS k(first_name, last_name, street, street_set, city, city_set, postal, postal_set, method, language, note, note_set, ord) ORDER BY k.ord) AS s
		ON false
		WHEN NOT MATCHED THEN INSERT (tenant_id, first_name, last_name, address_street, address_city, address_postal_code, preferred_contact_method, language_preference, notes)
			VALUES (?, s.first_name, s.last_name, CASE WHEN s.street_set THEN s.street END, CASE WHEN s.city_set THEN s.city END,
				CASE WHEN s.postal_set THEN s.postal END, s.method, s.language, CASE WHEN s.note_set THEN s.note END)
		RETURNING s.ord, g.id`,
		pgdialect.Array(firsts), pgdialect.Array(lasts), pgdialect.Array(street), pgdialect.Array(streetSet),
		pgdialect.Array(city), pgdialect.Array(citySet), pgdialect.Array(postal), pgdialect.Array(postalSet),
		pgdialect.Array(methods), pgdialect.Array(languages), pgdialect.Array(note), pgdialect.Array(noteSet),
		scope.TenantID).Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, wrapGuardianWriteError("create new guardians", err)
	}
	ids := make([]int64, n)
	for _, row := range rows {
		index := int(row.Ord) - 1
		if index < 0 || index >= n || ids[index] != 0 {
			return nil, stats, fmt.Errorf("people directory postgres: create new guardians: unexpected input position %d", row.Ord)
		}
		ids[index] = row.ID
	}
	if len(rows) != n {
		return nil, stats, fmt.Errorf("people directory postgres: create new guardians: %d of %d profiles returned", len(rows), n)
	}
	stats.Rows = int64(n)
	return ids, stats, nil
}

// InsertNewGuardianPhones inserts the phone rows of new profiles in one
// statement, with the columns AddGuardianPhone writes.
func (s *GuardianStore) InsertNewGuardianPhones(ctx context.Context, phones []domain.NewGuardianPhone) (domain.OperationStats, error) {
	if len(phones) == 0 {
		return domain.OperationStats{}, nil
	}
	db, scope, err := s.tenantDB(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	rows := make([]guardianPhoneRow, len(phones))
	for i, phone := range phones {
		rows[i] = guardianPhoneRow{
			TenantID: scope.TenantID, GuardianProfileID: phone.GuardianProfileID, PhoneNumber: phone.PhoneNumber,
			PhoneType: phone.PhoneType, Label: phone.Label, IsPrimary: phone.IsPrimary, Priority: phone.Priority,
		}
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewInsert().Model(&rows).ModelTableExpr("users.guardian_phone_numbers").
		Column("tenant_id", "guardian_profile_id", "phone_number", "phone_type", "label", "is_primary", "priority").
		Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, wrapGuardianWriteError("add new guardian phones", err)
	}
	stats.Rows, _ = result.RowsAffected()
	if stats.Rows != int64(len(rows)) {
		return stats, fmt.Errorf("people directory postgres: add new guardian phones: %d of %d rows written", stats.Rows, len(rows))
	}
	return stats, nil
}

type newLinkRow struct {
	ID                int64  `bun:"id"`
	StudentID         int64  `bun:"student_id"`
	GuardianProfileID int64  `bun:"guardian_profile_id"`
	AccountID         *int64 `bun:"account_id"`
}

// LinkNewGuardians clears the retention mark of every linked profile and
// inserts the relationships in one statement, as LinkGuardianToStudent does
// per link: the UPDATE takes each profile's row lock and the insert consumes
// it, and the account comes from the UPDATE's RETURNING, never from an
// earlier read. A link is assigned by its (student, guardian) key. A
// profile the school does not have yields no row and fails the call with
// ErrGuardianNotFound.
func (s *GuardianStore) LinkNewGuardians(ctx context.Context, links []domain.GuardianLinkRecord) ([]domain.NewGuardianLinkResult, domain.OperationStats, error) {
	if len(links) == 0 {
		return []domain.NewGuardianLinkResult{}, domain.OperationStats{}, nil
	}
	db, scope, err := s.tenantDB(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	args, index, err := newLinkArguments(scope.TenantID, links)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	n := len(links)
	var rows []newLinkRow
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`WITH linked_guardian AS (
		UPDATE users.guardian_profiles SET childless_since = NULL
		WHERE tenant_id = ?0 AND id = ANY(?2::bigint[])
		RETURNING id, account_id
	), inserted AS (
		INSERT INTO users.student_guardian_relationships
			(tenant_id, student_id, guardian_profile_id, relationship_type, guardian_role,
			 is_primary, is_emergency_contact, emergency_priority, is_payer)
		SELECT ?0, k.student_id, k.guardian_profile_id, k.relationship_type, k.guardian_role,
			k.is_primary, k.is_emergency_contact, k.emergency_priority, k.is_payer
		FROM unnest(?1::bigint[], ?2::bigint[], ?3::text[], ?4::text[], ?5::boolean[], ?6::boolean[], ?7::bigint[], ?8::boolean[])
			WITH ORDINALITY AS k(student_id, guardian_profile_id, relationship_type, guardian_role, is_primary, is_emergency_contact, emergency_priority, is_payer, ord)
		JOIN linked_guardian ON linked_guardian.id = k.guardian_profile_id
		ORDER BY k.ord
		RETURNING id, student_id, guardian_profile_id
	)
	SELECT inserted.id, inserted.student_id, inserted.guardian_profile_id, linked_guardian.account_id
	FROM inserted JOIN linked_guardian ON linked_guardian.id = inserted.guardian_profile_id`,
		args...).Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, wrapGuardianWriteError("link new guardians", err)
	}
	results := make([]domain.NewGuardianLinkResult, n)
	for _, row := range rows {
		i, ok := index[[2]int64{row.StudentID, row.GuardianProfileID}]
		if !ok || results[i].ID != 0 {
			return nil, stats, fmt.Errorf("people directory postgres: link new guardians: unexpected pair %d/%d", row.StudentID, row.GuardianProfileID)
		}
		results[i] = domain.NewGuardianLinkResult{ID: row.ID, AccountID: row.AccountID}
	}
	if len(rows) != n {
		return nil, stats, domain.ErrGuardianNotFound
	}
	stats.Rows = int64(n)
	return results, stats, nil
}

// newLinkArguments are the statement arguments of LinkNewGuardians, one
// array per column, and each (student, guardian) pair's input position.
func newLinkArguments(tenantID int64, links []domain.GuardianLinkRecord) ([]any, map[[2]int64]int, error) {
	n := len(links)
	students, guardians, priorities := make([]int64, n), make([]int64, n), make([]int64, n)
	types, roles := make([]string, n), make([]string, n)
	primaries, emergencies, payers := make([]bool, n), make([]bool, n), make([]bool, n)
	index := make(map[[2]int64]int, n)
	for i, link := range links {
		students[i], guardians[i], priorities[i] = link.StudentID, link.GuardianProfileID, int64(link.EmergencyPriority)
		types[i], roles[i] = link.RelationshipType, link.GuardianRole
		primaries[i], emergencies[i], payers[i] = link.IsPrimary, link.IsEmergencyContact, link.IsPayer
		key := [2]int64{link.StudentID, link.GuardianProfileID}
		if _, seen := index[key]; seen {
			return nil, nil, fmt.Errorf("people directory postgres: link new guardians: pair %v twice", key)
		}
		index[key] = i
	}
	return []any{tenantID, pgdialect.Array(students), pgdialect.Array(guardians), pgdialect.Array(types), pgdialect.Array(roles),
		pgdialect.Array(primaries), pgdialect.Array(emergencies), pgdialect.Array(priorities), pgdialect.Array(payers)}, index, nil
}
