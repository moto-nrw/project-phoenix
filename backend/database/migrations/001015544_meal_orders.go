package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	mealOrdersVersion     = "1.15.544"
	mealOrdersDescription = "Mittagessen: Bestellung zur Frist einfrieren und spätere Änderungen nummeriert festhalten (#3873)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     mealOrdersVersion,
		Description: mealOrdersDescription,
		DependsOn:   []string{mealPermissionsVersion},
	})
	Migrations.MustRegister(mealOrdersUp, mealOrdersDown)
}

// mealOrdersUp adds the order of a school (#3873). An order fixes the meal
// days of a day or a week at the deadline; every later change is a numbered
// change of it. A day belongs to one order only, so changing the rhythm never
// orders a day twice. A frozen order never changes afterwards: deleting a
// child keeps its lines and changes with their class and only clears the
// child (ON DELETE SET NULL), so the counts the caterer saw stay. Changes
// are numbered from a counter per school that never goes back, so a number
// the caterer saw is never given again.
func mealOrdersUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, createMealOrderTablesSQL); err != nil {
			return fmt.Errorf("create meal order tables: %w", err)
		}
		if err := provisionTenantRLS(ctx, tx,
			"schedule.meal_orders",
			"schedule.meal_order_days",
			"schedule.meal_order_lines",
			"schedule.meal_order_changes",
			"schedule.meal_order_change_counters",
		); err != nil {
			return fmt.Errorf("provision meal order tenant isolation: %w", err)
		}
		return nil
	})
}

func mealOrdersDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `
		DROP TABLE IF EXISTS schedule.meal_order_change_counters;
		DROP TABLE IF EXISTS schedule.meal_order_changes;
		DROP TABLE IF EXISTS schedule.meal_order_lines;
		DROP TABLE IF EXISTS schedule.meal_order_days;
		DROP TABLE IF EXISTS schedule.meal_orders;
	`); err != nil {
		return fmt.Errorf("drop meal order tables: %w", err)
	}
	return nil
}

const createMealOrderTablesSQL = `
			CREATE TABLE schedule.meal_orders (
				id BIGSERIAL PRIMARY KEY,
				tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
				period_start DATE NOT NULL,
				period_end DATE NOT NULL,
				deadline_at TIMESTAMPTZ NOT NULL,
				frozen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				format_version SMALLINT NOT NULL DEFAULT 1,
				confirmed_at TIMESTAMPTZ,
				confirmed_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CONSTRAINT uq_meal_order_period UNIQUE (tenant_id, period_start),
				CONSTRAINT uq_meal_order_tenant_id UNIQUE (tenant_id, id),
				CONSTRAINT chk_meal_order_period CHECK (period_end >= period_start)
			);
			COMMENT ON TABLE schedule.meal_orders IS
				'Bestellung einer Schule für einen Tag oder eine Woche, eingefroren zur Frist (#3873).';

			CREATE TABLE schedule.meal_order_days (
				tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
				order_id BIGINT NOT NULL,
				date DATE NOT NULL,
				PRIMARY KEY (tenant_id, date),
				CONSTRAINT fk_meal_order_day_order FOREIGN KEY (tenant_id, order_id)
					REFERENCES schedule.meal_orders(tenant_id, id) ON DELETE CASCADE
			);
			CREATE INDEX idx_meal_order_days_order ON schedule.meal_order_days (tenant_id, order_id);

			CREATE TABLE schedule.meal_order_lines (
				id BIGSERIAL PRIMARY KEY,
				tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
				order_id BIGINT NOT NULL,
				date DATE NOT NULL,
				student_id BIGINT,
				school_class TEXT NOT NULL DEFAULT '',
				CONSTRAINT uq_meal_order_line_child UNIQUE (tenant_id, date, student_id),
				CONSTRAINT fk_meal_order_line_day FOREIGN KEY (tenant_id, date)
					REFERENCES schedule.meal_order_days(tenant_id, date) ON DELETE CASCADE,
				CONSTRAINT fk_meal_order_line_order FOREIGN KEY (tenant_id, order_id)
					REFERENCES schedule.meal_orders(tenant_id, id) ON DELETE CASCADE,
				CONSTRAINT fk_meal_order_line_student FOREIGN KEY (tenant_id, student_id)
					REFERENCES users.student_profiles(tenant_id, id) ON DELETE SET NULL (student_id)
			);
			CREATE INDEX idx_meal_order_lines_order ON schedule.meal_order_lines (tenant_id, order_id);
			COMMENT ON COLUMN schedule.meal_order_lines.student_id IS
				'NULL once the child is deleted: the ordered meal and its class stay (#3873).';

			CREATE TABLE schedule.meal_order_changes (
				id BIGSERIAL PRIMARY KEY,
				tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
				order_id BIGINT NOT NULL,
				sequence INTEGER NOT NULL,
				date DATE NOT NULL,
				student_id BIGINT,
				school_class TEXT NOT NULL DEFAULT '',
				delta SMALLINT NOT NULL,
				detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				changed_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
				CONSTRAINT uq_meal_order_change_sequence UNIQUE (tenant_id, sequence),
				CONSTRAINT chk_meal_order_change_delta CHECK (delta IN (-1, 1)),
				CONSTRAINT fk_meal_order_change_day FOREIGN KEY (tenant_id, date)
					REFERENCES schedule.meal_order_days(tenant_id, date) ON DELETE CASCADE,
				CONSTRAINT fk_meal_order_change_order FOREIGN KEY (tenant_id, order_id)
					REFERENCES schedule.meal_orders(tenant_id, id) ON DELETE CASCADE,
				CONSTRAINT fk_meal_order_change_student FOREIGN KEY (tenant_id, student_id)
					REFERENCES users.student_profiles(tenant_id, id) ON DELETE SET NULL (student_id)
			);
			CREATE INDEX idx_meal_order_changes_date ON schedule.meal_order_changes (tenant_id, date, student_id);
			COMMENT ON TABLE schedule.meal_order_changes IS
				'Nachmeldung: ein Kind mehr oder weniger an einem Tag nach der Frist, je Schule fortlaufend nummeriert (#3873).';
			COMMENT ON COLUMN schedule.meal_order_changes.student_id IS
				'NULL once the child is deleted: the change and its class stay (#3873).';

			CREATE TABLE schedule.meal_order_change_counters (
				tenant_id BIGINT PRIMARY KEY REFERENCES platform.schools(id) ON DELETE CASCADE,
				last_sequence INTEGER NOT NULL,
				CONSTRAINT chk_meal_order_change_counter CHECK (last_sequence > 0)
			);
			COMMENT ON TABLE schedule.meal_order_change_counters IS
				'Letzte Nummer der Nachmeldungen einer Schule; sie sinkt nie, auch wenn eine Nachmeldung wegfällt (#3873).';

			GRANT SELECT, INSERT, UPDATE ON schedule.meal_orders TO phoenix_tenant;
			GRANT SELECT, INSERT ON schedule.meal_order_days TO phoenix_tenant;
			GRANT SELECT, INSERT ON schedule.meal_order_lines TO phoenix_tenant;
			GRANT SELECT, INSERT ON schedule.meal_order_changes TO phoenix_tenant;
			GRANT SELECT, INSERT, UPDATE ON schedule.meal_order_change_counters TO phoenix_tenant;
			-- The counter only rises; the schema's default privileges would
			-- let a school remove it and number from 1 again.
			REVOKE DELETE, TRUNCATE ON schedule.meal_order_change_counters FROM phoenix_tenant;
			GRANT USAGE ON SEQUENCE schedule.meal_orders_id_seq TO phoenix_tenant;
			GRANT USAGE ON SEQUENCE schedule.meal_order_lines_id_seq TO phoenix_tenant;
			GRANT USAGE ON SEQUENCE schedule.meal_order_changes_id_seq TO phoenix_tenant;
		`
