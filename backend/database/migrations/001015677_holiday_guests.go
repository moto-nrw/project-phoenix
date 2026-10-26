package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayGuestsVersion = "1.15.677"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayGuestsVersion,
		Description: "Feriengäste erscheinen in der Feriengruppe und im Check-in der ausrichtenden OGS (#4666)",
		DependsOn: []string{
			holidayHostAccessVersion,           // the host policies and enrollment.holiday_host_week
			parentBillingRunExportEventVersion, // the last migration before this one
		},
	})
	Migrations.MustRegister(holidayGuestsUp, holidayGuestsDown)
}

// holidayGuestsUpSQL gives the school that hosts a holiday week its holiday
// guests (#4666, spec #4611 user stories 43 and 44, ADR 0120). A holiday
// guest is a child of another school of the Träger, or a child without a
// school of the Träger, whose booking for a week the school hosts is
// confirmed. The school copies it into its own tenant as a child whose care
// covers exactly that week, so its groups, its check-in and the kiosk work
// unchanged. These tables belong to the school's tenant:
//
//   - holiday_host_groups: the group of the school the guests of one hosted
//     week belong to, chosen by the school;
//   - holiday_guests: the copy of one confirmed booking, with the booked
//     time model (Betreuung), the child the copy created and the day its
//     access ended early (cancellation or rebooking);
//   - holiday_guest_contacts: the emergency contacts and the persons allowed
//     to collect the guest (Notfall und Abholung); a name holds up to 201
//     characters, the registering parent's first and last name (100 each)
//     with the space between them.
//
// The contacts of a confirmed booking leave the Träger only through
// enrollment.holiday_host_guest_contacts, which answers for the weeks the
// school hosts until their last day, and never for children of the school
// itself, who are its students already: the registering parent with name and
// phone as the first emergency contact, then the emergency contacts and the
// persons allowed to collect the child. The guest copies no health
// information, no consent, no e-mail address and no other data of the
// parents.
const holidayGuestsUpSQL = `
CREATE TABLE enrollment.holiday_host_groups (
	tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
	organization_id BIGINT NOT NULL,
	holiday_period_week_id BIGINT NOT NULL,
	group_id BIGINT NOT NULL,
	assigned_by_account_id BIGINT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (tenant_id, holiday_period_week_id),
	CONSTRAINT fk_holiday_host_groups_group FOREIGN KEY (tenant_id, group_id)
		REFERENCES education.groups (tenant_id, id) ON DELETE CASCADE
);
COMMENT ON TABLE enrollment.holiday_host_groups IS
	'Group of the hosting school the holiday guests of one hosted week belong to (#4666, ADR 0120).';

CREATE TABLE enrollment.holiday_guests (
	id BIGSERIAL PRIMARY KEY,
	tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
	organization_id BIGINT NOT NULL,
	holiday_period_week_id BIGINT NOT NULL,
	holiday_registration_child_id BIGINT NOT NULL,
	student_id BIGINT NOT NULL,
	starts_on DATE NOT NULL,
	ends_on DATE NOT NULL,
	ended_on DATE,
	home_school_id BIGINT,
	guest_school_name TEXT NOT NULL DEFAULT '',
	time_model_name TEXT NOT NULL,
	time_model_starts_at TEXT NOT NULL,
	time_model_ends_at TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_holiday_guests_tenant UNIQUE (tenant_id, id),
	CONSTRAINT uq_holiday_guests_booking UNIQUE (tenant_id, holiday_period_week_id, holiday_registration_child_id),
	CONSTRAINT uq_holiday_guests_student UNIQUE (tenant_id, student_id),
	CONSTRAINT fk_holiday_guests_student FOREIGN KEY (tenant_id, student_id)
		REFERENCES users.student_profiles (tenant_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_holiday_guests_days CHECK (starts_on <= ends_on),
	CONSTRAINT chk_holiday_guests_texts CHECK (char_length(guest_school_name) <= 120
		AND BTRIM(time_model_name) <> '' AND char_length(time_model_name) <= 80
		AND time_model_starts_at ~ '^[0-2][0-9]:[0-5][0-9]$' AND time_model_ends_at ~ '^[0-2][0-9]:[0-5][0-9]$')
);
CREATE INDEX idx_holiday_guests_week ON enrollment.holiday_guests (tenant_id, holiday_period_week_id);
COMMENT ON TABLE enrollment.holiday_guests IS
	'Holiday guest of the hosting school: copy of a confirmed booking of a child of another school or without a school, care limited to the booked week (#4666, ADR 0120).';

CREATE TABLE enrollment.holiday_guest_contacts (
	id BIGSERIAL PRIMARY KEY,
	tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
	holiday_guest_id BIGINT NOT NULL,
	kind VARCHAR(16) NOT NULL,
	position SMALLINT NOT NULL,
	name TEXT NOT NULL,
	phone TEXT NOT NULL DEFAULT '',
	relationship TEXT NOT NULL DEFAULT '',
	CONSTRAINT fk_holiday_guest_contacts_guest FOREIGN KEY (tenant_id, holiday_guest_id)
		REFERENCES enrollment.holiday_guests (tenant_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_holiday_guest_contacts_position UNIQUE (holiday_guest_id, kind, position),
	CONSTRAINT chk_holiday_guest_contacts_kind CHECK (kind IN ('emergency', 'pickup')),
	CONSTRAINT chk_holiday_guest_contacts_texts CHECK (
		BTRIM(name) <> '' AND char_length(name) <= 201 AND char_length(phone) <= 40 AND char_length(relationship) <= 60)
);
COMMENT ON TABLE enrollment.holiday_guest_contacts IS
	'Emergency contact or person allowed to collect a holiday guest, copied from its confirmed registration (#4666, ADR 0120).';

GRANT SELECT, INSERT, UPDATE, DELETE ON enrollment.holiday_host_groups, enrollment.holiday_guests,
	enrollment.holiday_guest_contacts TO phoenix_tenant;
GRANT USAGE ON SEQUENCE enrollment.holiday_guests_id_seq, enrollment.holiday_guest_contacts_id_seq TO phoenix_tenant;

CREATE FUNCTION enrollment.holiday_host_guest_contacts(p_organization_id BIGINT, p_week_id BIGINT)
	RETURNS TABLE (child_id BIGINT, kind TEXT, "position" SMALLINT, name TEXT, phone TEXT, relationship TEXT)
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
	WITH guest AS (
		SELECT child.id AS child_id, registration.id AS registration_id, registration.guardian_first_name,
			registration.guardian_last_name, registration.guardian_phone
		FROM enrollment.holiday_registration_bookings AS booking
		JOIN enrollment.holiday_registration_children AS child
			ON child.organization_id = booking.organization_id AND child.id = booking.holiday_registration_child_id
		JOIN enrollment.holiday_registrations AS registration
			ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
		WHERE booking.organization_id = p_organization_id AND booking.holiday_period_week_id = p_week_id
			AND registration.status = 'confirmed'
			AND child.home_school_id IS DISTINCT FROM NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
			AND enrollment.holiday_host_week(p_organization_id, p_week_id)
	)
	SELECT guest.child_id, 'emergency'::text, 0::smallint,
		guest.guardian_first_name || ' ' || guest.guardian_last_name, guest.guardian_phone, 'Eltern'::text
	FROM guest
	UNION ALL
	SELECT guest.child_id, contact.kind::text, contact.position, contact.name, contact.phone, contact.relationship
	FROM guest
	JOIN enrollment.holiday_registration_contacts AS contact
		ON contact.organization_id = p_organization_id AND contact.holiday_registration_id = guest.registration_id
	$$;
COMMENT ON FUNCTION enrollment.holiday_host_guest_contacts(BIGINT, BIGINT) IS
	'Emergency and pickup contacts of the confirmed holiday guests of a week the school of app.current_tenant_id hosts (#4666, ADR 0120); none for its own children.';
REVOKE ALL ON FUNCTION enrollment.holiday_host_guest_contacts(BIGINT, BIGINT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION enrollment.holiday_host_guest_contacts(BIGINT, BIGINT) TO phoenix_tenant;
`

const holidayGuestsDownSQL = `
DROP FUNCTION IF EXISTS enrollment.holiday_host_guest_contacts(BIGINT, BIGINT);
DROP TABLE IF EXISTS enrollment.holiday_guest_contacts;
DROP TABLE IF EXISTS enrollment.holiday_guests;
DROP TABLE IF EXISTS enrollment.holiday_host_groups;
`

func holidayGuestsUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, holidayGuestsUpSQL); err != nil {
			return fmt.Errorf("add holiday guests: %w", err)
		}
		if err := provisionTenantRLS(ctx, tx, "enrollment.holiday_host_groups", "enrollment.holiday_guests",
			"enrollment.holiday_guest_contacts"); err != nil {
			return fmt.Errorf("provision holiday guest tenant isolation: %w", err)
		}
		return nil
	})
}

func holidayGuestsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayGuestsDownSQL); err != nil {
		return fmt.Errorf("drop holiday guests: %w", err)
	}
	return nil
}
