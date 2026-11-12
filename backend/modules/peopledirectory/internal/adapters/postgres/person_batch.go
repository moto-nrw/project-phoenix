package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory/internal/domain"
)

// personOrdinalRow is one person together with the input position it belongs
// to (ADR 0067 addendum of 2026-11-08, ID contract: assigned by input
// position, never by an assumed RETURNING order).
type personOrdinalRow struct {
	Ord int64 `bun:"ord"`
	personRow
}

// CreatePersons inserts the persons in input order in one statement, after
// the same shared staff-person-merge lock Create takes. It writes the columns
// Create writes and leaves id, both timestamps and deleted_at to their
// defaults, as Create does. Birthday, card and account are NULL where the
// input has none; Create leaves them to the column default, which is NULL
// for all three (no DEFAULT clause). The unique indexes refuse a taken card
// or account as there.
func (s *Store) CreatePersons(ctx context.Context, inputs []peopledirectory.CreatePerson) ([]peopledirectory.Person, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	if scope.TenantID <= 0 {
		return nil, domain.OperationStats{}, errors.New("people directory postgres: tenant is required to create persons")
	}
	firsts, lasts, birthdays := make([]string, len(inputs)), make([]string, len(inputs)), make([]string, len(inputs))
	tags, accounts := make([]string, len(inputs)), make([]int64, len(inputs))
	for i, input := range inputs {
		if _, err := optionalDate(input.Birthday); err != nil {
			return nil, domain.OperationStats{}, err
		}
		firsts[i], lasts[i], birthdays[i] = input.FirstName, input.LastName, input.Birthday
		if input.TagID != nil {
			tags[i] = *input.TagID
		}
		if input.AccountID != nil {
			accounts[i] = *input.AccountID
		}
	}
	var rows []personOrdinalRow
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`WITH staff_person_merge_gate AS MATERIALIZED (SELECT pg_advisory_xact_lock_shared(hashtextextended(?, 0)))
		MERGE INTO users.persons AS p
		USING (SELECT k.* FROM unnest(?::text[], ?::text[], ?::text[], ?::text[], ?::bigint[]) WITH ORDINALITY AS k(first_name, last_name, birthday, tag_id, account_id, ord)
			CROSS JOIN staff_person_merge_gate ORDER BY k.ord) AS s
		ON false
		WHEN NOT MATCHED THEN INSERT (tenant_id, first_name, last_name, birthday, tag_id, account_id)
			VALUES (?, s.first_name, s.last_name, NULLIF(s.birthday, '')::date, NULLIF(s.tag_id, ''), NULLIF(s.account_id, 0))
		RETURNING s.ord, p.*`,
		peopledirectory.StaffPersonMergeLockKey(scope.TenantID),
		pgdialect.Array(firsts), pgdialect.Array(lasts), pgdialect.Array(birthdays), pgdialect.Array(tags), pgdialect.Array(accounts),
		scope.TenantID).Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, wrapWriteError("create", err)
	}
	result := make([]peopledirectory.Person, len(inputs))
	seen := make([]bool, len(inputs))
	for _, row := range rows {
		index := int(row.Ord) - 1
		if index < 0 || index >= len(inputs) || seen[index] {
			return nil, stats, fmt.Errorf("people directory postgres: create persons: unexpected input position %d", row.Ord)
		}
		seen[index] = true
		result[index] = toDomain(row.personRow)
	}
	if len(rows) != len(inputs) {
		return nil, stats, fmt.Errorf("people directory postgres: create persons: %d of %d persons returned", len(rows), len(inputs))
	}
	stats.Rows = int64(len(rows))
	return result, stats, nil
}
