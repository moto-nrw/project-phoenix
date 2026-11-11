package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const dataReturnErasureOrderLockVersion = "1.15.919"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     dataReturnErasureOrderLockVersion,
		Description: "Datenrückgabe: Freigabe eines Trägerpakets und Start der Trägerlöschung sperren sich gegenseitig (#6665, ADR 0203)",
		DependsOn: []string{
			dataReturnGrantsVersion,       // data_return_issue and data_return_open_for; the newest on development (1.15.917)
			traegerRecordsHandoverVersion, // begin_organization_erasure as it stands
		},
	})
	Migrations.MustRegister(dataReturnErasureOrderLockUp, dataReturnErasureOrderLockDown)
}

// The release of a Träger package and the start of the Träger's erasure
// meet on the open erasure order (#6665). Before, the release read the order
// without a lock and the begin did not look for packages, so a release
// between the CLI's package check and the begin left a started erasure with
// a retrievable package.
//
//   - The release locks the open order FOR SHARE before it reads the start.
//     A begin that holds the order FOR UPDATE makes it wait; the check that
//     follows then sees the start and refuses.
//   - The begin counts the unpurged packages of the Träger once it holds the
//     order FOR UPDATE, started or not. A release that held the order makes
//     it wait; the count then sees the package and refuses with 55006, the
//     code of the delete trigger.
//
// Both functions are VOLATILE: under READ COMMITTED the statement after the
// lock reads what the other transaction committed. The release locks the
// order before the Träger row, like begin_organization_erasure and
// erase_organization, so it cannot deadlock with either.
const (
	dataReturnIssueOrderLockSQL = `		IF p_package = 'organization' THEN
			PERFORM 1 FROM platform.organization_erasure_orders
				WHERE organization_id = p_subject AND withdrawn_at IS NULL FOR SHARE;
		END IF;
`
	dataReturnIssueWithOrderLockSQL    = dataReturnIssueHeadSQL + dataReturnIssueOrderLockSQL + dataReturnIssueRootLockSQL + dataReturnIssueErasureBegunSQL + dataReturnIssueTailSQL
	dataReturnIssueWithoutOrderLockSQL = dataReturnIssueHeadSQL + dataReturnIssueRootLockSQL + dataReturnIssueErasureBegunSQL + dataReturnIssueTailSQL

	organizationErasureBeginDataReturnSQL = organizationErasureBeginHeadSQL + organizationErasureBeginRecordsChoiceSQL + `
  IF platform.data_return_open_for('organization', p_org) > 0 THEN
   RAISE EXCEPTION 'organization erasure: the Träger has a released return package that is not purged' USING ERRCODE = '55006';
  END IF;` + organizationErasureBeginTailSQL
)

func dataReturnErasureOrderLockUp(ctx context.Context, db *bun.DB) error {
	return replaceDataReturnErasureFunctions(ctx, db, dataReturnIssueWithOrderLockSQL, organizationErasureBeginDataReturnSQL)
}

func dataReturnErasureOrderLockDown(ctx context.Context, db *bun.DB) error {
	return replaceDataReturnErasureFunctions(ctx, db, dataReturnIssueWithoutOrderLockSQL, organizationErasureBeginRecordsSQL)
}

// CREATE OR REPLACE keeps the owner, the grants and the revokes.
func replaceDataReturnErasureFunctions(ctx context.Context, db *bun.DB, issueSQL, beginSQL string) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, issueSQL); err != nil {
			return fmt.Errorf("replace data_return_issue: %w", err)
		}
		if _, err := tx.ExecContext(ctx, beginSQL); err != nil {
			return fmt.Errorf("replace begin_organization_erasure: %w", err)
		}
		return nil
	})
}
