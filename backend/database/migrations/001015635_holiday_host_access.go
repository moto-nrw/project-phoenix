package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayHostAccessVersion = "1.15.635"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayHostAccessVersion,
		Description: "Ausrichtende OGS liest die bestätigten Kinder ihrer Ferienwochen bis zum Ende der Woche (#4665)",
		DependsOn: []string{
			holidayRegistrationReviewVersion, // the last migration before this one, with the confirmed state
		},
	})
	Migrations.MustRegister(holidayHostAccessUp, holidayHostAccessDown)
}

// holidayHostAccessUpSQL lets the school that hosts a holiday week read its
// participants in its own tenant transaction (#4665, spec #4611 user stories
// 41 and 42, ADR 0116). It adds the second, read-only policy of ADR 0086
// section 5 to the Träger's holiday tables, narrowed to what the hosting OGS
// needs:
//
//   - a week of a published period the school hosts, while its last day has
//     not passed (Berlin calendar day), and the period and time models of
//     that week;
//   - the bookings of that week whose registration is confirmed, and the
//     children of those bookings.
//
// The school sees no registration, no contact and no audit entry: the
// parents' data, the consents and the review stay with the Träger. No policy
// admits a write. Access to a child of another school therefore exists only
// through a confirmed booking for a week the school hosts, and it ends after
// that week without anyone revoking it. Two SECURITY DEFINER functions decide
// a week and a booking; they read the period, the week and the registration
// without the Träger policy and so keep the policies free of cycles. They
// answer false outside a school transaction, so a Träger transaction keeps
// exactly its own policy.
const holidayHostAccessUpSQL = `
CREATE FUNCTION enrollment.holiday_host_week(p_organization_id BIGINT, p_week_id BIGINT)
	RETURNS BOOLEAN
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
	SELECT EXISTS (
		SELECT 1
		FROM enrollment.holiday_period_weeks AS week
		JOIN enrollment.holiday_periods AS period
			ON period.organization_id = week.organization_id AND period.id = week.holiday_period_id
		JOIN platform.schools AS school
			ON school.id = week.host_school_id AND school.organization_id = week.organization_id
		WHERE week.organization_id = p_organization_id AND week.id = p_week_id
			AND week.host_school_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
			AND period.status = 'published'
			AND week.ends_on >= (CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date
	)
	$$;
COMMENT ON FUNCTION enrollment.holiday_host_week(BIGINT, BIGINT) IS
	'True while the school of app.current_tenant_id hosts this week of a published period and its last day has not passed (#4665, ADR 0116).';

CREATE FUNCTION enrollment.holiday_host_booking(p_organization_id BIGINT, p_week_id BIGINT, p_child_id BIGINT)
	RETURNS BOOLEAN
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
	SELECT enrollment.holiday_host_week(p_organization_id, p_week_id) AND EXISTS (
		SELECT 1
		FROM enrollment.holiday_registration_children AS child
		JOIN enrollment.holiday_registrations AS registration
			ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
		WHERE child.organization_id = p_organization_id AND child.id = p_child_id
			AND registration.status = 'confirmed'
	)
	$$;
COMMENT ON FUNCTION enrollment.holiday_host_booking(BIGINT, BIGINT, BIGINT) IS
	'True for a booking of a confirmed registration in a week the school of app.current_tenant_id hosts (#4665, ADR 0116).';

REVOKE ALL ON FUNCTION enrollment.holiday_host_week(BIGINT, BIGINT) FROM PUBLIC;
REVOKE ALL ON FUNCTION enrollment.holiday_host_booking(BIGINT, BIGINT, BIGINT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION enrollment.holiday_host_week(BIGINT, BIGINT) TO phoenix_tenant;
GRANT EXECUTE ON FUNCTION enrollment.holiday_host_booking(BIGINT, BIGINT, BIGINT) TO phoenix_tenant;

CREATE POLICY host_school_reads_enrollment_holiday_period_weeks ON enrollment.holiday_period_weeks
	FOR SELECT
	USING (NULLIF(current_setting('app.current_tenant_id', true), '') IS NOT NULL
		AND enrollment.holiday_host_week(organization_id, id));
CREATE POLICY host_school_reads_enrollment_holiday_periods ON enrollment.holiday_periods
	FOR SELECT
	USING (NULLIF(current_setting('app.current_tenant_id', true), '') IS NOT NULL
		AND EXISTS (
			SELECT 1 FROM enrollment.holiday_period_weeks AS week
			WHERE week.organization_id = holiday_periods.organization_id AND week.holiday_period_id = holiday_periods.id
		));
CREATE POLICY host_school_reads_enrollment_holiday_week_time_models ON enrollment.holiday_week_time_models
	FOR SELECT
	USING (NULLIF(current_setting('app.current_tenant_id', true), '') IS NOT NULL
		AND enrollment.holiday_host_week(organization_id, holiday_period_week_id));
CREATE POLICY host_school_reads_enrollment_holiday_registration_bookings ON enrollment.holiday_registration_bookings
	FOR SELECT
	USING (NULLIF(current_setting('app.current_tenant_id', true), '') IS NOT NULL
		AND enrollment.holiday_host_booking(organization_id, holiday_period_week_id, holiday_registration_child_id));
CREATE POLICY host_school_reads_enrollment_holiday_registration_children ON enrollment.holiday_registration_children
	FOR SELECT
	USING (NULLIF(current_setting('app.current_tenant_id', true), '') IS NOT NULL
		AND EXISTS (
			SELECT 1 FROM enrollment.holiday_registration_bookings AS booking
			WHERE booking.organization_id = holiday_registration_children.organization_id
				AND booking.holiday_registration_child_id = holiday_registration_children.id
		));
`

const holidayHostAccessDownSQL = `
DROP POLICY IF EXISTS host_school_reads_enrollment_holiday_registration_children ON enrollment.holiday_registration_children;
DROP POLICY IF EXISTS host_school_reads_enrollment_holiday_registration_bookings ON enrollment.holiday_registration_bookings;
DROP POLICY IF EXISTS host_school_reads_enrollment_holiday_week_time_models ON enrollment.holiday_week_time_models;
DROP POLICY IF EXISTS host_school_reads_enrollment_holiday_periods ON enrollment.holiday_periods;
DROP POLICY IF EXISTS host_school_reads_enrollment_holiday_period_weeks ON enrollment.holiday_period_weeks;
DROP FUNCTION IF EXISTS enrollment.holiday_host_booking(BIGINT, BIGINT, BIGINT);
DROP FUNCTION IF EXISTS enrollment.holiday_host_week(BIGINT, BIGINT);
`

func holidayHostAccessUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayHostAccessUpSQL); err != nil {
		return fmt.Errorf("add holiday host access: %w", err)
	}
	return nil
}

func holidayHostAccessDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayHostAccessDownSQL); err != nil {
		return fmt.Errorf("drop holiday host access: %w", err)
	}
	return nil
}
