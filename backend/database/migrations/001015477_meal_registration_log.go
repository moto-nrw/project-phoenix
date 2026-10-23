package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	mealRegistrationLogVersion     = "1.15.477"
	mealRegistrationLogDescription = "Mittagessen: eigenes Recht meals:register für Änderungen durch die Schule und Protokoll jeder Anmeldungsänderung (#3873)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     mealRegistrationLogVersion,
		Description: mealRegistrationLogDescription,
		DependsOn:   []string{mealOrdersVersion},
	})
	Migrations.MustRegister(mealRegistrationLogUp, mealRegistrationLogDown)
}

// mealRegistrationLogUp separates seeing the lists with names (meals:orders)
// from changing a child's registration for the school (meals:register), and
// logs every change of a registration, by the school or a parent (#3873).
//
// No role gets meals:register: changing a registration after the deadline
// never existed before, so a school grants it on purpose. Admins hold it
// through admin:*. The log only grows and goes with the child, like the
// lines and changes of an order; an order itself changes only when the
// caterer confirms it.
func mealRegistrationLogUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO auth.permissions (name, description, resource, action, is_system) VALUES
				('meals:register', 'Change a child''s meal registration for the school, also after the deadline', 'meals', 'register', TRUE)
			ON CONFLICT (name) DO NOTHING;
			UPDATE auth.permissions SET description = 'See meal lists with children''s names'
			WHERE name = 'meals:orders';

			CREATE TABLE schedule.meal_registration_changes (
				id BIGSERIAL PRIMARY KEY,
				tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
				student_id BIGINT NOT NULL,
				account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
				by_school BOOLEAN NOT NULL,
				kind TEXT NOT NULL,
				date DATE NOT NULL,
				participating BOOLEAN,
				weekdays SMALLINT[],
				changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CONSTRAINT chk_meal_registration_change_kind CHECK (kind IN ('day', 'regular', 'schedule')),
				CONSTRAINT fk_meal_registration_change_student FOREIGN KEY (tenant_id, student_id)
					REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE
			);
			CREATE INDEX idx_meal_registration_changes_student ON schedule.meal_registration_changes (tenant_id, student_id, changed_at);
			COMMENT ON TABLE schedule.meal_registration_changes IS
				'Protokoll jeder Änderung einer Essensanmeldung durch Schule oder Eltern (#3873).';

			GRANT SELECT, INSERT ON schedule.meal_registration_changes TO phoenix_tenant;
			GRANT USAGE ON SEQUENCE schedule.meal_registration_changes_id_seq TO phoenix_tenant;
			-- The log, the order days, lines and changes only grow; the
			-- schema's default privileges would let a school rewrite them.
			REVOKE UPDATE, DELETE, TRUNCATE ON schedule.meal_registration_changes FROM phoenix_tenant;
			REVOKE UPDATE, DELETE, TRUNCATE ON schedule.meal_order_days FROM phoenix_tenant;
			REVOKE UPDATE, DELETE, TRUNCATE ON schedule.meal_order_lines FROM phoenix_tenant;
			REVOKE UPDATE, DELETE, TRUNCATE ON schedule.meal_order_changes FROM phoenix_tenant;
			REVOKE DELETE, TRUNCATE ON schedule.meal_orders FROM phoenix_tenant;
		`); err != nil {
			return fmt.Errorf("create meal registration log: %w", err)
		}
		if err := provisionTenantRLS(ctx, tx, "schedule.meal_registration_changes"); err != nil {
			return fmt.Errorf("provision meal registration log tenant isolation: %w", err)
		}
		return nil
	})
}

func mealRegistrationLogDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `
		DROP TABLE IF EXISTS schedule.meal_registration_changes;
		DELETE FROM auth.account_permissions WHERE permission_id IN (SELECT id FROM auth.permissions WHERE name = 'meals:register');
		UPDATE auth.permissions SET description = 'See meal lists with children''s names and change a registration after the deadline'
		WHERE name = 'meals:orders';
	`); err != nil {
		return fmt.Errorf("drop meal registration log: %w", err)
	}
	return dropPermission(ctx, db, "meals:register")
}
