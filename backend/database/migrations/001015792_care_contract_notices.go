package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const careContractNoticesVersion = "1.15.792"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     careContractNoticesVersion,
		Description: "Betreuungsvertrag: Widerruf und Kündigung mit Bestätigung (#4682)",
		DependsOn: []string{
			careContractAmendmentsVersion, // the timeline of the versions (#4681)
			careContractSignaturesVersion, // the trail actions and statuses of the signature (#4679)
		},
	})
	Migrations.MustRegister(careContractNoticesUp, careContractNoticesDown)
}

// careContractNoticesUpSQL lets the parents (or the Träger-Büro for them)
// end a concluded contract (#4682, ADR 0088 section 2):
//
//   - gekuendigt: the contract was terminated and ends on ends_on, the last
//     day it holds. The versions stay as they are; the contract cuts them.
//     An extraordinary termination (§ 312k Abs. 2 BGB: the parents name
//     the kind and their reason) has no end yet: the Träger sets ends_on
//     after it checked the reason, until then the contract holds.
//   - widerrufen: the parents withdrew within the withdrawal period
//     (§ 355 BGB). The contract is undone from its start and holds on no day.
//   - withdrawal_informed_at is when the parents received the withdrawal
//     notice on a durable medium: every contract PDF carries it from this
//     migration on (document.withdrawal_notice). Positive delivery or a
//     parent download records provision, never enqueueing. Without proof the period
//     ends 12 months and 14 days after the conclusion (§ 356 Abs. 3 BGB),
//     a paper contract signed at the office has none. early_start_requested records the
//     parents' express wish that care begins within the period (§ 356 Abs. 4,
//     § 357a Abs. 2 BGB), taken from the ticked block of the application.
//   - contract_notices holds every declaration once, with the text of its
//     confirmation as sent (Textform, § 309 Nr. 13 BGB, § 312k BGB). A
//     contract carries at most one, because the contract leaves aktiv with
//     it; the row lock of the contract orders concurrent declarations.
const careContractNoticesUpSQL = `
ALTER TABLE care_contract.contract_documents ADD COLUMN withdrawal_notice BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE care_contract.contracts DROP CONSTRAINT chk_contracts_status;
ALTER TABLE care_contract.contracts ADD CONSTRAINT chk_contracts_status
	CHECK (status IN ('entwurf', 'versendet', 'unterschrieben', 'aktiv', 'gekuendigt', 'widerrufen', 'beendet'));
ALTER TABLE care_contract.contracts
	ADD COLUMN ends_on DATE,
	ADD COLUMN withdrawal_informed_at TIMESTAMPTZ,
	ADD COLUMN early_start_requested BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE care_contract.contracts
	ADD CONSTRAINT chk_contracts_terminated CHECK (ends_on IS NULL OR status = 'gekuendigt'),
	ADD CONSTRAINT chk_contracts_notice_concluded CHECK (status NOT IN ('gekuendigt', 'widerrufen') OR concluded_at IS NOT NULL);
COMMENT ON COLUMN care_contract.contracts.ends_on IS
	'Last day a terminated contract holds (#4682); set only with status gekuendigt, open while the Träger checks an extraordinary termination.';
COMMENT ON COLUMN care_contract.contracts.withdrawal_informed_at IS
	'When the parents received the withdrawal notice with the contract PDF (§ 356 Abs. 3 BGB, #4682).';
COMMENT ON COLUMN care_contract.contracts.early_start_requested IS
	'The parents asked in the application that care begins within the withdrawal period (§ 356 Abs. 4 BGB, #4682).';
GRANT UPDATE (ends_on, withdrawal_informed_at) ON care_contract.contracts TO phoenix_tenant;

CREATE TABLE care_contract.contract_notices (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	contract_id BIGINT NOT NULL,
	kind VARCHAR(16) NOT NULL,
	notice_kind VARCHAR(24),
	channel VARCHAR(16) NOT NULL,
	account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	declared_at TIMESTAMPTZ NOT NULL,
	received_on DATE NOT NULL,
	dispatched_on DATE,
	ends_on DATE,
	reason TEXT,
	recipient_email VARCHAR(320) NOT NULL,
	confirmation_text TEXT NOT NULL,
	outbox_id BIGINT,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (organization_id, contract_id) REFERENCES care_contract.contracts (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_contract_notices_dispatch CHECK (dispatched_on IS NULL OR (kind = 'widerruf' AND dispatched_on <= received_on)),
	CONSTRAINT chk_contract_notices_kind CHECK (kind IN ('kuendigung', 'widerruf')),
	CONSTRAINT chk_contract_notices_notice_kind CHECK (
		(kind = 'kuendigung' AND notice_kind IS NOT NULL AND notice_kind IN ('ordentlich', 'sonderkuendigung') AND ends_on IS NOT NULL)
		OR (kind = 'kuendigung' AND notice_kind IS NOT NULL AND notice_kind = 'ausserordentlich')
		OR (kind = 'widerruf' AND notice_kind IS NULL AND ends_on IS NULL)),
	CONSTRAINT chk_contract_notices_reason CHECK (
		(notice_kind IS NOT NULL AND notice_kind = 'ausserordentlich') = (reason IS NOT NULL)
		AND (reason IS NULL OR (BTRIM(reason) <> '' AND char_length(reason) <= 2000))),
	CONSTRAINT chk_contract_notices_channel CHECK (channel IN ('eltern_app', 'traeger_buero')),
	CONSTRAINT chk_contract_notices_text CHECK (BTRIM(confirmation_text) <> '' AND char_length(confirmation_text) <= 8000)
);
-- Require attribution when a parent declaration is created, but allow
-- ON DELETE SET NULL to retain it after the account is hard-deleted.
CREATE FUNCTION care_contract.require_notice_account() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.channel = 'eltern_app' AND NEW.account_id IS NULL THEN
  RAISE EXCEPTION 'a parent declaration requires an account'
   USING ERRCODE = '23514', CONSTRAINT = 'chk_contract_notices_account';
 END IF;
 RETURN NEW;
END;
$$;
CREATE TRIGGER require_care_contract_notice_account
 BEFORE INSERT ON care_contract.contract_notices
 FOR EACH ROW EXECUTE FUNCTION care_contract.require_notice_account();

CREATE INDEX idx_contract_notices_contract ON care_contract.contract_notices (organization_id, contract_id);
COMMENT ON TABLE care_contract.contract_notices IS
	'Every withdrawal and termination of a care contract with its confirmation as sent (#4682).';

ALTER TABLE care_contract.contract_notices ENABLE ROW LEVEL SECURITY;
ALTER TABLE care_contract.contract_notices FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_care_contract_contract_notices ON care_contract.contract_notices FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
REVOKE ALL ON care_contract.contract_notices FROM PUBLIC;
GRANT SELECT, INSERT ON care_contract.contract_notices TO phoenix_tenant;
GRANT UPDATE (outbox_id, ends_on) ON care_contract.contract_notices TO phoenix_tenant;
GRANT USAGE ON SEQUENCE care_contract.contract_notices_id_seq TO phoenix_tenant;
GRANT ALL ON care_contract.contract_notices TO phoenix_admin;
GRANT ALL ON SEQUENCE care_contract.contract_notices_id_seq TO phoenix_admin;

ALTER TABLE care_contract.contract_events DROP CONSTRAINT chk_contract_events_action;
ALTER TABLE care_contract.contract_events ADD CONSTRAINT chk_contract_events_action
	CHECK (action IN ('read', 'create', 'send', 'discard', 'template', 'conclude', 'sign', 'remind', 'terminate', 'withdraw', 'set_end'));
`

// careContractNoticesDownSQL returns the ended contracts to aktiv; their
// declarations go with the table.
const careContractNoticesDownSQL = `
DELETE FROM care_contract.contract_events WHERE action IN ('terminate', 'withdraw', 'set_end');
ALTER TABLE care_contract.contract_events DROP CONSTRAINT chk_contract_events_action;
ALTER TABLE care_contract.contract_events ADD CONSTRAINT chk_contract_events_action
	CHECK (action IN ('read', 'create', 'send', 'discard', 'template', 'conclude', 'sign', 'remind'));
ALTER TABLE care_contract.contract_documents DROP COLUMN withdrawal_notice;
DROP TABLE IF EXISTS care_contract.contract_notices;
DROP FUNCTION IF EXISTS care_contract.require_notice_account();
ALTER TABLE care_contract.contracts DROP CONSTRAINT IF EXISTS chk_contracts_terminated;
ALTER TABLE care_contract.contracts DROP CONSTRAINT IF EXISTS chk_contracts_notice_concluded;
UPDATE care_contract.contracts SET status = 'aktiv' WHERE status IN ('gekuendigt', 'widerrufen');
ALTER TABLE care_contract.contracts DROP CONSTRAINT chk_contracts_status;
ALTER TABLE care_contract.contracts ADD CONSTRAINT chk_contracts_status
	CHECK (status IN ('entwurf', 'versendet', 'unterschrieben', 'aktiv', 'beendet'));
ALTER TABLE care_contract.contracts
	DROP COLUMN IF EXISTS ends_on,
	DROP COLUMN IF EXISTS withdrawal_informed_at,
	DROP COLUMN IF EXISTS early_start_requested;
`

func careContractNoticesUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, careContractNoticesUpSQL); err != nil {
		return fmt.Errorf("add care contract notices: %w", err)
	}
	return nil
}

func careContractNoticesDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, careContractNoticesDownSQL); err != nil {
		return fmt.Errorf("drop care contract notices: %w", err)
	}
	return nil
}
