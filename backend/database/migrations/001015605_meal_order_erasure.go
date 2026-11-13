package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	mealOrderErasureVersion     = "1.15.605"
	mealOrderErasureDescription = "Mandantenlöschung: Bestellungen, Anmeldeänderungen, Caterer-Einladungen und Belieferungen einer Schule (#3873, #3837)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     mealOrderErasureVersion,
		Description: mealOrderErasureDescription,
		DependsOn: []string{
			mealCaterersVersion,
			classHomeworkVersion,
			surveysVersion, // newest reinstall of the erasure functions
		},
	})
	Migrations.MustRegister(mealOrderErasureUp, mealOrderErasureDown)
}

// The installed erasure plan is a frozen projection of the data lifecycle
// catalog (ADR 0092): a new school-scoped table needs a migration updating
// the school erasure functions. Up installs them with the eight
// school-scoped tables of the meal ordering (#3873) on top of development's
// current functions (the school_erasure_*.go constants, last reinstalled by
// 1.15.510); Down installs development's current functions. The caterer
// business and its accounts are shared across schools and stay (ADR 0077).
func mealOrderErasureUp(ctx context.Context, db *bun.DB) error {
	return installSchoolErasureSteps(ctx, db, "add the meal orders to the school erasure",
		installMealOrderErasurePlan, installMealOrderErasurePreflight, installMealOrderErasureResidual,
		installMealOrderErasureDelete, installMealOrderErasurePreview)
}

func mealOrderErasureDown(ctx context.Context, db *bun.DB) error {
	return installSchoolErasureSteps(ctx, db, "remove the meal orders from the school erasure",
		installSchoolErasurePlan, installSchoolErasurePreflight, installSchoolErasureResidual,
		installSchoolErasureDelete, installSchoolErasurePreview)
}

func installSchoolErasureSteps(ctx context.Context, db *bun.DB, action string, steps ...func(context.Context, bun.Tx) error) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		for _, step := range steps {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("%s: %w", action, err)
			}
		}
		return nil
	})
}

// installDerivedSchoolErasureFunctions reinstalls school erasure functions a later
// migration derived from an earlier version by checked edits (1.15.956,
// #6854), in the caller's transaction.
func installDerivedSchoolErasureFunctions(ctx context.Context, tx bun.Tx, functions []string) error {
	for _, function := range functions {
		if _, err := tx.ExecContext(ctx, function); err != nil {
			return err
		}
	}
	return nil
}
