package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const careContractConclusionVersion = "1.15.772"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     careContractConclusionVersion,
		Description: "Betreuungsvertrag: Abschluss bei der Annahme, Status aktiv und wirksame Version (#4680)",
		DependsOn: []string{
			careContractDraftsVersion, // the tables of the contract (#4678)
		},
	})
	Migrations.MustRegister(careContractConclusionUp, careContractConclusionDown)
}

// careContractConclusionUpSQL lets a contract be concluded (#4680, ADR 0088
// sections 2 and 3):
//
//   - contracts.concluded_at and contracts.conclusion_method record when and
//     how a contract was concluded: mit_anmeldung at the acceptance (Weg b),
//     einfach_elektronisch or papier with the signature (#4679). Both are
//     set together, once; an active contract always has them.
//   - The request role may set them, next to status and sent_at.
//   - The request role may change the status of a version, and only along
//     ADR 0088: beantragt to wirksam or abgelehnt, wirksam to abgeloest. Every
//     other column of a version stays frozen (column grant), and a trigger
//     refuses any other status step.
//   - The trail knows the action conclude. The list of actions is the one
//     of the signature (#4679, 1.15.771) with conclude, sign and remind, so
//     both migrations leave the same check whichever runs last.
const careContractConclusionUpSQL = `
ALTER TABLE care_contract.contracts
	ADD COLUMN concluded_at TIMESTAMPTZ,
	ADD COLUMN conclusion_method VARCHAR(24);
ALTER TABLE care_contract.contracts
	ADD CONSTRAINT chk_contracts_conclusion_method CHECK (conclusion_method IS NULL
		OR conclusion_method IN ('mit_anmeldung', 'einfach_elektronisch', 'papier')),
	ADD CONSTRAINT chk_contracts_concluded CHECK ((concluded_at IS NULL) = (conclusion_method IS NULL)),
	ADD CONSTRAINT chk_contracts_active_concluded CHECK (status <> 'aktiv' OR concluded_at IS NOT NULL);
COMMENT ON COLUMN care_contract.contracts.concluded_at IS
	'When the contract was concluded: at the acceptance (Weg b, #4680) or with the signature (#4679).';
COMMENT ON COLUMN care_contract.contracts.conclusion_method IS
	'How the contract was concluded (ADR 0088 section 3): mit_anmeldung, einfach_elektronisch or papier.';
GRANT UPDATE (concluded_at, conclusion_method) ON care_contract.contracts TO phoenix_tenant;

CREATE OR REPLACE FUNCTION care_contract.guard_version_status() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF NEW.status IS DISTINCT FROM OLD.status AND NOT (
		(OLD.status = 'beantragt' AND NEW.status IN ('wirksam', 'abgelehnt'))
		OR (OLD.status = 'wirksam' AND NEW.status = 'abgeloest')) THEN
		RAISE EXCEPTION 'care contract version % cannot move from % to %', OLD.id, OLD.status, NEW.status
			USING ERRCODE = 'check_violation';
	END IF;
	RETURN NEW;
END;
$$;
CREATE TRIGGER guard_care_contract_version_status
	BEFORE UPDATE OF status ON care_contract.contract_versions
	FOR EACH ROW EXECUTE FUNCTION care_contract.guard_version_status();
GRANT UPDATE (status) ON care_contract.contract_versions TO phoenix_tenant;

ALTER TABLE care_contract.contract_events DROP CONSTRAINT chk_contract_events_action;
ALTER TABLE care_contract.contract_events ADD CONSTRAINT chk_contract_events_action
	CHECK (action IN ('read', 'create', 'send', 'discard', 'template', 'conclude', 'sign', 'remind'));
`

const careContractConclusionDownSQL = `
DELETE FROM care_contract.contract_events WHERE action = 'conclude';
ALTER TABLE care_contract.contract_events DROP CONSTRAINT chk_contract_events_action;
ALTER TABLE care_contract.contract_events ADD CONSTRAINT chk_contract_events_action
	CHECK (action IN ('read', 'create', 'send', 'discard', 'template', 'sign', 'remind'));
-- The grant on the status of a version stays: the signature (#4679) shares it.
DROP TRIGGER IF EXISTS guard_care_contract_version_status ON care_contract.contract_versions;
DROP FUNCTION IF EXISTS care_contract.guard_version_status();
UPDATE care_contract.contract_versions SET status = 'beantragt' WHERE status <> 'beantragt';
UPDATE care_contract.contracts SET status = 'versendet' WHERE status IN ('unterschrieben', 'aktiv', 'beendet');
REVOKE UPDATE (concluded_at, conclusion_method) ON care_contract.contracts FROM phoenix_tenant;
ALTER TABLE care_contract.contracts
	DROP CONSTRAINT IF EXISTS chk_contracts_active_concluded,
	DROP CONSTRAINT IF EXISTS chk_contracts_concluded,
	DROP CONSTRAINT IF EXISTS chk_contracts_conclusion_method,
	DROP COLUMN IF EXISTS conclusion_method,
	DROP COLUMN IF EXISTS concluded_at;
`

func careContractConclusionUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, careContractConclusionUpSQL); err != nil {
		return fmt.Errorf("add care contract conclusion: %w", err)
	}
	return nil
}

func careContractConclusionDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, careContractConclusionDownSQL); err != nil {
		return fmt.Errorf("drop care contract conclusion: %w", err)
	}
	return nil
}
