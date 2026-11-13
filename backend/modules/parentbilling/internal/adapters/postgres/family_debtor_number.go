package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The debtor number of a family (#6692). A holiday registration writes it at
// the row of its family in parent_billing.family_billing, keyed by the
// family's address; a family the holiday care knows only gets a row without
// a contract, which keeps the parent who registered and the latest day a
// registration needs the number (last_used_on). Every statement runs in the
// Träger transaction and names the Träger in its own filter, except the
// retention, which runs in the administrative transaction across every
// Träger.

// LockFamilies adds a row without a contract for each family that has
// none and holds the row of every family until the transaction ends, in the
// order of their IDs, so two confirmations of one family decide one after
// the other. It answers the rows as they stand.
func (s *Store) LockFamilies(ctx context.Context, organizationID int64, families []parentbilling.FamilyMemory) ([]parentbilling.FamilyBilling, error) {
	if len(families) == 0 {
		return nil, nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	emails, first, last, used := make([]string, len(families)), make([]string, len(families)), make([]string, len(families)), make([]string, len(families))
	for i, family := range families {
		emails[i], first[i], last[i], used[i] = family.FamilyEmail, family.FirstName, family.LastName, family.UsedOn.String()
	}
	if _, err := db.NewRaw(`INSERT INTO parent_billing.family_billing (organization_id, family_email, care_contract_id, recipient_kind,
			holder_first_name, holder_last_name, last_used_on)
		SELECT ?0, family.email, NULL, ?5, family.first_name, family.last_name, family.used_on
		FROM unnest(?1::text[], ?2::text[], ?3::text[], ?4::date[]) AS family(email, first_name, last_name, used_on)
		ON CONFLICT (organization_id, family_email) DO NOTHING`,
		organizationID, pgdialect.Array(emails), pgdialect.Array(first), pgdialect.Array(last), pgdialect.Array(used),
		parentbilling.RecipientApplicant).Exec(ctx); err != nil {
		return nil, fmt.Errorf("add the families of the holiday care: %w", err)
	}
	var rows []familyBillingRow
	if err := db.NewRaw(`SELECT * FROM parent_billing.family_billing
		WHERE organization_id = ? AND family_email = ANY(?::text[]) ORDER BY id FOR UPDATE`,
		organizationID, pgdialect.Array(emails)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("hold the families of the holiday care: %w", err)
	}
	return familiesOfRows(rows), nil
}

// RememberFamilies writes what the holiday registrations tell their
// families in one statement: the parent who registered, the latest day a
// registration needs the number (never earlier than before) and, where
// given, the number. A number that changes names the registration it came
// from; the same number again keeps where it came from, so a registration
// that took the proposal never becomes its origin. It answers the rows it
// wrote. A contract row without a number only changes when one is given: a
// confirmation without a number refreshes only the use of a remembered one,
// or of a row of the holiday care without a contract, which a Träger that
// keeps the number per child knows its families by (#7007).
func (s *Store) RememberFamilies(ctx context.Context, organizationID, accountID int64, families []parentbilling.FamilyMemory) ([]parentbilling.FamilyBilling, error) {
	if len(families) == 0 {
		return nil, nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	count := len(families)
	emails, first, last, used, numbers := make([]string, count), make([]string, count), make([]string, count), make([]string, count), make([]string, count)
	set, origins := make([]bool, count), make([]int64, count)
	for i, family := range families {
		emails[i], first[i], last[i], used[i] = family.FamilyEmail, family.FirstName, family.LastName, family.UsedOn.String()
		origins[i] = family.RegistrationID
		if family.DebtorNumber != nil {
			numbers[i], set[i] = *family.DebtorNumber, true
		}
	}
	var rows []familyBillingRow
	if err := db.NewRaw(`UPDATE parent_billing.family_billing AS family
		SET holder_first_name = CASE WHEN memory.first_name <> '' OR memory.last_name <> '' THEN memory.first_name ELSE family.holder_first_name END,
			holder_last_name = CASE WHEN memory.first_name <> '' OR memory.last_name <> '' THEN memory.last_name ELSE family.holder_last_name END,
			last_used_on = GREATEST(family.last_used_on, memory.used_on),
			debtor_number = CASE WHEN memory.set THEN NULLIF(memory.number, '') ELSE family.debtor_number END,
			debtor_number_registration_id = CASE
				WHEN memory.set AND COALESCE(family.debtor_number, '') <> memory.number THEN NULLIF(memory.origin, 0)
				ELSE family.debtor_number_registration_id END,
			updated_by_account_id = ?7, updated_at = CURRENT_TIMESTAMP
		FROM unnest(?1::text[], ?2::text[], ?3::text[], ?4::date[], ?5::text[], ?6::boolean[], ?8::bigint[])
			AS memory(email, first_name, last_name, used_on, number, set, origin)
		WHERE family.organization_id = ?0 AND family.family_email = memory.email
			AND (memory.set OR family.debtor_number IS NOT NULL OR family.care_contract_id IS NULL)
		RETURNING family.*`,
		organizationID, pgdialect.Array(emails), pgdialect.Array(first), pgdialect.Array(last), pgdialect.Array(used),
		pgdialect.Array(numbers), pgdialect.Array(set), accountRef(accountID), pgdialect.Array(origins)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("remember the families of the holiday care: %w", err)
	}
	return familiesOfRows(rows), nil
}

// FamilyBillingOf reads the row of one family, nil without one.
func (s *Store) FamilyBillingOf(ctx context.Context, organizationID int64, familyEmail string) (*parentbilling.FamilyBilling, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var row familyBillingRow
	err = db.NewSelect().Model(&row).Where("organization_id = ?", organizationID).Where("family_email = ?", familyEmail).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the family billing: %w", err)
	}
	family := row.family()
	return &family, nil
}

// FamilyHasDocuments reports whether a holiday document or a monthly
// invoice of the Träger belongs to the family, by the rule of the list of
// families: the family it copied, else its recipient's address.
func (s *Store) FamilyHasDocuments(ctx context.Context, organizationID int64, familyEmail string) (bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return false, err
	}
	var found bool
	if err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM parent_billing.documents AS document
			WHERE document.organization_id = ?0 AND COALESCE(document.family_email, LOWER(document.recipient_email)) = ?1)
		OR EXISTS (SELECT 1 FROM parent_billing.care_invoices AS invoice
			WHERE invoice.organization_id = ?0 AND COALESCE(invoice.family_email, LOWER(invoice.recipient_email)) = ?1)`,
		organizationID, familyEmail).Scan(ctx, &found); err != nil {
		return false, fmt.Errorf("read whether the family has documents: %w", err)
	}
	return found, nil
}

// SetFamilyDebtorNumber sets the number of a family from its list. A family
// without a row gets one without a contract, kept from today; a row without
// a contract counts today as a day the number is needed. The number then
// comes from no registration, so no registration corrects it silently.
func (s *Store) SetFamilyDebtorNumber(ctx context.Context, organizationID, accountID int64, familyEmail, number string, today calendar.Date) (parentbilling.FamilyBilling, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.FamilyBilling{}, err
	}
	row := &familyBillingRow{OrganizationID: organizationID, FamilyEmail: familyEmail, RecipientKind: parentbilling.RecipientApplicant,
		DebtorNumber: textRef(number), LastUsedOn: &today, UpdatedByAccountID: accountRef(accountID), UpdatedAt: time.Now()}
	if err := db.NewInsert().Model(row).
		On("CONFLICT (organization_id, family_email) DO UPDATE").
		Set("debtor_number = EXCLUDED.debtor_number").Set("debtor_number_registration_id = NULL").
		Set("last_used_on = CASE WHEN family_billing_row.care_contract_id IS NULL THEN GREATEST(family_billing_row.last_used_on, EXCLUDED.last_used_on) ELSE family_billing_row.last_used_on END").
		Set("updated_by_account_id = EXCLUDED.updated_by_account_id").Set("updated_at = CURRENT_TIMESTAMP").
		Returning("*").Scan(ctx); err != nil {
		return parentbilling.FamilyBilling{}, fmt.Errorf("set the debtor number of the family: %w", err)
	}
	return row.family(), nil
}

// DeleteExpiredFamilies deletes, across every Träger, the rows without a
// contract whose period ended (#6692): last_used_on lies before 1 January
// of the third year before today's, and no holiday document or monthly
// invoice of the family is kept any more. Like the Belege, a row goes only
// under a current instruction of its Träger for the holiday documents and
// without a deletion stop of that whole type, and under the lock of the
// Träger (parent_billing.lock_beleg_retention). The count per Träger goes
// into the deletion log (BelegFamilyDebtorNumber); audit.data_deletions
// belongs to a school and cannot hold it. Then it deletes the trail entries
// without a contract whose row is gone and which are as old, and the
// numbers of children under the same rule (#7007).
func (s *Store) DeleteExpiredFamilies(ctx context.Context, today calendar.Date) (int, error) {
	db, err := adminDB(ctx)
	if err != nil {
		return 0, err
	}
	cutoff := calendar.NewDate(today.Year()-parentbilling.FamilyDebtorNumberRetentionYears, time.January, 1)
	var traeger []int64
	if err := db.NewRaw(`SELECT organization_id FROM parent_billing.family_billing
			WHERE care_contract_id IS NULL AND last_used_on < ?0::date
		UNION SELECT organization_id FROM parent_billing.child_debtor_numbers WHERE last_used_on < ?0::date
		ORDER BY organization_id`, cutoff).Scan(ctx, &traeger); err != nil {
		return 0, fmt.Errorf("parent billing postgres: list the Träger of expired family debtor numbers: %w", err)
	}
	if err := lockTraeger(ctx, db, traeger...); err != nil {
		return 0, err
	}
	var families int
	if err := db.NewRaw(`WITH removed AS (
			DELETE FROM parent_billing.family_billing AS family
			WHERE family.care_contract_id IS NULL AND family.last_used_on < ?0::date
				AND EXISTS (SELECT 1 FROM parent_billing.beleg_retention_instructions AS instruction
					WHERE instruction.organization_id = family.organization_id AND instruction.beleg_type = ?1
						AND instruction.replaced_at IS NULL)
				AND NOT EXISTS (SELECT 1 FROM parent_billing.beleg_retention_suspensions AS suspension
					WHERE suspension.organization_id = family.organization_id AND suspension.beleg_type = ?1
						AND suspension.beleg_id IS NULL AND suspension.lifted_at IS NULL)
				AND NOT EXISTS (SELECT 1 FROM parent_billing.documents AS document
					WHERE document.organization_id = family.organization_id
						AND COALESCE(document.family_email, LOWER(document.recipient_email)) = family.family_email)
				AND NOT EXISTS (SELECT 1 FROM parent_billing.care_invoices AS invoice
					WHERE invoice.organization_id = family.organization_id
						AND COALESCE(invoice.family_email, LOWER(invoice.recipient_email)) = family.family_email)
			RETURNING family.organization_id),
		logged AS (
			INSERT INTO parent_billing.beleg_deletions (organization_id, beleg_type, deleted_on, deleted_count)
			SELECT organization_id, ?2, ?3::date, count(*)::int FROM removed GROUP BY organization_id)
		SELECT count(*) FROM removed`, cutoff, parentbilling.BelegHolidayDocument, parentbilling.BelegFamilyDebtorNumber, today).
		Scan(ctx, &families); err != nil {
		return 0, fmt.Errorf("parent billing postgres: delete expired family debtor numbers: %w", err)
	}
	result, err := db.NewRaw(`DELETE FROM parent_billing.family_billing_events AS event
		WHERE event.care_contract_id IS NULL AND event.occurred_at < ?0::date
			AND NOT EXISTS (SELECT 1 FROM parent_billing.family_billing AS family
				WHERE family.organization_id = event.organization_id AND family.id = event.family_billing_id)`, cutoff).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("parent billing postgres: delete the expired trail of family debtor numbers: %w", err)
	}
	events, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	children, err := deleteExpiredChildNumbers(ctx, db, cutoff, today)
	if err != nil {
		return 0, err
	}
	return families + int(events) + children, nil
}

func familiesOfRows(rows []familyBillingRow) []parentbilling.FamilyBilling {
	families := make([]parentbilling.FamilyBilling, len(rows))
	for i, row := range rows {
		families[i] = row.family()
	}
	return families
}
