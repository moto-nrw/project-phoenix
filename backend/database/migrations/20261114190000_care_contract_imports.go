package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const careContractImportsVersion = "20261114190000"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     careContractImportsVersion,
		Description: "Betreuungsvertrag: Bestandsverträge aus einer Datei, ohne Anmeldung (#4723)",
		DependsOn: []string{
			sessionMFAByDeviceVersion,            // the last migration before this one
			careContractDraftsVersion,            // care_contract.contracts
			careContractConclusionVersion,        // the conclusion an imported contract carries
			careContractAmendmentsVersion,        // the source of a version
			careContractNoticeEndDecisionVersion, // the newest shape of the contract tables
		},
	})
	Migrations.MustRegister(careContractImportsUp, careContractImportsDown)
}

// careContractImportsUpSQL lets a contract come from an import file instead
// of an accepted application (#4723, ADR 0088 section 3, ADR 0232):
//
//   - A contract names its origin by exactly one key: request_child_id, the
//     application child (ADR 0153), or import_origin, the child, the school and
//     the start an import file named. request_child_id is therefore
//     optional.
//   - import_origin is the trimmed first and last name of the child without
//     case, its date of birth and the first day of the contract. It is
//     unique per Träger and school, so two imports of one file running at
//     the same time store each contract once. Whether a child already has a
//     running contract is the import's check, not this key.
//   - The first version of an imported contract has the source import.
const careContractImportsUpSQL = `
ALTER TABLE care_contract.contracts ALTER COLUMN request_child_id DROP NOT NULL;
ALTER TABLE care_contract.contracts ADD COLUMN import_origin TEXT;
ALTER TABLE care_contract.contracts
	ADD CONSTRAINT chk_contracts_origin CHECK ((request_child_id IS NULL) <> (import_origin IS NULL)),
	ADD CONSTRAINT chk_contracts_import_origin CHECK (import_origin IS NULL
		OR (BTRIM(import_origin) <> '' AND char_length(import_origin) <= 600));
CREATE UNIQUE INDEX uq_contracts_import ON care_contract.contracts (organization_id, school_id, import_origin)
	WHERE import_origin IS NOT NULL;
COMMENT ON COLUMN care_contract.contracts.import_origin IS
	'Origin of a contract taken over from an import file (#4723): child name without case, date of birth and first day; NULL for a contract from an application.';
COMMENT ON COLUMN care_contract.contracts.request_child_id IS
	'Origin of a contract from an accepted application (ADR 0153); NULL for a contract taken over from an import file (#4723).';

ALTER TABLE care_contract.contract_versions DROP CONSTRAINT chk_contract_versions_source;
ALTER TABLE care_contract.contract_versions ADD CONSTRAINT chk_contract_versions_source
	CHECK (source IN ('abschluss', 'buchungsaenderung', 'import'));
COMMENT ON COLUMN care_contract.contract_versions.source IS
	'Where the version came from: abschluss (first version), buchungsaenderung (approved change of the booked care, #4681) or import (first version of a contract from an import file, #4723).';
`

// careContractImportsDownSQL removes the imported contracts with everything
// that hangs on them: without the import key nothing names their origin, and
// request_child_id cannot stay empty.
const careContractImportsDownSQL = `
DELETE FROM care_contract.contracts WHERE request_child_id IS NULL;
ALTER TABLE care_contract.contract_versions DROP CONSTRAINT chk_contract_versions_source;
ALTER TABLE care_contract.contract_versions ADD CONSTRAINT chk_contract_versions_source
	CHECK (source IN ('abschluss', 'buchungsaenderung'));
DROP INDEX IF EXISTS care_contract.uq_contracts_import;
ALTER TABLE care_contract.contracts
	DROP CONSTRAINT IF EXISTS chk_contracts_import_origin,
	DROP CONSTRAINT IF EXISTS chk_contracts_origin,
	DROP COLUMN IF EXISTS import_origin;
ALTER TABLE care_contract.contracts ALTER COLUMN request_child_id SET NOT NULL;
COMMENT ON COLUMN care_contract.contracts.request_child_id IS NULL;
COMMENT ON COLUMN care_contract.contract_versions.source IS
	'Where the version came from: abschluss (first version) or buchungsaenderung (approved change of the booked care, #4681).';
`

func careContractImportsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, careContractImportsUpSQL); err != nil {
		return fmt.Errorf("allow care contracts from an import: %w", err)
	}
	return nil
}

func careContractImportsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, careContractImportsDownSQL); err != nil {
		return fmt.Errorf("remove care contracts from an import: %w", err)
	}
	return nil
}
