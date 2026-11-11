package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayHostOwnContactsVersion = "1.15.923"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayHostOwnContactsVersion,
		Description: "Ferienwochen: die ausrichtende OGS liest Notfall und Abholung aus der Anmeldung ihrer eigenen Kinder (#6678)",
		DependsOn: []string{
			holidayWeekCapacityVersion,        // enrollment.holiday_host_guest_contacts as it stands
			dataReturnErasureOrderLockVersion, // the newest on development (1.15.919)
		},
	})
	Migrations.MustRegister(holidayHostOwnContactsUp, holidayHostOwnContactsDown)
}

// holidayHostOwnContactsUpSQL lets the school that hosts a holiday week read
// the emergency contacts and the persons allowed to collect its own children
// in that week (#6678, ADR 0120 addendum). Until now only holiday guests
// brought them: a child of the school is its student already, so its contacts
// stayed with the Träger. The team cares for both kinds of children in the
// same week and needs who to call and who may collect for both; what the
// parents wrote for the holiday week may differ from the school's master data.
//
// The function sits beside enrollment.holiday_host_guest_contacts and mirrors
// it: it answers in the tenant transaction of the hosting school only, for
// confirmed registrations, for weeks the school hosts until their last day
// (enrollment.holiday_host_week), and only for children whose registration
// names this school as their home school. The registering parent comes first,
// then the second guardian, then the contacts of the registration. No health
// information, no e-mail address, no consent. Nothing is copied: the rows go
// with the registration, and the access ends with the week.
const holidayHostOwnContactsUpSQL = `
CREATE FUNCTION enrollment.holiday_host_own_contacts(p_organization_id BIGINT, p_week_id BIGINT)
	RETURNS TABLE (child_id BIGINT, kind TEXT, "position" SMALLINT, name TEXT, phone TEXT, relationship TEXT)
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
	WITH own AS (
		SELECT child.id AS child_id, registration.id AS registration_id, registration.guardian_first_name,
			registration.guardian_last_name, registration.guardian_phone, registration.second_guardian_first_name,
			registration.second_guardian_last_name, registration.second_guardian_phone
		FROM enrollment.holiday_registration_bookings AS booking
		JOIN enrollment.holiday_registration_children AS child
			ON child.organization_id = booking.organization_id AND child.id = booking.holiday_registration_child_id
		JOIN enrollment.holiday_registrations AS registration
			ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
		WHERE booking.organization_id = p_organization_id AND booking.holiday_period_week_id = p_week_id
			AND registration.status = 'confirmed'
			AND child.home_school_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
			AND enrollment.holiday_host_booking(booking.organization_id, booking.holiday_period_week_id, child.id)
			AND enrollment.holiday_host_week(p_organization_id, p_week_id)
	)
	SELECT own.child_id, 'emergency'::text, 0::smallint,
		own.guardian_first_name || ' ' || own.guardian_last_name, own.guardian_phone, 'Eltern'::text
	FROM own
	UNION ALL
	SELECT own.child_id, 'emergency'::text, 1::smallint,
		own.second_guardian_first_name || ' ' || own.second_guardian_last_name,
		COALESCE(own.second_guardian_phone, ''), 'Eltern'::text
	FROM own
	WHERE own.second_guardian_first_name IS NOT NULL
	UNION ALL
	SELECT own.child_id, contact.kind::text, (contact.position + 1)::smallint, contact.name, contact.phone, contact.relationship
	FROM own
	JOIN enrollment.holiday_registration_contacts AS contact
		ON contact.organization_id = p_organization_id AND contact.holiday_registration_id = own.registration_id
	$$;
COMMENT ON FUNCTION enrollment.holiday_host_own_contacts(BIGINT, BIGINT) IS
	'Emergency and pickup contacts of the confirmed children of the school of app.current_tenant_id itself in a week it hosts (#6678, ADR 0120 addendum).';
REVOKE ALL ON FUNCTION enrollment.holiday_host_own_contacts(BIGINT, BIGINT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION enrollment.holiday_host_own_contacts(BIGINT, BIGINT) TO phoenix_tenant;
`

const holidayHostOwnContactsDownSQL = `DROP FUNCTION IF EXISTS enrollment.holiday_host_own_contacts(BIGINT, BIGINT);`

func holidayHostOwnContactsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayHostOwnContactsUpSQL); err != nil {
		return fmt.Errorf("add holiday host own contacts: %w", err)
	}
	return nil
}

func holidayHostOwnContactsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayHostOwnContactsDownSQL); err != nil {
		return fmt.Errorf("drop holiday host own contacts: %w", err)
	}
	return nil
}
