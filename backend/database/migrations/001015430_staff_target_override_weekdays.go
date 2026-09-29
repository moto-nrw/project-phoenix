package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffTargetOverrideWeekdaysVersion     = "1.15.430"
	staffTargetOverrideWeekdaysDescription = "Add weekday_minutes to config.staff_target_overrides - a Sonderarbeitszeit with its own target per weekday (#3745)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffTargetOverrideWeekdaysVersion,
		Description: staffTargetOverrideWeekdaysDescription,
		DependsOn:   []string{staffTargetOverridesVersion, guardianOwnerContractVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return staffTargetOverrideWeekdaysUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return staffTargetOverrideWeekdaysDown(ctx, db)
		},
	)
}

// staffTargetOverrideWeekdaysUp lets a Sonderarbeitszeit carry one target per
// weekday (#3745): weekday_minutes holds Monday to Friday. A row has either
// the one daily_minutes of #3259 or weekday_minutes, never both, so existing
// rows keep their meaning unchanged. An element comparison with ALL() is NULL
// for a NULL element and would pass the CHECK, hence the explicit
// array_position test.
func staffTargetOverrideWeekdaysUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE config.staff_target_overrides
			ADD COLUMN IF NOT EXISTS weekday_minutes INTEGER[],
			ALTER COLUMN daily_minutes DROP NOT NULL;

		ALTER TABLE config.staff_target_overrides
			DROP CONSTRAINT IF EXISTS staff_target_overrides_minutes_ok,
			DROP CONSTRAINT IF EXISTS staff_target_overrides_weekday_minutes_ok,
			DROP CONSTRAINT IF EXISTS staff_target_overrides_one_target;

		ALTER TABLE config.staff_target_overrides
			ADD CONSTRAINT staff_target_overrides_minutes_ok
				CHECK (daily_minutes IS NULL OR daily_minutes BETWEEN 0 AND 720),
			ADD CONSTRAINT staff_target_overrides_weekday_minutes_ok
				CHECK (weekday_minutes IS NULL OR (
					array_ndims(weekday_minutes) = 1
					AND cardinality(weekday_minutes) = 5
					AND array_position(weekday_minutes, NULL) IS NULL
					AND 0 <= ALL(weekday_minutes)
					AND 720 >= ALL(weekday_minutes)
				)),
			ADD CONSTRAINT staff_target_overrides_one_target
				CHECK ((daily_minutes IS NULL) <> (weekday_minutes IS NULL));
	`)
	if err != nil {
		return fmt.Errorf("error adding weekday_minutes to config.staff_target_overrides: %w", err)
	}

	return tx.Commit()
}

// staffTargetOverrideWeekdaysDown restores the former daily target only for
// uniform weekday rows. A non-uniform row cannot be represented by the former
// schema without changing its day-specific targets, so the rollback stops
// before it changes data.
func staffTargetOverrideWeekdaysDown(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		LOCK TABLE config.staff_target_overrides IN ACCESS EXCLUSIVE MODE;
	`)
	if err != nil {
		return fmt.Errorf("lock config.staff_target_overrides for rollback: %w", err)
	}

	var hasNonUniformWeekdayTargets bool
	if err := tx.NewRaw(`
		SELECT EXISTS (
			SELECT 1
			FROM config.staff_target_overrides
			WHERE weekday_minutes IS NOT NULL
				AND NOT (weekday_minutes[1] = ALL(weekday_minutes))
		)
	`).Scan(ctx, &hasNonUniformWeekdayTargets); err != nil {
		return fmt.Errorf("check weekday targets before rollback: %w", err)
	}
	if hasNonUniformWeekdayTargets {
		return fmt.Errorf("cannot roll back weekday target overrides: non-uniform weekday targets cannot be represented by daily_minutes")
	}

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE config.staff_target_overrides
			DROP CONSTRAINT IF EXISTS staff_target_overrides_one_target,
			DROP CONSTRAINT IF EXISTS staff_target_overrides_weekday_minutes_ok;

		UPDATE config.staff_target_overrides
		SET daily_minutes = weekday_minutes[1]
		WHERE weekday_minutes IS NOT NULL;

		ALTER TABLE config.staff_target_overrides
			DROP COLUMN IF EXISTS weekday_minutes,
			ALTER COLUMN daily_minutes SET NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error dropping weekday_minutes from config.staff_target_overrides: %w", err)
	}
	return tx.Commit()
}
