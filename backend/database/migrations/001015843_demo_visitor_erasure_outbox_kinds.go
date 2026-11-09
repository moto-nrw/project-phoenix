package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const demoVisitorErasureOutboxKindsVersion = "1.15.843"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     demoVisitorErasureOutboxKindsVersion,
		Description: "Demo-Besuch löschen: jede Art der Versand-Warteschlange erfassen, Träger-Arten im Schema festhalten",
		DependsOn: []string{
			cleanupRunFailuresVersion,       // the newest migration before this one
			parentBillingCareNoticesVersion, // the erasure of 1.15.823 this one extends
			traegerSharedDocumentsVersion,   // the releases whose pushes go
			holidayRegistrationsVersion,     // the registrations whose receipts go
		},
	})
	Migrations.MustRegister(demoVisitorErasureOutboxKindsUp, demoVisitorErasureOutboxKindsDown)
}

// A row of the Träger in Delivery's outbox (organization_id set, ADR 0110,
// ADR 0126) outlives the school it is about, so the demo erasure has to name
// it by its kind. The constraints are the list of those kinds: a producer
// cannot enqueue a kind of the Träger that is not in it, and the guard of
// #6342 (TestDemoErasureCoversEveryOutboxKind) reads the list from the
// schema and fails for a kind no statement of the erasure names. A new kind
// of the Träger is therefore a migration that adds it here and to the
// erasure, or to demoErasureOutboxKindExceptions with its reason.
const demoVisitorErasureOutboxKindsConstraintsSQL = `
ALTER TABLE platform.email_outbox ADD CONSTRAINT chk_email_outbox_traeger_kind CHECK (
	organization_id IS NULL OR kind IN (
		'care_contract', 'care_contract_notice', 'holiday_registration_received', 'parent_billing_letter'));
COMMENT ON CONSTRAINT chk_email_outbox_traeger_kind ON platform.email_outbox IS
	'The mail kinds of a Träger. A new kind needs its demo erasure in platform.erase_closed_demo_visitors (#6342).';
ALTER TABLE platform.push_outbox ADD CONSTRAINT chk_push_outbox_traeger_kind CHECK (
	organization_id IS NULL OR kind IN (
		'care_contract_document', 'parent_billing_document', 'traeger_document_release', 'traeger_document_release_account'));
COMMENT ON CONSTRAINT chk_push_outbox_traeger_kind ON platform.push_outbox IS
	'The push kinds of a Träger. A new kind needs its demo erasure in platform.erase_closed_demo_visitors (#6342).';
`

const demoVisitorErasureOutboxKindsConstraintsDownSQL = `
ALTER TABLE platform.email_outbox DROP CONSTRAINT IF EXISTS chk_email_outbox_traeger_kind;
ALTER TABLE platform.push_outbox DROP CONSTRAINT IF EXISTS chk_push_outbox_traeger_kind;
`

func demoVisitorErasureOutboxKindsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, demoVisitorErasureOutboxKindsConstraintsSQL); err != nil {
		return fmt.Errorf("declare the outbox kinds of a Träger: %w", err)
	}
	return installOutboxKindErasure(ctx, db)
}

func demoVisitorErasureOutboxKindsDown(ctx context.Context, db *bun.DB) error {
	if err := installCareNoticeErasure(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, demoVisitorErasureOutboxKindsConstraintsDownSQL); err != nil {
		return fmt.Errorf("drop the outbox kinds of a Träger: %w", err)
	}
	return nil
}
