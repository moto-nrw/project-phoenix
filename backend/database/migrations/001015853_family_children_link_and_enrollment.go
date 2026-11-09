package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const familyChildrenLinkAndEnrollmentVersion = "1.15.853"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     familyChildrenLinkAndEnrollmentVersion,
		Description: "Kinderzahl der Familie auch bei der Anmeldung und über den Unterschriftslink (#6335)",
		DependsOn: []string{
			parentBillingOnlineSEPAMandatesVersion, // the newest migration before this one
			parentBillingFamilyChildrenVersion,     // the numbers and their trail this one extends
		},
	})
	Migrations.MustRegister(familyChildrenLinkAndEnrollmentUp, familyChildrenLinkAndEnrollmentDown)
}

// familyChildrenLinkAndEnrollmentUpSQL lets the parents state the children
// of their family before a contract and through the one-time link of a
// contract mail (#6335):
//
//   - enrollment.requests.family_children: the number the parents state on
//     the status page of their own registration, with the time they stated
//     it. It belongs to the registration and goes with it; no names, no
//     dates of birth. The contract of an accepted child of the registration
//     takes it over into the Elternabrechnung, unless the family stated a
//     newer number there.
//   - parent_billing.family_child_count_events.channel: where a parents'
//     read or statement came from, so the trail tells the parents portal,
//     the link of a contract mail and the registration apart. A read or
//     statement without an account (the link) has no account but a channel.
//     Only a parents' entry carries one; earlier parents' entries came from
//     the parents portal.
const familyChildrenLinkAndEnrollmentUpSQL = `
ALTER TABLE enrollment.requests
	ADD COLUMN IF NOT EXISTS family_children SMALLINT,
	ADD COLUMN IF NOT EXISTS family_children_stated_at TIMESTAMPTZ;
ALTER TABLE enrollment.requests DROP CONSTRAINT IF EXISTS chk_requests_family_children;
ALTER TABLE enrollment.requests ADD CONSTRAINT chk_requests_family_children CHECK (
	(family_children IS NULL) = (family_children_stated_at IS NULL)
	AND (family_children IS NULL OR family_children BETWEEN 1 AND 20));
COMMENT ON COLUMN enrollment.requests.family_children IS
	'Children of the family the parents stated on their registration for the sibling rule (#6335), siblings without a contract included; only the number.';

ALTER TABLE parent_billing.family_child_count_events
	ADD COLUMN IF NOT EXISTS channel VARCHAR(16);
ALTER TABLE parent_billing.family_child_count_events DROP CONSTRAINT IF EXISTS chk_family_child_count_events_channel;
ALTER TABLE parent_billing.family_child_count_events ADD CONSTRAINT chk_family_child_count_events_channel CHECK (
	channel IS NULL OR (actor = 'parent' AND channel IN ('portal', 'link', 'enrollment')));
UPDATE parent_billing.family_child_count_events SET channel = 'portal' WHERE actor = 'parent' AND channel IS NULL;
`

const familyChildrenLinkAndEnrollmentDownSQL = `
ALTER TABLE parent_billing.family_child_count_events DROP CONSTRAINT IF EXISTS chk_family_child_count_events_channel;
ALTER TABLE parent_billing.family_child_count_events DROP COLUMN IF EXISTS channel;
ALTER TABLE enrollment.requests DROP CONSTRAINT IF EXISTS chk_requests_family_children;
ALTER TABLE enrollment.requests DROP COLUMN IF EXISTS family_children_stated_at;
ALTER TABLE enrollment.requests DROP COLUMN IF EXISTS family_children;
`

func familyChildrenLinkAndEnrollmentUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, familyChildrenLinkAndEnrollmentUpSQL); err != nil {
		return fmt.Errorf("add the family children of the link and the registration: %w", err)
	}
	return nil
}

func familyChildrenLinkAndEnrollmentDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, familyChildrenLinkAndEnrollmentDownSQL); err != nil {
		return fmt.Errorf("drop the family children of the link and the registration: %w", err)
	}
	return nil
}
