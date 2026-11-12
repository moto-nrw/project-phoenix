package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// Reads for the files of the holiday registrations (#6691): the contacts and
// the sealed health information of a whole period, or of the one
// registration registrationID names, in one statement each, and the health
// reads of many registrations in one insert.

const (
	recordCriteriaQuery = `SELECT DISTINCT criterion.id, criterion.name, criterion.question, criterion.scope,
			(criterion.active AND criterion.archived_at IS NULL) AS active, criterion.position,
			(answer.criterion_id IS NOT NULL) AS answered
		FROM enrollment.holiday_allocation_criteria AS criterion
		LEFT JOIN enrollment.holiday_registration_criteria AS answer
			ON answer.organization_id = criterion.organization_id AND answer.criterion_id = criterion.id
			AND answer.holiday_period_id = ? AND (?::bigint = 0 OR answer.holiday_registration_id = ?)
		WHERE criterion.organization_id = ?
			AND ((criterion.active AND criterion.archived_at IS NULL) OR answer.criterion_id IS NOT NULL)
		ORDER BY criterion.position, criterion.id`
	recordContactsQuery = `SELECT contact.holiday_registration_id, contact.kind, contact.name, contact.phone, contact.relationship
		FROM enrollment.holiday_registration_contacts AS contact
		JOIN enrollment.holiday_registrations AS registration
			ON registration.organization_id = contact.organization_id AND registration.id = contact.holiday_registration_id
		WHERE contact.organization_id = ? AND registration.holiday_period_id = ? AND (?::bigint = 0 OR registration.id = ?)
		ORDER BY contact.holiday_registration_id, contact.kind, contact.position`
	recordFamilyHealthQuery = `SELECT id, health_data FROM enrollment.holiday_registrations
		WHERE organization_id = ? AND holiday_period_id = ? AND (?::bigint = 0 OR id = ?)`
	recordChildrenHealthQuery = `SELECT id, holiday_registration_id, position, health_data
		FROM enrollment.holiday_registration_children
		WHERE organization_id = ? AND holiday_period_id = ? AND (?::bigint = 0 OR holiday_registration_id = ?)
		ORDER BY holiday_registration_id, position`
	insertHolidayHealthReadsQuery = `INSERT INTO enrollment.holiday_registration_audit
			(organization_id, holiday_period_id, holiday_registration_id, action, details, actor_account_id)
		SELECT ?, ?, registration.id, 'health', ?::jsonb, NULLIF(?::bigint, 0)
		FROM unnest(?::bigint[]) AS registration(id)`
)

// ReadHolidayRecordCriteria reads current criteria and historical criteria
// answered by the selected registrations, without excluding archived ones.
func (s *HolidayRegistrationStore) ReadHolidayRecordCriteria(ctx context.Context, organizationID, periodID, registrationID int64) ([]enrollment.HolidayCriterion, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []holidayCriterionRow
	if err := db.NewRaw(recordCriteriaQuery, periodID, registrationID, registrationID, organizationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday record criteria: %w", err)
	}
	criteria := make([]enrollment.HolidayCriterion, len(rows))
	for i, row := range rows {
		criteria[i] = enrollment.HolidayCriterion(row)
	}
	return criteria, nil
}

// ReadHolidayRecordContacts reads the contacts of the registrations of a
// period by registration, or of the one registrationID names.
func (s *HolidayRegistrationStore) ReadHolidayRecordContacts(ctx context.Context, organizationID, periodID, registrationID int64) (map[int64][]enrollment.HolidayRegistrationContact, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		RegistrationID int64  `bun:"holiday_registration_id"`
		Kind           string `bun:"kind"`
		Name           string `bun:"name"`
		Phone          string `bun:"phone"`
		Relationship   string `bun:"relationship"`
	}
	if err := db.NewRaw(recordContactsQuery, organizationID, periodID, registrationID, registrationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday record contacts: %w", err)
	}
	contacts := map[int64][]enrollment.HolidayRegistrationContact{}
	for _, row := range rows {
		contacts[row.RegistrationID] = append(contacts[row.RegistrationID], enrollment.HolidayRegistrationContact{
			Kind: row.Kind, Name: row.Name, Phone: row.Phone, Relationship: row.Relationship,
		})
	}
	return contacts, nil
}

// ReadHolidayRecordHealth reads the sealed health information of the
// registrations of a period by registration, or of the one registrationID
// names: two statements for any number of registrations.
func (s *HolidayRegistrationStore) ReadHolidayRecordHealth(ctx context.Context, organizationID, periodID, registrationID int64) (map[int64]enrollment.HolidaySealedRegistrationHealth, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var families []struct {
		ID         int64  `bun:"id"`
		HealthData []byte `bun:"health_data"`
	}
	if err := db.NewRaw(recordFamilyHealthQuery, organizationID, periodID, registrationID, registrationID).Scan(ctx, &families); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read sealed holiday family health: %w", err)
	}
	var children []struct {
		ID             int64  `bun:"id"`
		RegistrationID int64  `bun:"holiday_registration_id"`
		Position       int    `bun:"position"`
		HealthData     []byte `bun:"health_data"`
	}
	if err := db.NewRaw(recordChildrenHealthQuery, organizationID, periodID, registrationID, registrationID).Scan(ctx, &children); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read sealed holiday child health: %w", err)
	}
	sealed := make(map[int64]enrollment.HolidaySealedRegistrationHealth, len(families))
	for _, family := range families {
		sealed[family.ID] = enrollment.HolidaySealedRegistrationHealth{Family: family.HealthData}
	}
	for _, child := range children {
		registration := sealed[child.RegistrationID]
		registration.Children = append(registration.Children, enrollment.HolidaySealedChildHealth{
			ChildID: child.ID, Position: child.Position, Sealed: child.HealthData,
		})
		sealed[child.RegistrationID] = registration
	}
	return sealed, nil
}

// AppendHolidayHealthReads records a read of the health information of
// every registration named, in one statement.
func (s *HolidayRegistrationStore) AppendHolidayHealthReads(ctx context.Context, organizationID, periodID int64, registrationIDs []int64, details string, actorAccountID int64) error {
	if len(registrationIDs) == 0 {
		return nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(insertHolidayHealthReadsQuery, organizationID, periodID, details, actorAccountID,
		pgdialect.Array(registrationIDs)).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: append holiday health reads: %w", err)
	}
	return nil
}
