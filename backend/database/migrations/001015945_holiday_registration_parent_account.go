package migrations

import (
	"context"
	"fmt"
	"slices"

	"github.com/uptrace/bun"
)

const holidayRegistrationParentAccountVersion = "1.15.945"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayRegistrationParentAccountVersion,
		Description: "Ferienanmeldung: Elternkonto an der Anmeldung und Angebot eines Kontos per Mail (#6673, ADR 0216)",
		DependsOn: []string{
			traegerParentAccessVersion,   // the newest migration on this branch (1.15.944)
			holidayAcceptanceMailVersion, // the outbox CHECK and the erasure this one extends (1.15.927)
		},
	})
	Migrations.MustRegister(holidayRegistrationParentAccountUp, holidayRegistrationParentAccountDown)
}

// holidayRegistrationParentAccountUpSQL links a holiday registration to the
// parent account it belongs to (ADR 0216 points 1 to 4 and 10):
//
//   - parent_account_id names the account; ON DELETE SET NULL keeps the
//     registration when the account goes. The link lives in the registration
//     row and goes with it when the retention deletes it (ADR 0118).
//   - parent_account_requested records that the parent asked for an account
//     at the end of the form; the offer mail went out with the registration.
//   - idx_holiday_registrations_parent_account serves the read of the
//     account's registrations, idx_holiday_registrations_unclaimed_email the
//     claim by the account's address of the registrations without an
//     account.
const holidayRegistrationParentAccountUpSQL = `
ALTER TABLE enrollment.holiday_registrations
	ADD COLUMN parent_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	ADD COLUMN parent_account_requested BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX idx_holiday_registrations_parent_account ON enrollment.holiday_registrations (parent_account_id, submitted_at DESC)
	WHERE parent_account_id IS NOT NULL;
CREATE INDEX idx_holiday_registrations_unclaimed_email ON enrollment.holiday_registrations (LOWER(guardian_email))
	WHERE parent_account_id IS NULL;
COMMENT ON COLUMN enrollment.holiday_registrations.parent_account_id IS
	'Parent account the registration belongs to (ADR 0216): from the sealed proof of the prefill, or claimed by the account''s address; goes with the registration.';
COMMENT ON COLUMN enrollment.holiday_registrations.parent_account_requested IS
	'The parent asked for a parent account at the end of the form (ADR 0216); the offer mail went out with the registration.';
`

const holidayRegistrationParentAccountDownSQL = `
ALTER TABLE enrollment.holiday_registrations
	DROP COLUMN parent_account_requested,
	DROP COLUMN parent_account_id;
`

// The offer of a parent account (ADR 0216 point 4) is a mail of the Träger,
// so the CHECK of 1.15.927 declares its kind on top of the acceptance mail,
// and the demo erasure deletes it with the registration it names, like the
// receipt.
var holidayParentAccountOfferTraegerKinds = slices.Sorted(slices.Values(append(slices.Clone(holidayAcceptanceEmailOutboxTraegerKinds), "holiday_parent_account_offer")))

const holidayParentAccountOfferKindCheckSQL = `
ALTER TABLE platform.email_outbox DROP CONSTRAINT IF EXISTS chk_email_outbox_traeger_kind;
ALTER TABLE platform.email_outbox ADD CONSTRAINT chk_email_outbox_traeger_kind CHECK (
	organization_id IS NULL OR kind IN (
		'care_contract', 'care_contract_notice', 'holiday_closure_notice', 'holiday_parent_account_offer', 'holiday_place_granted',
		'holiday_registration_accepted', 'holiday_registration_received', 'parent_billing_letter')) NOT VALID;
COMMENT ON CONSTRAINT chk_email_outbox_traeger_kind ON platform.email_outbox IS
	'The mail kinds of a Träger. A new kind needs its demo erasure in platform.erase_closed_demo_visitors (#6342).';
`

// A rollback deletes the offers, which the CHECK of 1.15.927 refuses, and
// writes that CHECK back.
const deleteHolidayParentAccountOffersSQL = `DELETE FROM platform.email_outbox
	WHERE organization_id IS NOT NULL AND kind = 'holiday_parent_account_offer'`

// holidayParentAccountRestoredKindCheckSQL is the CHECK of 1.15.927, which a
// rollback writes back. The literals stay in this file so the architecture
// check reads the migration's SQL.
const holidayParentAccountRestoredKindCheckSQL = `
ALTER TABLE platform.email_outbox DROP CONSTRAINT IF EXISTS chk_email_outbox_traeger_kind;
ALTER TABLE platform.email_outbox ADD CONSTRAINT chk_email_outbox_traeger_kind CHECK (
	organization_id IS NULL OR kind IN (
		'care_contract', 'care_contract_notice', 'holiday_closure_notice', 'holiday_place_granted', 'holiday_registration_accepted',
		'holiday_registration_received', 'parent_billing_letter')) NOT VALID;
COMMENT ON CONSTRAINT chk_email_outbox_traeger_kind ON platform.email_outbox IS
	'The mail kinds of a Träger. A new kind needs its demo erasure in platform.erase_closed_demo_visitors (#6342).';
`

// holidayRegistrationAccountUnusedSQL is auth.account_is_unused of 1.15.944
// whose access at a live Träger also keeps the account past the 30 days of
// its setup link while a holiday registration at that Träger names the
// account or, not yet claimed, its address (ADR 0216 point 10): the family
// still has data there, as a guardian profile keeps a school parent's
// account. The registration goes with its retention (ADR 0118 addendum
// #6691), and with it this reason. The function already reads
// enrollment.requests; this clause reads enrollment.holiday_registrations
// the same way.
const holidayRegistrationAccountUnusedSQL = `
		CREATE OR REPLACE FUNCTION auth.account_is_unused(target_id BIGINT, target_email TEXT)
		RETURNS BOOLEAN
		LANGUAGE sql
		STABLE
		AS $$
			SELECT NOT EXISTS (
				SELECT 1 FROM auth.account_tenants AS membership
				WHERE membership.account_id = target_id
					AND (
						membership.status = 'pending'
						OR (
							membership.status = 'active'
							AND NOT EXISTS (
								SELECT 1 FROM auth.account_roles AS grant_row
								WHERE grant_row.account_id = membership.account_id
									AND grant_row.tenant_id = membership.tenant_id
							)
						)
						OR (
							membership.status = 'active'
							AND EXISTS (
								SELECT 1 FROM auth.account_roles AS grant_row
								JOIN auth.roles AS granted ON granted.id = grant_row.role_id
								WHERE grant_row.account_id = membership.account_id
									AND grant_row.tenant_id = membership.tenant_id
									AND LOWER(granted.name) <> 'guardian'
							)
						)
					)
			)
			AND NOT EXISTS (SELECT 1 FROM users.persons AS person WHERE person.account_id = target_id)
			AND NOT EXISTS (SELECT 1 FROM users.guardian_profiles AS guardian WHERE guardian.account_id = target_id)
			AND NOT EXISTS (SELECT 1 FROM enrollment.requests AS request WHERE request.guardian_account_id = target_id)
			AND NOT EXISTS (
				SELECT 1 FROM auth.invitation_tokens AS invitation
				WHERE LOWER(invitation.email) = LOWER(target_email)
					AND invitation.used_at IS NULL
					AND invitation.expires_at > statement_timestamp()
			)
			AND NOT EXISTS (
				SELECT 1 FROM auth.guardian_invitations AS invitation
				JOIN users.guardian_profiles AS invited ON invited.id = invitation.guardian_profile_id
				WHERE LOWER(invited.email) = LOWER(target_email)
					AND invitation.accepted_at IS NULL
					AND invitation.expires_at > statement_timestamp()
					AND invitation.approval_status <> 'rejected'
			)
			AND NOT EXISTS (
				SELECT 1 FROM auth.demo_accesses AS demo
				WHERE demo.account_id = target_id OR demo.parent_account_id = target_id
			)
			AND NOT EXISTS (
				SELECT 1 FROM platform.organization_accounts AS traeger
				WHERE traeger.account_id = target_id AND traeger.status IN ('pending', 'active')
			)
			AND NOT EXISTS (
				SELECT 1 FROM auth.caterer_accounts AS caterer_account
				JOIN auth.caterer_schools AS school
					ON school.caterer_id = caterer_account.caterer_id AND school.ended_at IS NULL
				WHERE caterer_account.account_id = target_id
			)
			AND NOT EXISTS (
				SELECT 1 FROM auth.caterer_invitations AS invitation
				WHERE LOWER(invitation.email) = LOWER(target_email)
					AND invitation.accepted_at IS NULL
					AND invitation.revoked_at IS NULL
					AND invitation.expires_at > statement_timestamp()
			)
			AND NOT EXISTS (
				SELECT 1 FROM auth.traeger_parent_access AS parent_access
				JOIN platform.organizations AS traeger ON traeger.id = parent_access.organization_id
				WHERE parent_access.account_id = target_id
					AND traeger.active AND traeger.deleted_at IS NULL
					AND (
						parent_access.created_at > statement_timestamp() - INTERVAL '30 days'
						OR EXISTS (
							SELECT 1 FROM enrollment.holiday_registrations AS registration
							WHERE registration.organization_id = parent_access.organization_id
								AND (registration.parent_account_id = target_id
									OR (registration.parent_account_id IS NULL AND LOWER(registration.guardian_email) = LOWER(target_email)))
						)
					)
			);
		$$;
		REVOKE ALL ON FUNCTION auth.account_is_unused(BIGINT, TEXT) FROM PUBLIC;
`

const validateHolidayParentAccountKindCheckSQL = `ALTER TABLE platform.email_outbox VALIDATE CONSTRAINT chk_email_outbox_traeger_kind`

func holidayRegistrationParentAccountUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayRegistrationParentAccountUpSQL); err != nil {
		return fmt.Errorf("link holiday registrations to parent accounts: %w", err)
	}
	// As in 1.15.887: the ADD holds the table only for the catalog change,
	// the VALIDATE scans it in a statement of its own.
	if _, err := db.ExecContext(ctx, holidayParentAccountOfferKindCheckSQL); err != nil {
		return fmt.Errorf("declare the parent account offer of a Träger: %w", err)
	}
	if _, err := db.ExecContext(ctx, validateHolidayParentAccountKindCheckSQL); err != nil {
		return fmt.Errorf("validate the mail kinds of a Träger: %w", err)
	}
	if err := installParentAccountOfferErasure(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, holidayRegistrationAccountUnusedSQL); err != nil {
		return fmt.Errorf("keep the accounts of linked holiday registrations: %w", err)
	}
	return nil
}

func holidayRegistrationParentAccountDown(ctx context.Context, db *bun.DB) error {
	// auth.account_is_unused of 1.15.944 names no column this rollback drops.
	if err := installTraegerParentAccessAccountUnused(ctx, db); err != nil {
		return err
	}
	if err := installHolidayAcceptanceErasure(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, deleteHolidayParentAccountOffersSQL); err != nil {
		return fmt.Errorf("delete the parent account offers: %w", err)
	}
	if _, err := db.ExecContext(ctx, holidayParentAccountRestoredKindCheckSQL); err != nil {
		return fmt.Errorf("restore the mail kinds of a Träger: %w", err)
	}
	if _, err := db.ExecContext(ctx, validateHolidayParentAccountKindCheckSQL); err != nil {
		return fmt.Errorf("validate the mail kinds of a Träger: %w", err)
	}
	if _, err := db.ExecContext(ctx, holidayRegistrationParentAccountDownSQL); err != nil {
		return fmt.Errorf("unlink holiday registrations from parent accounts: %w", err)
	}
	return nil
}
