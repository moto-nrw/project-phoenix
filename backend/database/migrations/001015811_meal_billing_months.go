package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const mealBillingMonthsVersion = "1.15.811"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     mealBillingMonthsVersion,
		Description: "Mensa: Monatsfreigabe der OGS und Abrechnung als Pauschale oder je Essen im Monatslauf (#4686)",
		DependsOn: []string{
			parentBillingCareMonthsVersion,         // the monthly runs and invoices the meal positions join
			demoVisitorErasureContractPushVersion, // the erasure this one extends, and the last erasure before it
		},
	})
	Migrations.MustRegister(mealBillingMonthsUp, mealBillingMonthsDown)
}

// mealBillingMonthsUpSQL adds the meal billing of #4686 (ADR 0166):
//
//   - schedule.meal_billing_months: the Mensa-Monatsfreigabe of Meal Plan.
//     The OGS-Leitung confirms in its school transaction that attendance
//     and the parents' cancellations of a month are right; the release
//     keeps the count per child as it was then and never changes. The row
//     carries the school and its Träger (organization_id) instead of a
//     tenant_id: it is billing evidence of the Träger and outlives the
//     school like the invoices it feeds (ADR 0088 section 9). Only the
//     school reads and writes it; the policy checks that the Träger named
//     is the school's.
//   - schedule.meal_billing_month_children: one child of a release with its
//     registered days and the meals counted (registered and present).
//   - schedule.traeger_meal_billing_children: the only read of the Träger.
//     It answers in a Träger transaction alone, for the releases of that
//     Träger, and only the fields a bill needs: no registered day, no
//     absence, no class.
//   - parent_billing.care_meal_tariffs: how the Träger bills the Mensa of a
//     school from a month on: a flat amount per child and month or a price
//     per meal.
//   - parent_billing.care_meal_positions: a meal month of a child on an
//     issued monthly invoice. A child's meal month is billed once
//     (uq_care_meal_positions_child_month); it never changes.
const mealBillingMonthsUpSQL = `
CREATE FUNCTION schedule.meal_billing_school_organization(p_school_id BIGINT)
	RETURNS BIGINT
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
	SELECT school.organization_id FROM platform.schools AS school
	WHERE school.id = p_school_id AND school.deleted_at IS NULL
	$$;
COMMENT ON FUNCTION schedule.meal_billing_school_organization(BIGINT) IS
	'Träger of a school that is not deleted; NULL without one (#4686).';
REVOKE ALL ON FUNCTION schedule.meal_billing_school_organization(BIGINT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION schedule.meal_billing_school_organization(BIGINT) TO phoenix_tenant;

CREATE TABLE schedule.meal_billing_months (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	school_id BIGINT NOT NULL,
	billing_month DATE NOT NULL,
	released_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	released_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	child_count INT NOT NULL,
	meal_count INT NOT NULL,
	CONSTRAINT uq_meal_billing_months_organization UNIQUE (organization_id, id),
	CONSTRAINT uq_meal_billing_months_month UNIQUE (school_id, billing_month),
	CONSTRAINT chk_meal_billing_months_month CHECK (EXTRACT(DAY FROM billing_month) = 1),
	CONSTRAINT chk_meal_billing_months_counts CHECK (child_count >= 0 AND meal_count >= 0)
);
CREATE INDEX idx_meal_billing_months_traeger ON schedule.meal_billing_months (organization_id, billing_month);
COMMENT ON TABLE schedule.meal_billing_months IS
	'Mensa-Monatsfreigabe: the OGS-Leitung confirmed attendance and cancellations of a month (#4686); never changes.';

CREATE TABLE schedule.meal_billing_month_children (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	month_id BIGINT NOT NULL,
	student_id BIGINT NOT NULL,
	first_name TEXT NOT NULL,
	last_name TEXT NOT NULL,
	school_class TEXT NOT NULL DEFAULT '',
	registered_days SMALLINT NOT NULL,
	meal_days SMALLINT NOT NULL,
	CONSTRAINT fk_meal_billing_month_children_month FOREIGN KEY (organization_id, month_id)
		REFERENCES schedule.meal_billing_months (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_meal_billing_month_children_child UNIQUE (month_id, student_id),
	CONSTRAINT chk_meal_billing_month_children_days CHECK (
		registered_days BETWEEN 1 AND 31 AND meal_days BETWEEN 0 AND registered_days)
);
COMMENT ON TABLE schedule.meal_billing_month_children IS
	'A child of a Mensa-Monatsfreigabe with its registered days and the meals counted (#4686); never changes.';

-- A release never changes, except that deleting the account that released
-- it clears released_by_account_id (ON DELETE SET NULL).
CREATE FUNCTION schedule.refuse_meal_billing_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF TG_TABLE_NAME = 'meal_billing_months' AND NEW.released_by_account_id IS NULL
		AND to_jsonb(NEW) - 'released_by_account_id' = to_jsonb(OLD) - 'released_by_account_id' THEN
		RETURN NEW;
	END IF;
	RAISE EXCEPTION 'a released Mensa month never changes (%)', TG_TABLE_NAME
		USING ERRCODE = '55000';
END;
$$;
REVOKE ALL ON FUNCTION schedule.refuse_meal_billing_change() FROM PUBLIC;
CREATE TRIGGER refuse_meal_billing_month_change
	BEFORE UPDATE ON schedule.meal_billing_months
	FOR EACH ROW EXECUTE FUNCTION schedule.refuse_meal_billing_change();
CREATE TRIGGER refuse_meal_billing_month_child_change
	BEFORE UPDATE ON schedule.meal_billing_month_children
	FOR EACH ROW EXECUTE FUNCTION schedule.refuse_meal_billing_change();

ALTER TABLE schedule.meal_billing_months ENABLE ROW LEVEL SECURITY;
ALTER TABLE schedule.meal_billing_months FORCE ROW LEVEL SECURITY;
CREATE POLICY school_isolation_meal_billing_months ON schedule.meal_billing_months FOR ALL
	USING (school_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
	WITH CHECK (school_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
		AND organization_id = schedule.meal_billing_school_organization(school_id));
ALTER TABLE schedule.meal_billing_month_children ENABLE ROW LEVEL SECURITY;
ALTER TABLE schedule.meal_billing_month_children FORCE ROW LEVEL SECURITY;
CREATE POLICY school_isolation_meal_billing_month_children ON schedule.meal_billing_month_children FOR ALL
	USING (EXISTS (SELECT 1 FROM schedule.meal_billing_months AS month
		WHERE month.organization_id = meal_billing_month_children.organization_id
			AND month.id = meal_billing_month_children.month_id))
	WITH CHECK (EXISTS (SELECT 1 FROM schedule.meal_billing_months AS month
		WHERE month.organization_id = meal_billing_month_children.organization_id
			AND month.id = meal_billing_month_children.month_id));

REVOKE ALL ON schedule.meal_billing_months, schedule.meal_billing_month_children FROM PUBLIC;
REVOKE ALL ON schedule.meal_billing_months, schedule.meal_billing_month_children FROM phoenix_tenant;
GRANT SELECT, INSERT ON schedule.meal_billing_months, schedule.meal_billing_month_children TO phoenix_tenant;
GRANT USAGE ON SEQUENCE schedule.meal_billing_months_id_seq, schedule.meal_billing_month_children_id_seq TO phoenix_tenant;

CREATE FUNCTION schedule.traeger_meal_billing_children(p_until DATE)
	RETURNS TABLE (
		school_id BIGINT, billing_month DATE, released_at TIMESTAMPTZ,
		student_id BIGINT, first_name TEXT, last_name TEXT, meal_days SMALLINT
	)
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
	SELECT month.school_id, month.billing_month, month.released_at,
		child.student_id, child.first_name, child.last_name, child.meal_days
	FROM schedule.meal_billing_months AS month
	LEFT JOIN schedule.meal_billing_month_children AS child
		ON child.organization_id = month.organization_id AND child.month_id = month.id
	WHERE month.organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint
		AND NULLIF(current_setting('app.current_tenant_id', true), '') IS NULL
		AND month.billing_month <= p_until
	ORDER BY month.school_id, month.billing_month, child.student_id
	$$;
COMMENT ON FUNCTION schedule.traeger_meal_billing_children(DATE) IS
	'Released Mensa months up to a day for the Träger of app.current_org_id, in a Träger transaction only, with the fields a bill needs (#4686).';
REVOKE ALL ON FUNCTION schedule.traeger_meal_billing_children(DATE) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION schedule.traeger_meal_billing_children(DATE) TO phoenix_tenant;

CREATE TABLE parent_billing.care_meal_tariffs (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	school_id BIGINT NOT NULL,
	valid_from DATE NOT NULL,
	mode VARCHAR(16) NOT NULL,
	amount_cents BIGINT NOT NULL,
	created_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_care_meal_tariffs_month UNIQUE (organization_id, school_id, valid_from),
	CONSTRAINT chk_care_meal_tariffs_month CHECK (EXTRACT(DAY FROM valid_from) = 1),
	CONSTRAINT chk_care_meal_tariffs_mode CHECK (mode IN ('flat', 'per_meal')),
	CONSTRAINT chk_care_meal_tariffs_amount CHECK (amount_cents BETWEEN 0 AND 100000000)
);
COMMENT ON TABLE parent_billing.care_meal_tariffs IS
	'How the Träger bills the Mensa of a school from a month on: flat per child and month or per meal (#4686).';

CREATE TABLE parent_billing.care_meal_positions (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	invoice_id BIGINT NOT NULL,
	position SMALLINT NOT NULL,
	school_id BIGINT NOT NULL,
	student_id BIGINT NOT NULL,
	meal_month DATE NOT NULL,
	mode VARCHAR(16) NOT NULL,
	quantity INT NOT NULL,
	unit_cents BIGINT NOT NULL,
	amount_cents BIGINT NOT NULL,
	net_cents BIGINT NOT NULL,
	tax_cents BIGINT NOT NULL DEFAULT 0,
	CONSTRAINT fk_care_meal_positions_invoice FOREIGN KEY (organization_id, invoice_id)
		REFERENCES parent_billing.care_invoices (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_care_meal_positions_position UNIQUE (invoice_id, position),
	CONSTRAINT uq_care_meal_positions_child_month UNIQUE (organization_id, school_id, student_id, meal_month),
	CONSTRAINT chk_care_meal_positions_month CHECK (EXTRACT(DAY FROM meal_month) = 1),
	CONSTRAINT chk_care_meal_positions_mode CHECK (
		(mode = 'per_meal' AND quantity BETWEEN 1 AND 31 AND amount_cents = quantity * unit_cents)
		OR (mode = 'flat' AND quantity = 1 AND amount_cents = unit_cents)),
	CONSTRAINT chk_care_meal_positions_amounts CHECK (
		unit_cents >= 0 AND amount_cents >= 0 AND amount_cents = net_cents + tax_cents AND tax_cents = 0)
);
COMMENT ON TABLE parent_billing.care_meal_positions IS
	'A Mensa month of a child on an issued monthly invoice (#4686); billed once, never changes.';
CREATE TRIGGER refuse_parent_billing_care_meal_position_change
	BEFORE UPDATE ON parent_billing.care_meal_positions
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_issued_change();

ALTER TABLE parent_billing.care_meal_tariffs ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_meal_tariffs FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_care_meal_tariffs ON parent_billing.care_meal_tariffs FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.care_meal_positions ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_meal_positions FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_care_meal_positions ON parent_billing.care_meal_positions FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.care_meal_tariffs, parent_billing.care_meal_positions FROM PUBLIC;
GRANT SELECT, INSERT, DELETE ON parent_billing.care_meal_tariffs TO phoenix_tenant;
GRANT SELECT, INSERT ON parent_billing.care_meal_positions TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.care_meal_tariffs_id_seq, parent_billing.care_meal_positions_id_seq TO phoenix_tenant;
GRANT ALL ON ALL TABLES IN SCHEMA parent_billing TO phoenix_admin;
GRANT ALL ON ALL SEQUENCES IN SCHEMA parent_billing TO phoenix_admin;
GRANT ALL ON schedule.meal_billing_months, schedule.meal_billing_month_children TO phoenix_admin;
GRANT ALL ON SEQUENCE schedule.meal_billing_months_id_seq, schedule.meal_billing_month_children_id_seq TO phoenix_admin;
`

const mealBillingMonthsDownSQL = `
DROP TABLE IF EXISTS parent_billing.care_meal_positions, parent_billing.care_meal_tariffs;
DROP FUNCTION IF EXISTS schedule.traeger_meal_billing_children(DATE);
DROP TABLE IF EXISTS schedule.meal_billing_month_children, schedule.meal_billing_months;
DROP FUNCTION IF EXISTS schedule.refuse_meal_billing_change();
DROP FUNCTION IF EXISTS schedule.meal_billing_school_organization(BIGINT);
`

// mealBillingErasureAnchor is the statement of the erasure of 1.15.808
// after which the released Mensa months of the demo schools go.
const mealBillingErasureAnchor = `					DELETE FROM parent_billing.care_run_holds
					WHERE school_ids && hidden_schools;
`

func mealBillingMonthsUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, mealBillingMonthsUpSQL); err != nil {
			return fmt.Errorf("create meal billing months: %w", err)
		}
		if _, err := tx.ExecContext(ctx, demoVisitorErasureMealBillingSQL); err != nil {
			return fmt.Errorf("erase the Mensa months of retired demo schools: %w", err)
		}
		return nil
	})
}

// mealBillingMonthsDown restores the erasure of 1.15.808 before it drops
// the tables that erasure no longer names.
func mealBillingMonthsDown(ctx context.Context, db *bun.DB) error {
	if err := demoVisitorErasureContractPushUp(ctx, db); err != nil {
		return fmt.Errorf("restore the demo visitor erasure of 1.15.808: %w", err)
	}
	if _, err := db.ExecContext(ctx, mealBillingMonthsDownSQL); err != nil {
		return fmt.Errorf("drop meal billing months: %w", err)
	}
	return nil
}
