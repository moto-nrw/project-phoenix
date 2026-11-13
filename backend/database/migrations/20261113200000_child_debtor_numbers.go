package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const childDebtorNumbersVersion = "20261113200000"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     childDebtorNumbersVersion,
		Description: "Elternabrechnung: Debitorennummer je Kind, wenn der Träger sie so führt (#7007)",
		DependsOn: []string{
			familyDebtorNumberVersion,                    // the family numbers and their retention (1.15.948)
			demoVisitorErasureFamilyDebtorNumbersVersion, // the erasure of the demo families this one extends (1.15.951)
			parentBillingPaymentTermsVersion,             // parent_billing.family_billing
			holidayRegistrationRetentionDaysVersion,      // enrollment.holiday_registrations the erasure reads
			careContractDraftsVersion,                    // care_contract.contracts the erasure reads
		},
	})
	Migrations.MustRegister(childDebtorNumbersUp, childDebtorNumbersDown)
}

// childDebtorNumbersUpSQL keeps the debtor number of a child of a Träger
// that keeps one per child (#7007, parent_billing.debtor_number_scope):
//
//   - family_email is the family of the child, the recipient of its
//     invoices (parentbilling.FamilyKey), as parent_billing.family_billing.
//   - child_key is the trimmed first and last name without case and the date
//     of birth (parentbilling.ChildKey); the child of a contract and the same
//     child at a holiday registration share it. child_first_name,
//     child_last_name and child_birth_date keep the names as entered for the
//     list of families.
//   - last_used_on is the latest day a number was saved or an invoice took
//     it. The Beleg retention deletes a row on 1 January of the fourth year
//     after it, unless a document of the family is still kept, like a family
//     row without a contract (#6692): the same instruction and deletion stop
//     of the Träger for its holiday documents, logged in beleg_deletions as
//     a family_debtor_number.
//
// The trail names a number only by its ID. Both go with their Träger. The
// erasure of a retired demo school deletes the numbers of its families with
// the families (platform.erase_demo_visitor_families, a repeatable
// definition since this migration).
const childDebtorNumbersUpSQL = `
CREATE TABLE parent_billing.child_debtor_numbers (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	family_email TEXT NOT NULL,
	child_key TEXT NOT NULL,
	child_first_name TEXT NOT NULL,
	child_last_name TEXT NOT NULL,
	child_birth_date DATE,
	debtor_number TEXT NOT NULL,
	last_used_on DATE NOT NULL,
	updated_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_child_debtor_numbers_organization UNIQUE (organization_id, id),
	CONSTRAINT uq_child_debtor_numbers_child UNIQUE (organization_id, family_email, child_key),
	CONSTRAINT chk_child_debtor_numbers_email CHECK (
		family_email = LOWER(BTRIM(family_email)) AND family_email <> '' AND char_length(family_email) <= 320),
	CONSTRAINT chk_child_debtor_numbers_key CHECK (child_key <> '' AND char_length(child_key) <= 220),
	CONSTRAINT chk_child_debtor_numbers_names CHECK (BTRIM(child_first_name) <> '' AND BTRIM(child_last_name) <> ''
		AND char_length(child_first_name) <= 100 AND char_length(child_last_name) <= 100),
	CONSTRAINT chk_child_debtor_numbers_number CHECK (BTRIM(debtor_number) <> '' AND char_length(debtor_number) <= 35)
);
CREATE INDEX idx_child_debtor_numbers_last_used ON parent_billing.child_debtor_numbers (last_used_on);
COMMENT ON TABLE parent_billing.child_debtor_numbers IS
	'Debtor number of one child of a family of a Träger that keeps one per child (#7007); goes with the Beleg retention like a family number without a contract.';

CREATE TABLE parent_billing.child_debtor_number_events (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	child_debtor_number_id BIGINT NOT NULL,
	action VARCHAR(16) NOT NULL,
	actor_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT chk_child_debtor_number_events_action CHECK (action IN ('read', 'save', 'delete'))
);
CREATE INDEX idx_child_debtor_number_events_number
	ON parent_billing.child_debtor_number_events (organization_id, child_debtor_number_id, occurred_at);
CREATE INDEX idx_child_debtor_number_events_occurred ON parent_billing.child_debtor_number_events (occurred_at);
COMMENT ON TABLE parent_billing.child_debtor_number_events IS
	'Trail of every read and change of the debtor number of a child (#7007): which one, who and when; no names. Goes after its row and three years.';

ALTER TABLE parent_billing.child_debtor_numbers ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.child_debtor_numbers FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_child_debtor_numbers ON parent_billing.child_debtor_numbers FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.child_debtor_number_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.child_debtor_number_events FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_child_debtor_number_events ON parent_billing.child_debtor_number_events FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.child_debtor_numbers, parent_billing.child_debtor_number_events FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE, DELETE ON parent_billing.child_debtor_numbers TO phoenix_tenant;
GRANT SELECT, INSERT ON parent_billing.child_debtor_number_events TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.child_debtor_numbers_id_seq, parent_billing.child_debtor_number_events_id_seq
	TO phoenix_tenant;
-- The Beleg retention deletes expired numbers across every Träger.
GRANT ALL ON parent_billing.child_debtor_numbers, parent_billing.child_debtor_number_events TO phoenix_admin;
`

const childDebtorNumbersDownSQL = `
DROP TABLE IF EXISTS parent_billing.child_debtor_number_events;
DROP TABLE IF EXISTS parent_billing.child_debtor_numbers;
`

func childDebtorNumbersUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, childDebtorNumbersUpSQL); err != nil {
		return fmt.Errorf("create the debtor numbers of the children: %w", err)
	}
	return nil
}

// childDebtorNumbersDown first puts back the erasure of the demo families of
// 1.15.951, which does not read the tables, and then drops them: the
// repeatable definition reads them and would fail at its next run.
func childDebtorNumbersDown(ctx context.Context, db *bun.DB) error {
	if err := restoreDemoVisitorFamiliesErasure(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, childDebtorNumbersDownSQL); err != nil {
		return fmt.Errorf("drop the debtor numbers of the children: %w", err)
	}
	return nil
}
