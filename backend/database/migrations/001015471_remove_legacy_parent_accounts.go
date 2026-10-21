package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version: "1.15.471", Description: "Remove unused legacy parent accounts and person links (#4329)",
		DependsOn:    []string{"1.15.470", "1.15.57"},
		Precondition: legacyParentAccountsPrecondition,
	})
	Migrations.MustRegister(removeLegacyParentAccountsUp, removeLegacyParentAccountsDown)
}

// Production held zero rows in both tables on 2026-10-21. Refuse to discard
// any data created since that observation, including on another installation.
func legacyParentAccountsPrecondition(ctx context.Context, db *bun.DB) error {
	// On a fresh installation both source migrations are pending. A partial
	// installation can have one table already, so still check that table.
	return requireEmptyLegacyParentAccounts(ctx, db,
		!migrationPending(ctx, AuthAccountsParentsVersion),
		!migrationPending(ctx, UsersPersonsGuardiansVersion),
	)
}

func requireEmptyLegacyParentAccounts(ctx context.Context, db bun.IDB, checkAccounts, checkLinks bool) error {
	var accounts, links int64
	if checkAccounts {
		if err := db.NewRaw(`SELECT COUNT(*) FROM auth.accounts_parents`).Scan(ctx, &accounts); err != nil {
			return fmt.Errorf("count legacy parent accounts: %w", err)
		}
	}
	if checkLinks {
		if err := db.NewRaw(`SELECT COUNT(*) FROM users.persons_guardians`).Scan(ctx, &links); err != nil {
			return fmt.Errorf("count legacy person guardian links: %w", err)
		}
	}
	if accounts != 0 || links != 0 {
		var occupied []string
		if accounts != 0 {
			occupied = append(occupied, fmt.Sprintf("auth.accounts_parents has %d rows", accounts))
		}
		if links != 0 {
			occupied = append(occupied, fmt.Sprintf("users.persons_guardians has %d rows", links))
		}
		return fmt.Errorf("legacy parent removal requires empty tables: %s; investigate legacy API usage before proceeding", strings.Join(occupied, ", "))
	}
	return nil
}

func removeLegacyParentAccountsUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin legacy parent removal: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			logRollbackFailure(ctx, err)
		}
	}()
	// Lock before checking emptiness: the previous release can still write
	// through its legacy API until deployment switches images.
	if _, err := tx.ExecContext(ctx, `LOCK TABLE auth.accounts_parents, users.persons_guardians IN ACCESS EXCLUSIVE MODE`); err != nil {
		return fmt.Errorf("lock legacy parent tables: %w", err)
	}
	if err := requireEmptyLegacyParentAccounts(ctx, tx, true, true); err != nil {
		return err
	}
	// No CASCADE: an unexpected dependency must fail rather than disappear.
	if _, err := tx.ExecContext(ctx, `DROP TABLE users.persons_guardians;
		DROP FUNCTION users.enforce_single_primary_guardian();
		DROP TABLE auth.accounts_parents;`); err != nil {
		return fmt.Errorf("remove legacy parent tables: %w", err)
	}
	return tx.Commit()
}

func removeLegacyParentAccountsDown(context.Context, *bun.DB) error {
	return errors.New("legacy parent account removal cannot be rolled back in place; restore the release database backup and matching images")
}
