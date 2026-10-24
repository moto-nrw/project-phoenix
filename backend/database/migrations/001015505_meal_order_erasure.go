package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	mealOrderErasureVersion     = "1.15.505"
	mealOrderErasureDescription = "Mandantenlöschung: Bestellungen, Anmeldeänderungen, Caterer-Einladungen und Belieferungen einer Schule (#3873, #3837)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     mealOrderErasureVersion,
		Description: mealOrderErasureDescription,
		DependsOn:   []string{mealCaterersVersion, enrollmentRequestUploadsVersion},
	})
	Migrations.MustRegister(mealOrderErasureUp, mealOrderErasureDown)
}

// The installed erasure plan is a frozen projection of the data lifecycle
// catalog (ADR 0092): a new school-scoped table needs a migration updating
// the school erasure functions. Up installs them with the seven
// school-scoped tables of the meal ordering (#3873); Down installs them as
// development's latest migration (1.15.491) left them. The caterer business
// and its accounts are shared across schools and stay (ADR 0077).
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
