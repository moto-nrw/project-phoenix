package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

const mealPlanAllergensVersion = "1.15.497"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     mealPlanAllergensVersion,
		Description: "Speiseplan: die 14 Hauptallergene je Gericht (#4777)",
		DependsOn: []string{
			enrollmentRequestUploadsVersion, // the last migration before this one
			mealPlanEntriesVersion,          // schedule.meal_plan_entries
		},
	})
	Migrations.MustRegister(mealPlanAllergensUp, mealPlanAllergensDown)
}

// mealPlanAllergensUpSQL adds the allergens of a dish (#4777). The column
// holds the codes of the 14 main allergens the Meal Plan module lists in
// mealplan.Allergens; the CHECK keeps any other value out. Existing dishes
// get an empty list: they stay valid without any marking.
const mealPlanAllergensUpSQL = `
	ALTER TABLE schedule.meal_plan_entries
		ADD COLUMN IF NOT EXISTS allergens TEXT[] NOT NULL DEFAULT '{}';
	ALTER TABLE schedule.meal_plan_entries
		DROP CONSTRAINT IF EXISTS chk_meal_plan_entries_allergens;
	ALTER TABLE schedule.meal_plan_entries
		ADD CONSTRAINT chk_meal_plan_entries_allergens CHECK (
			allergens <@ ARRAY[
				'gluten', 'crustaceans', 'eggs', 'fish', 'peanuts', 'soybeans', 'milk',
				'nuts', 'celery', 'mustard', 'sesame', 'sulphites', 'lupin', 'molluscs'
			]::TEXT[]
		);
`

const mealPlanAllergensDownSQL = `
	ALTER TABLE schedule.meal_plan_entries DROP CONSTRAINT IF EXISTS chk_meal_plan_entries_allergens;
	ALTER TABLE schedule.meal_plan_entries DROP COLUMN IF EXISTS allergens;
`

func mealPlanAllergensUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackMealPlanAllergens(ctx, tx)
	if _, err := tx.ExecContext(ctx, mealPlanAllergensUpSQL); err != nil {
		return fmt.Errorf("add meal plan allergens: %w", err)
	}
	return tx.Commit()
}

func mealPlanAllergensDown(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackMealPlanAllergens(ctx, tx)
	if _, err := tx.ExecContext(ctx, mealPlanAllergensDownSQL); err != nil {
		return fmt.Errorf("drop meal plan allergens: %w", err)
	}
	return tx.Commit()
}

func rollbackMealPlanAllergens(ctx context.Context, tx bun.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		logRollbackFailure(ctx, err)
	}
}
