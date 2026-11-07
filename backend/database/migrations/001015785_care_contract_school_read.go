package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const careContractSchoolReadVersion = "1.15.785"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     careContractSchoolReadVersion,
		Description: "Betreuungsvertrag: die Schule liest die gebuchten Leistungen ihrer versendeten Verträge (#4683)",
		DependsOn: []string{
			traegerDocumentDownloadsVersion, // the last migration before this one
			careContractDraftsVersion,       // the tables this policy reads
		},
	})
	Migrations.MustRegister(careContractSchoolReadUp, careContractSchoolReadDown)
}

// careContractSchoolReadUpSQL lets a school read the contracts whose place of
// care it is (ADR 0088 section 7, ADR 0153 point 9, ADR 0156): the booked
// services per weekday become the Soll of Soll und Ist (#4683). The school
// reads only in its own tenant transaction (app.current_tenant_id) and only
// contracts past the draft: a draft is the Träger-Büro's work in progress and
// promises nothing yet. The policies are SELECT only; every write stays with
// the Träger policy of migration 1.15.770. A version and its services carry
// no school, so their policies reach the school through their contract.
const careContractSchoolReadUpSQL = `
CREATE POLICY school_read_care_contract_contracts ON care_contract.contracts FOR SELECT
	USING (school_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint AND status <> 'entwurf');
CREATE POLICY school_read_care_contract_contract_versions ON care_contract.contract_versions FOR SELECT
	USING (EXISTS (SELECT 1 FROM care_contract.contracts AS c
		WHERE c.id = contract_versions.contract_id
			AND c.school_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
			AND c.status <> 'entwurf'));
CREATE POLICY school_read_care_contract_contract_version_services ON care_contract.contract_version_services FOR SELECT
	USING (EXISTS (SELECT 1 FROM care_contract.contract_versions AS v
		JOIN care_contract.contracts AS c ON c.id = v.contract_id
		WHERE v.id = contract_version_services.contract_version_id
			AND c.school_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
			AND c.status <> 'entwurf'));
CREATE INDEX IF NOT EXISTS idx_contracts_school_student ON care_contract.contracts (school_id, student_id);
`

const careContractSchoolReadDownSQL = `
DROP INDEX IF EXISTS care_contract.idx_contracts_school_student;
DROP POLICY IF EXISTS school_read_care_contract_contract_version_services ON care_contract.contract_version_services;
DROP POLICY IF EXISTS school_read_care_contract_contract_versions ON care_contract.contract_versions;
DROP POLICY IF EXISTS school_read_care_contract_contracts ON care_contract.contracts;
`

func careContractSchoolReadUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, careContractSchoolReadUpSQL); err != nil {
		return fmt.Errorf("add school read of care contracts: %w", err)
	}
	return nil
}

func careContractSchoolReadDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, careContractSchoolReadDownSQL); err != nil {
		return fmt.Errorf("drop school read of care contracts: %w", err)
	}
	return nil
}
