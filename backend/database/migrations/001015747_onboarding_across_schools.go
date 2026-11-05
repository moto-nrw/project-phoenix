package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const onboardingAcrossSchoolsVersion = "1.15.747"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     onboardingAcrossSchoolsVersion,
		Description: "Onboarding gilt schulübergreifend: erste Schritte je Konto, Ausblenden beim Absprung je Träger (#5659)",
		DependsOn: []string{
			holidaySiblingGroupsVersion, // the last migration before this one
			schoolSetupsVersion,         // the dismissals one function reads
			staffOnboardingsVersion,     // the progress the other function reads
			traegerPortalVersion,        // the Träger membership both directions need
		},
	})
	Migrations.MustRegister(onboardingAcrossSchoolsUp, onboardingAcrossSchoolsDown)
}

// onboardingAcrossSchoolsUpSQL lets a school's tenant transaction see what the
// calling person did in their other schools (#5659, ADR 0043 addendum). The
// rows stay per school, so school erasure and account erasure keep working
// unchanged, and no new table is needed.
//
//   - Every account's staff_onboardings rows are merged once. From here on a
//     write carries the person's whole state into the school it happens in, so
//     the newest row is the person's state.
//   - config.staff_onboarding_across_schools returns that newest row. It answers
//     only for an account with active access to the session's school, either
//     directly or through the Träger session that selected it.
//   - config.school_setup_dismissed_in_traeger reports whether the account hid
//     the school wizard in any school of the session school's Träger. It answers
//     only for an active member of that Träger.
//
// Both are SECURITY DEFINER because RLS rightly hides other schools' rows from
// the tenant role. They are no general way around it: each returns one person's
// onboarding state and checks the relationship itself, and neither returns
// anything outside a school transaction.
const onboardingAcrossSchoolsUpSQL = `
WITH merged AS (
	SELECT
		person.account_id,
		COALESCE((
			SELECT array_agg(DISTINCT step ORDER BY step)
			FROM config.staff_onboardings AS row_of_person
			CROSS JOIN LATERAL unnest(row_of_person.done_steps) AS step
			WHERE row_of_person.account_id = person.account_id
		), '{}') AS done_steps,
		MIN(person.dismissed_at) AS dismissed_at
	FROM config.staff_onboardings AS person
	GROUP BY person.account_id
	HAVING COUNT(*) > 1
),
merged_with_skips AS (
	SELECT
		merged.*,
		COALESCE((
			SELECT array_agg(DISTINCT step ORDER BY step)
			FROM config.staff_onboardings AS row_of_person
			CROSS JOIN LATERAL unnest(row_of_person.skipped_steps) AS step
			WHERE row_of_person.account_id = merged.account_id
				AND NOT step = ANY (merged.done_steps)
		), '{}') AS skipped_steps
	FROM merged
)
UPDATE config.staff_onboardings AS target
SET done_steps = merged_with_skips.done_steps,
	skipped_steps = merged_with_skips.skipped_steps,
	dismissed_at = merged_with_skips.dismissed_at
FROM merged_with_skips
WHERE target.account_id = merged_with_skips.account_id;

CREATE FUNCTION config.staff_onboarding_across_schools(p_account_id BIGINT)
	RETURNS TABLE (done_steps TEXT[], skipped_steps TEXT[], dismissed_at TIMESTAMPTZ)
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
	SELECT progress.done_steps, progress.skipped_steps, progress.dismissed_at
	FROM config.staff_onboardings AS progress
	WHERE progress.account_id = p_account_id
		AND EXISTS (
				SELECT FROM platform.schools AS session_school
				WHERE session_school.id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
					AND session_school.active
					AND session_school.deleted_at IS NULL
					AND (
						EXISTS (
							SELECT FROM auth.account_tenants AS access
							WHERE access.account_id = p_account_id
								AND access.tenant_id = session_school.id
								AND access.status = 'active'
						)
						OR EXISTS (
							SELECT FROM platform.organization_accounts AS membership
							WHERE membership.organization_id = session_school.organization_id
								AND membership.account_id = p_account_id
								AND membership.status = 'active'
						)
					)
		)
	ORDER BY progress.updated_at DESC, progress.id DESC
	LIMIT 1
	$$;
REVOKE ALL ON FUNCTION config.staff_onboarding_across_schools(BIGINT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION config.staff_onboarding_across_schools(BIGINT) TO phoenix_tenant;

CREATE FUNCTION config.school_setup_dismissed_in_traeger(p_account_id BIGINT)
	RETURNS BOOLEAN
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
	SELECT EXISTS (
		SELECT FROM platform.schools AS session_school
		JOIN platform.organization_accounts AS membership
			ON membership.organization_id = session_school.organization_id
			AND membership.account_id = p_account_id
			AND membership.status = 'active'
		JOIN platform.schools AS sibling
			ON sibling.organization_id = session_school.organization_id
			AND sibling.active
			AND sibling.deleted_at IS NULL
		JOIN config.school_setup_dismissals AS dismissal
			ON dismissal.tenant_id = sibling.id
			AND dismissal.account_id = p_account_id
		WHERE session_school.id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
			AND session_school.active
			AND session_school.deleted_at IS NULL
	)
	$$;
REVOKE ALL ON FUNCTION config.school_setup_dismissed_in_traeger(BIGINT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION config.school_setup_dismissed_in_traeger(BIGINT) TO phoenix_tenant;

CREATE FUNCTION config.restore_school_setup_dismissals_in_traeger(p_account_id BIGINT)
	RETURNS VOID
	LANGUAGE sql
	VOLATILE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
	DELETE FROM config.school_setup_dismissals AS dismissal
	USING platform.schools AS session_school
	JOIN platform.organization_accounts AS membership
		ON membership.organization_id = session_school.organization_id
		AND membership.account_id = p_account_id
		AND membership.status = 'active'
	JOIN platform.schools AS sibling
		ON sibling.organization_id = session_school.organization_id
	WHERE session_school.id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
		AND session_school.active
		AND session_school.deleted_at IS NULL
		AND dismissal.tenant_id = sibling.id
		AND dismissal.account_id = p_account_id
	$$;
REVOKE ALL ON FUNCTION config.restore_school_setup_dismissals_in_traeger(BIGINT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION config.restore_school_setup_dismissals_in_traeger(BIGINT) TO phoenix_tenant;
`

// The merge of the staff rows has no way back and needs none: the rows keep
// their shape and per-school reads stay valid.
const onboardingAcrossSchoolsDownSQL = `
DROP FUNCTION IF EXISTS config.restore_school_setup_dismissals_in_traeger(BIGINT);
DROP FUNCTION IF EXISTS config.school_setup_dismissed_in_traeger(BIGINT);
DROP FUNCTION IF EXISTS config.staff_onboarding_across_schools(BIGINT);
`

func onboardingAcrossSchoolsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, onboardingAcrossSchoolsUpSQL); err != nil {
		return fmt.Errorf("onboarding across schools up: %w", err)
	}
	return nil
}

func onboardingAcrossSchoolsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, onboardingAcrossSchoolsDownSQL); err != nil {
		return fmt.Errorf("onboarding across schools down: %w", err)
	}
	return nil
}
