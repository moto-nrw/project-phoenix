package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// The Träger-Büro's edit of the details of a registration (#5629). The
// caller holds the lock of the registration; every statement names the
// Träger and the registration once more. Bookings, consents and the sealed
// health information are not touched.

const (
	updateHolidayRegistrationDetailsQuery = `UPDATE enrollment.holiday_registrations SET
			guardian_first_name = ?, guardian_last_name = ?, guardian_email = ?, guardian_phone = ?,
			second_guardian_first_name = NULLIF(?, ''), second_guardian_last_name = NULLIF(?, ''),
			second_guardian_email = NULLIF(?, ''), second_guardian_phone = NULLIF(?, ''),
			guardian_address_street = NULLIF(?, ''), guardian_address_postal_code = NULLIF(?, ''),
			guardian_address_city = NULLIF(?, ''),
			custom_data = ?::jsonb
		WHERE organization_id = ? AND id = ?`
	// updateHolidayChildDetailsQuery leaves the school alone: the check of
	// the school runs on every write of home_school_id, and a school the
	// Träger has deleted since stays the child's school.
	updateHolidayChildDetailsQuery = `UPDATE enrollment.holiday_registration_children SET
			first_name = ?, last_name = ?, birth_date = ?::date, guest_school_name = ?, custom_data = ?::jsonb
		WHERE organization_id = ? AND holiday_registration_id = ? AND id = ?`
	// A changed school drops the known holiday care wish (#4703): it was the
	// wish at the other school, and a holiday guest has none.
	updateHolidayChildDetailsAndSchoolQuery = `UPDATE enrollment.holiday_registration_children SET
			first_name = ?, last_name = ?, birth_date = ?::date, guest_school_name = ?, custom_data = ?::jsonb,
			home_school_id = NULLIF(?::bigint, 0), holiday_wish = NULL, holiday_wish_source = NULL
		WHERE organization_id = ? AND holiday_registration_id = ? AND id = ?`
	deleteHolidayContactsQuery = `DELETE FROM enrollment.holiday_registration_contacts
		WHERE organization_id = ? AND holiday_registration_id = ?`
)

// updateHolidayChild writes one changed child; the school only when it
// changed.
func updateHolidayChild(ctx context.Context, db bun.IDB, organizationID, registrationID, childID int64,
	child enrollment.HolidayChildInput, answers string, schoolChanged bool) error {
	if schoolChanged {
		_, err := db.NewRaw(updateHolidayChildDetailsAndSchoolQuery, child.FirstName, child.LastName, child.BirthDate.String(),
			child.GuestSchoolName, answers, child.HomeSchoolID, organizationID, registrationID, childID).Exec(ctx)
		return err
	}
	_, err := db.NewRaw(updateHolidayChildDetailsQuery, child.FirstName, child.LastName, child.BirthDate.String(),
		child.GuestSchoolName, answers, organizationID, registrationID, childID).Exec(ctx)
	return err
}

// WriteHolidayRegistrationDetails writes the registration, one statement per
// changed child, the school only when it changed, and replaces the
// contacts: delete, then one insert for all.
func (s *HolidayRegistrationStore) WriteHolidayRegistrationDetails(ctx context.Context, organizationID, registrationID int64, details enrollment.HolidayRegistrationInput, childIDs []int64, schoolChanged []bool) error {
	if len(childIDs) != len(details.Children) || len(schoolChanged) != len(details.Children) {
		return errors.New("enrollment postgres: the child IDs do not match the children")
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	answers, err := encodeAnswers(details.CustomData)
	if err != nil {
		return err
	}
	guardian, second, address := details.Guardian, enrollment.HolidayGuardianInput{}, enrollment.HolidayAddress{}
	if details.SecondGuardian != nil {
		second = *details.SecondGuardian
	}
	if details.GuardianAddress != nil {
		address = *details.GuardianAddress
	}
	if _, err := db.NewRaw(updateHolidayRegistrationDetailsQuery, guardian.FirstName, guardian.LastName, guardian.Email, guardian.Phone,
		second.FirstName, second.LastName, second.Email, second.Phone, address.Street, address.PostalCode, address.City,
		answers, organizationID, registrationID).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: update holiday registration details: %w", err)
	}
	for i, child := range details.Children {
		childAnswers, err := encodeAnswers(child.CustomData)
		if err != nil {
			return err
		}
		if err := updateHolidayChild(ctx, db, organizationID, registrationID, childIDs[i], child, childAnswers, schoolChanged[i]); err != nil {
			return fmt.Errorf("enrollment postgres: update holiday registration child details: %w", err)
		}
	}
	if _, err := db.NewRaw(deleteHolidayContactsQuery, organizationID, registrationID).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: remove holiday registration contacts: %w", err)
	}
	return insertHolidayContacts(ctx, db, organizationID, registrationID, details)
}
