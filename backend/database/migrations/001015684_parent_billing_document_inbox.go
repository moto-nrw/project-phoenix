package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingDocumentInboxVersion = "1.15.684"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingDocumentInboxVersion,
		Description: "Dokumentenpostfach in der Eltern-App: Push des Trägers bei neuem Dokument (#4671)",
		DependsOn: []string{
			parentBillingLetterMailingsVersion, // the sends that put a letter in the inbox
			holidayGuestErasureVersion,         // the last migration before this one (#4666)
		},
	})
	Migrations.MustRegister(parentBillingDocumentInboxUp, parentBillingDocumentInboxDown)
}

// parentBillingDocumentInboxUpSQL prepares the document inbox of the parents
// portal (#4671, ADR 0126):
//
//   - platform.push_outbox.organization_id: a push of a Träger, which the
//     Träger transaction enqueues together with the send of a letter, like
//     the mail of a Träger (ADR 0110). It names no school and no device: the
//     worker resolves the parents' devices at dispatch time. A push has
//     exactly one scope, a school or a Träger. The Träger policy admits the
//     request role only to the pushes of its own Träger; the school policy
//     never sees them.
//   - parent_billing.documents: an index on the Träger and the lower-case
//     address of the recipient, the lookup of the inbox.
const parentBillingDocumentInboxUpSQL = `
ALTER TABLE platform.push_outbox
	ADD COLUMN organization_id BIGINT REFERENCES platform.organizations(id) ON DELETE CASCADE;
ALTER TABLE platform.push_outbox ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE platform.push_outbox
	ADD CONSTRAINT chk_push_outbox_one_scope CHECK ((tenant_id IS NULL) <> (organization_id IS NULL));
CREATE INDEX idx_push_outbox_organization ON platform.push_outbox (organization_id)
	WHERE organization_id IS NOT NULL;
DROP INDEX platform.uq_push_outbox_tenant_idempotency;
CREATE UNIQUE INDEX uq_push_outbox_tenant_idempotency
	ON platform.push_outbox (tenant_id, organization_id, idempotency_key) NULLS NOT DISTINCT
	WHERE idempotency_key IS NOT NULL;
CREATE POLICY traeger_isolation_platform_push_outbox ON platform.push_outbox FOR ALL
	USING (tenant_id IS NULL AND organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (tenant_id IS NULL AND organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

CREATE INDEX idx_parent_billing_documents_recipient
	ON parent_billing.documents (organization_id, LOWER(recipient_email));
`

const parentBillingDocumentInboxDownSQL = `
DROP INDEX IF EXISTS parent_billing.idx_parent_billing_documents_recipient;
DROP POLICY IF EXISTS traeger_isolation_platform_push_outbox ON platform.push_outbox;
DELETE FROM platform.push_outbox WHERE organization_id IS NOT NULL;
DROP INDEX IF EXISTS platform.uq_push_outbox_tenant_idempotency;
CREATE UNIQUE INDEX uq_push_outbox_tenant_idempotency
	ON platform.push_outbox (tenant_id, idempotency_key)
	WHERE idempotency_key IS NOT NULL;
DROP INDEX IF EXISTS platform.idx_push_outbox_organization;
ALTER TABLE platform.push_outbox DROP CONSTRAINT IF EXISTS chk_push_outbox_one_scope;
ALTER TABLE platform.push_outbox ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE platform.push_outbox DROP COLUMN IF EXISTS organization_id;
`

func parentBillingDocumentInboxUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingDocumentInboxUpSQL); err != nil {
		return fmt.Errorf("prepare parent billing document inbox: %w", err)
	}
	return nil
}

func parentBillingDocumentInboxDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingDocumentInboxDownSQL); err != nil {
		return fmt.Errorf("revert parent billing document inbox: %w", err)
	}
	return nil
}
