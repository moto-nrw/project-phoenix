package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The debtor number of a child (#7007) in
// parent_billing.child_debtor_numbers, keyed by the family and the child
// (parentbilling.ChildKey). Every statement runs in the Träger transaction
// and names the Träger in its own filter; the retention runs with the one of
// the families (DeleteExpiredFamilies).

type childDebtorNumberRow struct {
	bun.BaseModel `bun:"table:parent_billing.child_debtor_numbers"`

	ID                 int64          `bun:"id,pk,autoincrement"`
	OrganizationID     int64          `bun:"organization_id"`
	FamilyEmail        string         `bun:"family_email"`
	ChildKey           string         `bun:"child_key"`
	ChildFirstName     string         `bun:"child_first_name"`
	ChildLastName      string         `bun:"child_last_name"`
	ChildBirthDate     *calendar.Date `bun:"child_birth_date,type:date"`
	DebtorNumber       string         `bun:"debtor_number"`
	LastUsedOn         calendar.Date  `bun:"last_used_on,type:date"`
	UpdatedByAccountID *int64         `bun:"updated_by_account_id"`
	UpdatedAt          time.Time      `bun:"updated_at,nullzero,default:current_timestamp"`
}

func (r childDebtorNumberRow) number() parentbilling.ChildDebtorNumber {
	return parentbilling.ChildDebtorNumber{ID: r.ID, FamilyEmail: r.FamilyEmail, ChildFirstName: r.ChildFirstName,
		ChildLastName: r.ChildLastName, ChildBirthDate: r.ChildBirthDate, DebtorNumber: r.DebtorNumber,
		LastUsedOn: r.LastUsedOn, UpdatedAt: r.UpdatedAt}
}

func childNumbersOfRows(rows []childDebtorNumberRow) []parentbilling.ChildDebtorNumber {
	numbers := make([]parentbilling.ChildDebtorNumber, len(rows))
	for i, row := range rows {
		numbers[i] = row.number()
	}
	return numbers
}

// ChildDebtorNumbers reads every number of a child of the Träger, by family
// and child.
func (s *Store) ChildDebtorNumbers(ctx context.Context, organizationID int64) ([]parentbilling.ChildDebtorNumber, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []childDebtorNumberRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).
		OrderExpr("family_email, child_key").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read the debtor numbers of the children: %w", err)
	}
	return childNumbersOfRows(rows), nil
}

// SaveChildDebtorNumbers writes the number of each child in one statement:
// a child without a row gets one, a child with a row its new number. With
// onlyNew a child that has a number keeps it (the import). Each row counts
// today, or the later last day of the contract that needs it
// (ChildDebtorNumber.LastUsedOn), as a day its number is needed, so a number
// prepared for a contract ahead outlives the retention until then. It
// answers the rows it wrote.
func (s *Store) SaveChildDebtorNumbers(ctx context.Context, organizationID, accountID int64, numbers []parentbilling.ChildDebtorNumber,
	today calendar.Date, onlyNew bool,
) ([]parentbilling.ChildDebtorNumber, error) {
	if len(numbers) == 0 {
		return nil, nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	rows := make([]childDebtorNumberRow, len(numbers))
	for i, number := range numbers {
		usedOn := today
		if number.LastUsedOn.After(today) {
			usedOn = number.LastUsedOn
		}
		rows[i] = childDebtorNumberRow{OrganizationID: organizationID, FamilyEmail: parentbilling.FamilyKey(number.FamilyEmail),
			ChildKey:       parentbilling.ChildKey(number.ChildFirstName, number.ChildLastName, number.ChildBirthDate),
			ChildFirstName: number.ChildFirstName, ChildLastName: number.ChildLastName, ChildBirthDate: number.ChildBirthDate,
			DebtorNumber: number.DebtorNumber, LastUsedOn: usedOn, UpdatedByAccountID: accountRef(accountID), UpdatedAt: time.Now()}
	}
	query := db.NewInsert().Model(&rows)
	if onlyNew {
		// A child whose number was removed takes the new one.
		query = query.On("CONFLICT (organization_id, family_email, child_key) DO UPDATE").
			Set("debtor_number = EXCLUDED.debtor_number").
			Set("last_used_on = GREATEST(child_debtor_number_row.last_used_on, EXCLUDED.last_used_on)").
			Set("updated_by_account_id = EXCLUDED.updated_by_account_id").Set("updated_at = CURRENT_TIMESTAMP").
			Where("child_debtor_number_row.debtor_number = ''")
	} else {
		query = query.On("CONFLICT (organization_id, family_email, child_key) DO UPDATE").
			Set("debtor_number = EXCLUDED.debtor_number").Set("child_first_name = EXCLUDED.child_first_name").
			Set("child_last_name = EXCLUDED.child_last_name").
			Set("last_used_on = GREATEST(child_debtor_number_row.last_used_on, EXCLUDED.last_used_on)").
			Set("updated_by_account_id = EXCLUDED.updated_by_account_id").Set("updated_at = CURRENT_TIMESTAMP")
	}
	var saved []childDebtorNumberRow
	if err := query.Returning("*").Scan(ctx, &saved); err != nil {
		return nil, fmt.Errorf("save the debtor numbers of the children: %w", err)
	}
	return childNumbersOfRows(saved), nil
}

// DeleteChildDebtorNumber removes the number of one child and answers the
// child as it had it, nil when the child had none. The row stays with an
// empty number: the child stays known, also without a contract (#7007).
func (s *Store) DeleteChildDebtorNumber(ctx context.Context, organizationID, accountID int64, familyEmail, childKey string) (*parentbilling.ChildDebtorNumber, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var removed []childDebtorNumberRow
	if err := db.NewRaw(`UPDATE parent_billing.child_debtor_numbers AS child
		SET debtor_number = '', updated_by_account_id = ?3, updated_at = CURRENT_TIMESTAMP
		FROM (SELECT id, debtor_number FROM parent_billing.child_debtor_numbers
			WHERE organization_id = ?0 AND family_email = ?1 AND child_key = ?2 AND debtor_number <> '' FOR UPDATE) AS before
		WHERE child.id = before.id
		RETURNING child.id, child.organization_id, child.family_email, child.child_key, child.child_first_name, child.child_last_name,
			child.child_birth_date, before.debtor_number, child.last_used_on, child.updated_by_account_id, child.updated_at`,
		organizationID, familyEmail, childKey, accountRef(accountID)).Scan(ctx, &removed); err != nil {
		return nil, fmt.Errorf("delete the debtor number of the child: %w", err)
	}
	if len(removed) == 0 {
		return nil, nil
	}
	number := removed[0].number()
	return &number, nil
}

// ExtendChildDebtorNumbers moves the last day each number is needed to the
// later of the stored one and the given one, in one statement; the numbers
// stay as they are (#7007).
func (s *Store) ExtendChildDebtorNumbers(ctx context.Context, organizationID int64, numbers []parentbilling.ChildDebtorNumber) error {
	if len(numbers) == 0 {
		return nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	emails, keys, days := make([]string, len(numbers)), make([]string, len(numbers)), make([]string, len(numbers))
	for i, number := range numbers {
		emails[i] = parentbilling.FamilyKey(number.FamilyEmail)
		keys[i] = parentbilling.ChildKey(number.ChildFirstName, number.ChildLastName, number.ChildBirthDate)
		days[i] = number.LastUsedOn.String()
	}
	if _, err := db.NewRaw(`UPDATE parent_billing.child_debtor_numbers AS child
		SET last_used_on = GREATEST(child.last_used_on, needed.used_on)
		FROM unnest(?1::text[], ?2::text[], ?3::date[]) AS needed(email, child_key, used_on)
		WHERE child.organization_id = ?0 AND child.family_email = needed.email AND child.child_key = needed.child_key
			AND needed.used_on > child.last_used_on`,
		organizationID, pgdialect.Array(emails), pgdialect.Array(keys), pgdialect.Array(days)).Exec(ctx); err != nil {
		return fmt.Errorf("extend the debtor numbers of the children: %w", err)
	}
	return nil
}

// InsertChildDebtorNumberEvents records one action of the trail for each
// number in one statement.
func (s *Store) InsertChildDebtorNumberEvents(ctx context.Context, organizationID, accountID int64, action string,
	numbers []parentbilling.ChildDebtorNumber,
) error {
	if len(numbers) == 0 {
		return nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	ids := make([]int64, len(numbers))
	for i, number := range numbers {
		ids[i] = number.ID
	}
	if _, err := db.NewRaw(`INSERT INTO parent_billing.child_debtor_number_events (organization_id, child_debtor_number_id, action, actor_account_id)
		SELECT ?0, id, ?2, ?3 FROM unnest(?1::bigint[]) AS number(id)`,
		organizationID, pgdialect.Array(ids), action, accountRef(accountID)).Exec(ctx); err != nil {
		return fmt.Errorf("record the trail of the children's debtor numbers: %w", err)
	}
	return nil
}

// deleteExpiredChildNumbers deletes, in the retention of the families, the
// numbers of children whose period ended under the same rule as a family
// without a contract (#6692, #7007), logs their count per Träger as
// family_debtor_number, and then deletes the trail entries whose number is
// gone and which are as old. The caller holds the Träger.
func deleteExpiredChildNumbers(ctx context.Context, db bun.IDB, cutoff, today calendar.Date) (int, error) {
	var children int
	if err := db.NewRaw(`WITH removed AS (
			DELETE FROM parent_billing.child_debtor_numbers AS child
			WHERE child.last_used_on < ?0::date
				AND EXISTS (SELECT 1 FROM parent_billing.beleg_retention_instructions AS instruction
					WHERE instruction.organization_id = child.organization_id AND instruction.beleg_type = ?1
						AND instruction.replaced_at IS NULL)
				AND NOT EXISTS (SELECT 1 FROM parent_billing.beleg_retention_suspensions AS suspension
					WHERE suspension.organization_id = child.organization_id AND suspension.beleg_type = ?1
						AND suspension.beleg_id IS NULL AND suspension.lifted_at IS NULL)
				AND NOT EXISTS (SELECT 1 FROM parent_billing.documents AS document
					WHERE document.organization_id = child.organization_id
						AND COALESCE(document.family_email, LOWER(document.recipient_email)) = child.family_email)
				AND NOT EXISTS (SELECT 1 FROM parent_billing.care_invoices AS invoice
					WHERE invoice.organization_id = child.organization_id
						AND COALESCE(invoice.family_email, LOWER(invoice.recipient_email)) = child.family_email)
			RETURNING child.organization_id),
		logged AS (
			INSERT INTO parent_billing.beleg_deletions (organization_id, beleg_type, deleted_on, deleted_count)
			SELECT organization_id, ?2, ?3::date, count(*)::int FROM removed GROUP BY organization_id)
		SELECT count(*) FROM removed`, cutoff, parentbilling.BelegHolidayDocument, parentbilling.BelegFamilyDebtorNumber, today).
		Scan(ctx, &children); err != nil {
		return 0, fmt.Errorf("parent billing postgres: delete expired debtor numbers of children: %w", err)
	}
	result, err := db.NewRaw(`DELETE FROM parent_billing.child_debtor_number_events AS event
		WHERE event.occurred_at < ?0::date
			AND NOT EXISTS (SELECT 1 FROM parent_billing.child_debtor_numbers AS child
				WHERE child.organization_id = event.organization_id AND child.id = event.child_debtor_number_id)`, cutoff).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("parent billing postgres: delete the expired trail of children's debtor numbers: %w", err)
	}
	events, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return children + int(events), nil
}
