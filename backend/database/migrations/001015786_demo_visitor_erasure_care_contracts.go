package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

const demoVisitorErasureCareContractsVersion = "1.15.786"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     demoVisitorErasureCareContractsVersion,
		Description: "Demo-Besuch löschen: Betreuungsverträge der Demo-Schule mit Vertragsparteien und Vertragsmails entfernen",
		DependsOn: []string{
			traegerDocumentDownloadsVersion,  // the last migration before this one
			careContractDraftsVersion,        // the tables it deletes from
			demoVisitorAccountErasureVersion, // the function this one replaces
		},
	})
	Migrations.MustRegister(demoVisitorErasureCareContractsUp, demoVisitorErasureCareContractsDown)
}

// demoVisitorErasureCareContractsAnchor is the statement of 1.15.734 the
// contract deletion follows: the last deletion of the whole retired school.
const demoVisitorErasureCareContractsAnchor = `
					DELETE FROM iot.push_subscriptions
					WHERE tenant_id = ANY (hidden_schools);
`

// demoVisitorErasureCareContractsSQL deletes the care contracts of the
// retired demo schools (#4678, ADR 0153). A contract outlives its school
// (ADR 0088 section 9), so nothing removes it with the school, and it names
// the visitor: the parent of the application is the contracting party
// (contract_parties), is printed in the PDF (contract_documents) and is the
// recipient of the mail that carried it. The school is the visitor's alone,
// so every contract of it goes; versions, services, parties, documents and
// mailings go with it by cascade. The Träger mail of a contract has no
// school, so it goes by the contract it names; its delivery events go with
// it. The trail keeps the contract ID without a key and goes by school or
// contract.
//
// The deletions run in the order's subtransaction under its lock_timeout like
// every other one: a contract row another transaction holds defers the whole
// order, nothing of it is deleted partly.
const demoVisitorErasureCareContractsSQL = `
					-- A care contract outlives its school (ADR 0088 section 9) and
					-- names the visitor as the contracting parent, in its PDF and in
					-- the mail that carried it. The school is the visitor's alone, so
					-- every contract of it goes with what hangs on it by cascade.
					DELETE FROM platform.email_outbox AS outbox
					USING care_contract.contracts AS contract
					WHERE contract.school_id = ANY (hidden_schools)
						AND outbox.organization_id = contract.organization_id
						AND outbox.related_entity_type = 'care_contract'
						AND outbox.related_entity_id = contract.id;
					DELETE FROM care_contract.contract_events AS event
					WHERE event.school_id = ANY (hidden_schools)
						OR event.contract_id IN (
							SELECT contract.id FROM care_contract.contracts AS contract
							WHERE contract.school_id = ANY (hidden_schools));
					DELETE FROM care_contract.contracts
					WHERE school_id = ANY (hidden_schools);
`

// demoVisitorErasureCareContractsUpSQL is platform.erase_closed_demo_visitors
// of 1.15.734 with the contract deletion after the last deletion of the
// retired school. Everything else is unchanged.
func demoVisitorErasureCareContractsUpSQL() (string, error) {
	if count := strings.Count(demoVisitorAccountErasureUpSQL, demoVisitorErasureCareContractsAnchor); count != 1 {
		return "", fmt.Errorf("the erasure of 1.15.734 names its last school deletion %d times, want once", count)
	}
	return strings.Replace(demoVisitorAccountErasureUpSQL, demoVisitorErasureCareContractsAnchor,
		demoVisitorErasureCareContractsAnchor+demoVisitorErasureCareContractsSQL, 1), nil
}

func demoVisitorErasureCareContractsUp(ctx context.Context, db *bun.DB) error {
	definition, err := demoVisitorErasureCareContractsUpSQL()
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, definition); err != nil {
		return fmt.Errorf("erase the care contracts of retired demo schools: %w", err)
	}
	return nil
}

// demoVisitorErasureCareContractsDown reinstalls the erasure of 1.15.734.
func demoVisitorErasureCareContractsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, demoVisitorAccountErasureUpSQL); err != nil {
		return fmt.Errorf("restore the demo visitor erasure of 1.15.734: %w", err)
	}
	return nil
}
