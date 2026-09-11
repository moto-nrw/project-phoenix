package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const requestChildStorageExpandVersion = "1.15.375"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     requestChildStorageExpandVersion,
		Description: "Expand submitted offering selections and effective care bookings without dual writes (#2712)",
		DependsOn:   []string{requestChildOfferingValidityNonemptyVersion, createTenantRolesVersion},
	})
	Migrations.MustRegister(requestChildStorageExpandUp, requestChildStorageExpandDown)
}

func requestChildStorageExpandUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// These supporting indexes are required for tenant-safe foreign keys.
		// They change neither the mixed legacy table nor any application writer.
		_, err := tx.ExecContext(ctx, `
			CREATE UNIQUE INDEX request_children_expand_tenant_id ON enrollment.request_children (tenant_id, id);
			CREATE UNIQUE INDEX care_offerings_expand_tenant_id ON enrollment.care_offerings (tenant_id, id);

			CREATE TABLE enrollment.request_child_offering_selections (
				id BIGSERIAL PRIMARY KEY,
				tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
				request_child_id BIGINT NOT NULL,
				care_offering_id BIGINT NOT NULL,
				selected_days JSONB,
				notes TEXT,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE (tenant_id, request_child_id, care_offering_id),
				FOREIGN KEY (tenant_id, request_child_id) REFERENCES enrollment.request_children(tenant_id, id) ON DELETE CASCADE,
				FOREIGN KEY (tenant_id, care_offering_id) REFERENCES enrollment.care_offerings(tenant_id, id) ON DELETE RESTRICT
			);
			CREATE INDEX offering_selections_offering ON enrollment.request_child_offering_selections (tenant_id, care_offering_id);

			CREATE TABLE enrollment.care_offering_bookings (
				id BIGSERIAL PRIMARY KEY,
				tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
				request_child_id BIGINT NOT NULL,
				care_offering_id BIGINT NOT NULL,
				manual_selected_days JSONB,
				automatic_selected_days JSONB,
				valid_from DATE,
				valid_until DATE,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				FOREIGN KEY (tenant_id, request_child_id) REFERENCES enrollment.request_children(tenant_id, id) ON DELETE CASCADE,
				FOREIGN KEY (tenant_id, care_offering_id) REFERENCES enrollment.care_offerings(tenant_id, id) ON DELETE RESTRICT,
				CONSTRAINT care_offering_bookings_nonempty_validity CHECK (valid_from IS NULL OR valid_until IS NULL OR valid_from < valid_until),
				CONSTRAINT care_offering_bookings_non_overlapping_validity EXCLUDE USING gist (
					tenant_id WITH =, request_child_id WITH =, care_offering_id WITH =,
					daterange(COALESCE(valid_from, '-infinity'::date), COALESCE(valid_until, 'infinity'::date), '[)') WITH &&
				)
			);
			CREATE INDEX care_offering_bookings_active_validity ON enrollment.care_offering_bookings (tenant_id, care_offering_id, valid_from, valid_until);
			GRANT SELECT, INSERT, DELETE ON enrollment.request_child_offering_selections TO phoenix_tenant;
			REVOKE UPDATE, TRUNCATE ON enrollment.request_child_offering_selections FROM phoenix_tenant;
			GRANT SELECT, INSERT, UPDATE, DELETE ON enrollment.care_offering_bookings TO phoenix_tenant;
			GRANT ALL ON enrollment.request_child_offering_selections, enrollment.care_offering_bookings TO phoenix_admin;
			GRANT USAGE ON SEQUENCE enrollment.request_child_offering_selections_id_seq, enrollment.care_offering_bookings_id_seq TO phoenix_tenant;
			GRANT ALL ON SEQUENCE enrollment.request_child_offering_selections_id_seq, enrollment.care_offering_bookings_id_seq TO phoenix_admin;
		`)
		if err != nil {
			return fmt.Errorf("expand request child storage: %w", err)
		}
		return provisionTenantRLS(ctx, tx, "enrollment.request_child_offering_selections", "enrollment.care_offering_bookings")
	})
}

func requestChildStorageExpandDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Expand rollback is safe only before Cutover populates these tables.
		// Keep the emptiness check and drops under the same exclusive locks.
		_, err := tx.ExecContext(ctx, `
			LOCK TABLE enrollment.request_child_offering_selections, enrollment.care_offering_bookings IN ACCESS EXCLUSIVE MODE;
			DO $$ BEGIN
				IF EXISTS (SELECT FROM enrollment.request_child_offering_selections)
					OR EXISTS (SELECT FROM enrollment.care_offering_bookings) THEN
					RAISE EXCEPTION 'cannot roll back request child storage Expand: target tables are not empty';
				END IF;
			END $$;
			DROP TABLE enrollment.care_offering_bookings;
			DROP TABLE enrollment.request_child_offering_selections;
			DROP INDEX enrollment.care_offerings_expand_tenant_id;
			DROP INDEX enrollment.request_children_expand_tenant_id;
		`)
		if err != nil {
			return fmt.Errorf("roll back request child storage Expand: %w", err)
		}
		return nil
	})
}
