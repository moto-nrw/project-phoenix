package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayGuestAnswerFilterVersion = "1.15.713"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayGuestAnswerFilterVersion,
		Description: "Gesundheitseinstufung beim Lesen kopierter Ferienantworten prüfen (#5202)",
		DependsOn:   []string{holidayHostSharedAnswersVersion},
	})
	Migrations.MustRegister(holidayGuestAnswerFilterUp, holidayGuestAnswerFilterDown)
}

// Filter the persisted copy, not the live participant source, which closes
// after the week. A later health classification must also hide copied
// answers during the seven read-only days, without an hourly reconciliation.
// The caller can name only a guest of its school, never arbitrary answers
// or a Träger's form. Missing or unclear health markers fail closed, exactly
// as in holiday_host_participants. Ordinary copied values and order stay intact.
const holidayGuestAnswerFilterUpSQL = `
CREATE FUNCTION enrollment.holiday_guest_shared_answers(p_guest_id BIGINT)
	RETURNS JSONB
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
	WITH guest AS (
		SELECT copy.answers, copy.organization_id, week.holiday_period_id
		FROM enrollment.holiday_guests AS copy
		JOIN enrollment.holiday_period_weeks AS week
			ON week.organization_id = copy.organization_id AND week.id = copy.holiday_period_week_id
		WHERE copy.id = p_guest_id
			AND copy.tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
			AND NULLIF(current_setting('app.current_org_id', true), '') IS NULL
			AND copy.ended_on IS NULL
			AND copy.ends_on >= (CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date - 7
	), period_field AS (
		SELECT field.definition
		FROM guest
		JOIN enrollment.holiday_period_forms AS form
			ON form.organization_id = guest.organization_id AND form.holiday_period_id = guest.holiday_period_id
		CROSS JOIN LATERAL jsonb_array_elements(form.fields) AS field(definition)
	)
	SELECT COALESCE(jsonb_agg(answer.definition ORDER BY answer.ordinality), '[]'::jsonb)
	FROM guest
	CROSS JOIN LATERAL jsonb_array_elements(guest.answers) WITH ORDINALITY AS answer(definition, ordinality)
	WHERE EXISTS (
		SELECT 1 FROM period_field AS field
		WHERE field.definition->>'key' = answer.definition->>'key'
			AND field.definition->'health' = 'false'::jsonb)
		AND NOT EXISTS (
			SELECT 1 FROM period_field AS field
			WHERE field.definition->>'key' = answer.definition->>'key'
				AND field.definition->'health' IS DISTINCT FROM 'false'::jsonb)
	$$;
REVOKE ALL ON FUNCTION enrollment.holiday_guest_shared_answers(BIGINT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION enrollment.holiday_guest_shared_answers(BIGINT) TO phoenix_tenant;
`

func holidayGuestAnswerFilterUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayGuestAnswerFilterUpSQL); err != nil {
		return fmt.Errorf("add holiday guest answer filter: %w", err)
	}
	return nil
}

func holidayGuestAnswerFilterDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `DROP FUNCTION enrollment.holiday_guest_shared_answers(BIGINT)`); err != nil {
		return fmt.Errorf("drop holiday guest answer filter: %w", err)
	}
	return nil
}
