package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

const demoVisitorErasureContractPushVersion = "1.15.808"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     demoVisitorErasureContractPushVersion,
		Description: "Demo-Besuch löschen: Push „Neues Dokument vom Träger“ eines versendeten Vertrags entfernen",
		DependsOn: []string{
			demoVisitorErasureCareInvoicesVersion, // the function this one replaces, and the last erasure before it
		},
	})
	Migrations.MustRegister(demoVisitorErasureContractPushUp, demoVisitorErasureContractPushDown)
}

// demoVisitorErasureContractPushAnchor is the statement of 1.15.805 that
// deletes the mail of a contract of a retired demo school. The push of the
// contract follows it, while the contracts still exist to name it.
const demoVisitorErasureContractPushAnchor = `
					DELETE FROM platform.email_outbox AS outbox
					USING care_contract.contracts AS contract
					WHERE contract.school_id = ANY (hidden_schools)
						AND outbox.organization_id = contract.organization_id
						AND outbox.related_entity_type = 'care_contract'
						AND outbox.related_entity_id = contract.id;`

// demoVisitorErasureContractPushSQL deletes the deferred push of a sent
// contract (#5941). It is a row of the Träger in Delivery's push outbox,
// named by the contract alone, and its payload keeps the recipient's mail
// address; nothing removes it with the school.
const demoVisitorErasureContractPushSQL = `
					-- The push of a sent contract keeps the recipient's address
					-- in its payload (#5941); it goes with the contract it names.
					DELETE FROM platform.push_outbox AS push
					USING care_contract.contracts AS contract
					WHERE contract.school_id = ANY (hidden_schools)
						AND push.organization_id = contract.organization_id
						AND push.related_entity_type = 'care_contract_document'
						AND push.related_entity_id = contract.id;`

// demoVisitorErasureContractPushUpSQL is platform.erase_closed_demo_visitors
// of 1.15.805 that also deletes the push of a sent contract. The rest of the
// function is unchanged.
func demoVisitorErasureContractPushUpSQL() (string, error) {
	if strings.Count(demoVisitorErasureCareInvoicesUpSQL, demoVisitorErasureContractPushAnchor) != 1 {
		return "", fmt.Errorf("the erasure of %s no longer holds the contract mail statement exactly once", demoVisitorErasureCareInvoicesVersion)
	}
	return strings.Replace(demoVisitorErasureCareInvoicesUpSQL, demoVisitorErasureContractPushAnchor,
		demoVisitorErasureContractPushAnchor+demoVisitorErasureContractPushSQL, 1), nil
}

func demoVisitorErasureContractPushUp(ctx context.Context, db *bun.DB) error {
	statement, err := demoVisitorErasureContractPushUpSQL()
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("erase the contract push of retired demo schools: %w", err)
	}
	return nil
}

// demoVisitorErasureContractPushDown reinstalls the erasure of 1.15.805 by
// running that migration's Up, which defines exactly that function.
func demoVisitorErasureContractPushDown(ctx context.Context, db *bun.DB) error {
	if err := demoVisitorErasureCareInvoicesUp(ctx, db); err != nil {
		return fmt.Errorf("restore the demo visitor erasure of 1.15.805: %w", err)
	}
	return nil
}
