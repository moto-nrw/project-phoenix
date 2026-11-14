package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const emailFamilyDepartureVersion = "20261114093230"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     emailFamilyDepartureVersion,
		Description: "Zustellung: Familien ohne Kind in Betreuung fallen aus den Verteilern und von der Sperrliste der Schule (#5960)",
		DependsOn:   []string{emailAddressMarksVersion}, // "1.15.772": the two views this one redefines
	})
	Migrations.MustRegister(emailFamilyDepartureUp, emailFamilyDepartureDown)
}

// emailFamilyDepartureUpSQL (#5960):
//
//   - platform.delivery_guardian_contacts gets child_in_care: the person has a
//     child at the school whose membership is pending or active and whose last
//     care day has not passed in Berlin. This also excludes children before the
//     next status tick marks them inactive. Inactive and alumnus never count.
//     The column is appended, so CREATE OR REPLACE keeps the view's dependents.
//     The CTE is inlined so callers' school filters reach the guardian profiles.
//   - platform.delivery_known_email_addresses counts a guardian's address only
//     while the guardian has a child in care, now with the account's address
//     too. An account's school membership alone no longer holds the address when
//     the account belongs to a guardian of the school and to no staff member of
//     it: the parent's membership outlives the child. Open applications still
//     hold the address, so a family that applies again keeps its mark.
const emailFamilyDepartureUpSQL = `
		CREATE OR REPLACE VIEW platform.delivery_guardian_contacts
		WITH (security_invoker = true, security_barrier = true) AS
		WITH guardian AS NOT MATERIALIZED (
			SELECT profile.tenant_id, profile.id, profile.email, profile.account_id, profile.emails_stopped_at,
				EXISTS (SELECT 1 FROM users.student_guardian_relationships AS link
					INNER JOIN users.student_school_memberships AS membership
						ON membership.tenant_id = link.tenant_id AND membership.student_profile_id = link.student_id
						AND membership.deleted_at IS NULL
					WHERE link.tenant_id = profile.tenant_id AND link.guardian_profile_id = profile.id
						AND membership.status IN ('pending', 'active')
						AND (membership.enrolled_until IS NULL
							OR membership.enrolled_until >= (CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date)) AS child_in_care
			FROM users.guardian_profiles AS profile)
		SELECT guardian.tenant_id, guardian.id AS guardian_profile_id, 'profile'::text AS source,
			LOWER(TRIM(guardian.email)) AS address, guardian.emails_stopped_at, guardian.child_in_care
		FROM guardian
		UNION ALL
		SELECT guardian.tenant_id, guardian.id, 'account'::text,
			LOWER(TRIM(account.email)), guardian.emails_stopped_at, guardian.child_in_care
		FROM guardian
		INNER JOIN auth.accounts AS account ON account.id = guardian.account_id
		WHERE account.email IS NOT NULL;
		CREATE OR REPLACE VIEW platform.delivery_known_email_addresses
		WITH (security_invoker = true, security_barrier = true) AS
		SELECT contact.tenant_id, contact.address
		FROM platform.delivery_guardian_contacts AS contact
		WHERE contact.address IS NOT NULL AND contact.child_in_care
		UNION
		SELECT account_tenant.tenant_id, LOWER(TRIM(account.email)) AS address
		FROM auth.account_tenants AS account_tenant
		INNER JOIN auth.accounts AS account ON account.id = account_tenant.account_id
		WHERE account.email IS NOT NULL AND (
			NOT EXISTS (SELECT 1 FROM users.guardian_profiles AS guardian
				WHERE guardian.tenant_id = account_tenant.tenant_id AND guardian.account_id = account_tenant.account_id)
			OR EXISTS (SELECT 1 FROM users.staff_school_memberships AS staff
				INNER JOIN users.persons AS person ON person.tenant_id = staff.tenant_id AND person.id = staff.person_id
				WHERE staff.tenant_id = account_tenant.tenant_id AND staff.deleted_at IS NULL
					AND person.account_id = account_tenant.account_id))
		UNION
		SELECT request.tenant_id, LOWER(TRIM(request.guardian_email)) AS address
		FROM enrollment.requests AS request
		WHERE request.withdrawn_at IS NULL AND EXISTS (
			SELECT 1 FROM enrollment.request_children AS child
			WHERE child.tenant_id = request.tenant_id AND child.request_id = request.id
				AND child.status IN ('submitted', 'under_review', 'waitlisted', 'pending_renewal', 'pending_admin_review'))
		UNION
		SELECT guardian.tenant_id, LOWER(TRIM(guardian.email)) AS address
		FROM enrollment.request_guardians AS guardian
		INNER JOIN enrollment.requests AS request ON request.id = guardian.request_id AND request.tenant_id = guardian.tenant_id
		WHERE guardian.email IS NOT NULL AND request.withdrawn_at IS NULL AND EXISTS (
			SELECT 1 FROM enrollment.request_children AS child
			WHERE child.tenant_id = request.tenant_id AND child.request_id = request.id
				AND child.status IN ('submitted', 'under_review', 'waitlisted', 'pending_renewal', 'pending_admin_review'));
		COMMENT ON TABLE platform.email_suppressions IS
			'Adressen einer Schule mit endgültigem Fehler oder Beschwerde (#5932, #5934). Die Markierung bleibt ohne feste Frist, bis jemand an der Schule die Adresse geprüft hat oder niemand an der Schule sie mehr führt; eine Familie ohne Kind in Betreuung führt sie nicht mehr (#5960).';
`

// emailFamilyDepartureDownSQL restores the views of 1.15.772. A view cannot
// lose a column by CREATE OR REPLACE, and delivery_known_email_addresses now
// reads delivery_guardian_contacts, so both go and come back with their grants.
// platform.delivery_traeger_email_holds names the contacts only in its body,
// which PostgreSQL does not track as a dependency.
const emailFamilyDepartureDownSQL = `
		DROP VIEW platform.delivery_known_email_addresses;
		DROP VIEW platform.delivery_guardian_contacts;
		CREATE VIEW platform.delivery_guardian_contacts
		WITH (security_invoker = true, security_barrier = true) AS
		SELECT guardian.tenant_id, guardian.id AS guardian_profile_id, 'profile'::text AS source,
			LOWER(TRIM(guardian.email)) AS address, guardian.emails_stopped_at
		FROM users.guardian_profiles AS guardian
		UNION ALL
		SELECT guardian.tenant_id, guardian.id, 'account'::text,
			LOWER(TRIM(account.email)), guardian.emails_stopped_at
		FROM users.guardian_profiles AS guardian
		INNER JOIN auth.accounts AS account ON account.id = guardian.account_id
		WHERE account.email IS NOT NULL;
		REVOKE ALL ON platform.delivery_guardian_contacts FROM PUBLIC, phoenix_tenant;
		GRANT SELECT ON platform.delivery_guardian_contacts TO phoenix_tenant;
		CREATE VIEW platform.delivery_known_email_addresses
		WITH (security_invoker = true, security_barrier = true) AS
		SELECT guardian.tenant_id, LOWER(guardian.email) AS address
		FROM users.guardian_profiles AS guardian
		WHERE guardian.email IS NOT NULL
		UNION
		SELECT account_tenant.tenant_id, LOWER(account.email) AS address
		FROM auth.account_tenants AS account_tenant
		INNER JOIN auth.accounts AS account ON account.id = account_tenant.account_id
		WHERE account.email IS NOT NULL
		UNION
		SELECT request.tenant_id, LOWER(TRIM(request.guardian_email)) AS address
		FROM enrollment.requests AS request
		WHERE request.withdrawn_at IS NULL AND EXISTS (
			SELECT 1 FROM enrollment.request_children AS child
			WHERE child.tenant_id = request.tenant_id AND child.request_id = request.id
				AND child.status IN ('submitted', 'under_review', 'waitlisted', 'pending_renewal', 'pending_admin_review'))
		UNION
		SELECT guardian.tenant_id, LOWER(TRIM(guardian.email)) AS address
		FROM enrollment.request_guardians AS guardian
		INNER JOIN enrollment.requests AS request ON request.id = guardian.request_id AND request.tenant_id = guardian.tenant_id
		WHERE guardian.email IS NOT NULL AND request.withdrawn_at IS NULL AND EXISTS (
			SELECT 1 FROM enrollment.request_children AS child
			WHERE child.tenant_id = request.tenant_id AND child.request_id = request.id
				AND child.status IN ('submitted', 'under_review', 'waitlisted', 'pending_renewal', 'pending_admin_review'));
		REVOKE ALL ON platform.delivery_known_email_addresses FROM PUBLIC, phoenix_tenant;
		GRANT SELECT ON platform.delivery_known_email_addresses TO phoenix_tenant;
		COMMENT ON TABLE platform.email_suppressions IS
			'Adressen einer Schule mit endgültigem Fehler oder Beschwerde (#5932, #5934). Die Markierung bleibt ohne feste Frist, bis jemand an der Schule die Adresse geprüft hat oder niemand an der Schule sie mehr führt.';
`

func emailFamilyDepartureUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, emailFamilyDepartureUpSQL); err != nil {
			return fmt.Errorf("let departed families drop out of the school's mail: %w", err)
		}
		return nil
	})
}

func emailFamilyDepartureDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, emailFamilyDepartureDownSQL); err != nil {
			return fmt.Errorf("restore the delivery views of 1.15.772: %w", err)
		}
		return nil
	})
}
