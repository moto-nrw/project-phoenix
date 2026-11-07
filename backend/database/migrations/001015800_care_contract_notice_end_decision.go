package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const careContractNoticeEndDecisionVersion = "1.15.800"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     careContractNoticeEndDecisionVersion,
		Description: "Betreuungsvertrag: Entscheidung über die außerordentliche Kündigung mit Mitteilung an die Eltern (#4682)",
		DependsOn: []string{
			traegerDocumentsRoleVersion, // the last migration before this one
			careContractNoticesVersion,  // the declarations this one decides
		},
	})
	Migrations.MustRegister(careContractNoticeEndDecisionUp, careContractNoticeEndDecisionDown)
}

// careContractNoticeEndDecisionUpSQL records how the Träger-Büro decided the
// end of an extraordinary termination (#4682, decision of 07.11.2026):
//
//   - festgelegt: it accepted the reason and set the last day.
//   - umgedeutet: it did not accept the reason; the declaration holds as an
//     ordinary termination to the next notice date from its receipt
//     (§ 140 BGB).
//
// Either way the parents get a notice in Textform; end_notice_text is that
// notice as sent, end_notice_outbox_id its mail. An end set before this
// migration counts as festgelegt and has no notice. The columns hang on
// contract_notices, which the demo erasure removes with its contract
// (ON DELETE CASCADE); the mail is related to the contract as well.
const careContractNoticeEndDecisionUpSQL = `
ALTER TABLE care_contract.contract_notices
	ADD COLUMN end_decision VARCHAR(16),
	ADD COLUMN end_decided_at TIMESTAMPTZ,
	ADD COLUMN end_decided_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	ADD COLUMN end_notice_text TEXT,
	ADD COLUMN end_notice_outbox_id BIGINT;
UPDATE care_contract.contract_notices SET end_decision = 'festgelegt'
	WHERE notice_kind = 'ausserordentlich' AND ends_on IS NOT NULL;
ALTER TABLE care_contract.contract_notices
	ADD CONSTRAINT chk_contract_notices_end_decision CHECK (
		(end_decision IS NULL AND end_decided_at IS NULL AND end_notice_text IS NULL AND end_notice_outbox_id IS NULL
			AND (notice_kind IS DISTINCT FROM 'ausserordentlich' OR ends_on IS NULL))
		OR (end_decision IN ('festgelegt', 'umgedeutet') AND notice_kind IS NOT DISTINCT FROM 'ausserordentlich' AND ends_on IS NOT NULL)),
	ADD CONSTRAINT chk_contract_notices_end_notice_text CHECK (
		end_notice_text IS NULL OR (BTRIM(end_notice_text) <> '' AND char_length(end_notice_text) <= 8000));
COMMENT ON COLUMN care_contract.contract_notices.end_decision IS
	'How the Träger-Büro decided the end of an extraordinary termination: festgelegt or umgedeutet (ordinary termination, § 140 BGB) (#4682).';
COMMENT ON COLUMN care_contract.contract_notices.end_notice_text IS
	'The notice of the decided end the parents got in Textform, as sent (#4682).';
GRANT UPDATE (end_decision, end_decided_at, end_decided_by, end_notice_text, end_notice_outbox_id)
	ON care_contract.contract_notices TO phoenix_tenant;

ALTER TABLE care_contract.contract_events DROP CONSTRAINT chk_contract_events_action;
ALTER TABLE care_contract.contract_events ADD CONSTRAINT chk_contract_events_action
	CHECK (action IN ('read', 'create', 'send', 'discard', 'template', 'conclude', 'sign', 'remind', 'terminate', 'withdraw', 'set_end', 'reject_reason'));
`

// careContractNoticeEndDecisionDownSQL drops the decision. A declaration
// whose reason was not accepted keeps its end as an end the Träger set.
const careContractNoticeEndDecisionDownSQL = `
DELETE FROM care_contract.contract_events WHERE action = 'reject_reason';
ALTER TABLE care_contract.contract_events DROP CONSTRAINT chk_contract_events_action;
ALTER TABLE care_contract.contract_events ADD CONSTRAINT chk_contract_events_action
	CHECK (action IN ('read', 'create', 'send', 'discard', 'template', 'conclude', 'sign', 'remind', 'terminate', 'withdraw', 'set_end'));
ALTER TABLE care_contract.contract_notices
	DROP CONSTRAINT IF EXISTS chk_contract_notices_end_notice_text,
	DROP CONSTRAINT IF EXISTS chk_contract_notices_end_decision,
	DROP COLUMN IF EXISTS end_notice_outbox_id,
	DROP COLUMN IF EXISTS end_notice_text,
	DROP COLUMN IF EXISTS end_decided_by,
	DROP COLUMN IF EXISTS end_decided_at,
	DROP COLUMN IF EXISTS end_decision;
`

func careContractNoticeEndDecisionUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, careContractNoticeEndDecisionUpSQL); err != nil {
		return fmt.Errorf("add care contract notice end decision: %w", err)
	}
	return nil
}

func careContractNoticeEndDecisionDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, careContractNoticeEndDecisionDownSQL); err != nil {
		return fmt.Errorf("drop care contract notice end decision: %w", err)
	}
	return nil
}
