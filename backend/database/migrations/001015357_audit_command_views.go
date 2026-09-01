package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	auditCommandViewsVersion     = "1.15.357"
	auditCommandViewsDescription = "Create Audit-owned append and time-tracking views (#2655)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version: auditCommandViewsVersion, Description: auditCommandViewsDescription,
		DependsOn: []string{customLeaveAllowancesVersion},
	})
	Migrations.MustRegister(auditCommandViewsUp, auditCommandViewsDown)
}

func auditCommandViewsUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		CREATE VIEW audit.file_event_ledger WITH (security_invoker = true) AS
		SELECT id, tenant_id, folder_id, file_id, action, actor_account_id,
			actor_name, detail, created_at, updated_at
		FROM audit.file_events;

		CREATE VIEW audit.guardian_financial_change_ledger WITH (security_invoker = true) AS
		SELECT id, tenant_id, guardian_profile_id, student_id, changed_by,
			field_name, old_value, new_value, note, occurred_at
		FROM audit.guardian_financial_changes;

		GRANT SELECT, INSERT ON audit.file_event_ledger TO phoenix_tenant, phoenix_admin;
		GRANT SELECT, INSERT ON audit.guardian_financial_change_ledger TO phoenix_tenant, phoenix_admin;

		CREATE VIEW audit.time_tracking_audit_log WITH (security_invoker = true) AS
		SELECT e.tenant_id, e.created_at AS occurred_at, 'session_edit'::text AS source,
			MIN(e.id) AS entry_id, e.staff_id, ARRAY[e.staff_id] AS staff_ids,
			NULLIF(e.edited_by, 0) AS actor_staff_id, e.edited_by = 0 AS actor_is_system,
			COALESCE(MAX(NULLIF(e.notes, '')), '')::text AS reason,
			jsonb_build_object(
				'session_id', e.session_id, 'session_date', to_char(ws.date, 'YYYY-MM-DD'),
				'fields', jsonb_agg(jsonb_build_object(
					'field', e.field_name, 'old', e.old_value, 'new', e.new_value
				) ORDER BY e.id)
			) AS detail
		FROM audit.work_session_edits e
		JOIN active.work_sessions ws ON ws.id = e.session_id AND ws.tenant_id = e.tenant_id
		GROUP BY e.tenant_id, e.session_id, e.staff_id, e.edited_by, e.created_at, ws.date
		UNION ALL
		SELECT saa.tenant_id, saa.changed_at, 'absence'::text, saa.id, sa.staff_id,
			ARRAY[sa.staff_id], st.id, FALSE, COALESCE(saa.note, '')::text,
			jsonb_build_object(
				'absence_id', sa.id, 'absence_type', sa.absence_type,
				'absence_type_label', sat.name,
				'date_start', to_char(sa.date_start, 'YYYY-MM-DD'),
				'date_end', to_char(sa.date_end, 'YYYY-MM-DD'),
				'from_status', saa.from_status, 'to_status', saa.to_status
			)
		FROM active.staff_absence_audit saa
		JOIN active.staff_absences sa ON sa.id = saa.absence_id AND sa.tenant_id = saa.tenant_id
		LEFT JOIN active.staff_absence_types sat
			ON sat.tenant_id = sa.tenant_id AND sat.id = sa.absence_type_id
		LEFT JOIN users.persons p ON p.account_id = saa.actor_id
		LEFT JOIN users.staff st ON st.person_id = p.id
		UNION ALL
		SELECT ba.tenant_id, ba.decided_at, 'adjustment'::text, ba.id, ba.staff_id,
			ARRAY[ba.staff_id], ba.decided_by, FALSE, ba.note::text,
			jsonb_build_object(
				'adjustment_id', ba.id, 'type', ba.type,
				'minutes_delta', ba.minutes_delta,
				'effective_date', to_char(ba.effective_date, 'YYYY-MM-DD')
			)
		FROM active.staff_balance_adjustments ba
		UNION ALL
		SELECT s.tenant_id, s.closed_at, 'month_close'::text, MIN(s.id), NULL::bigint,
			array_agg(s.staff_id), s.closed_by, FALSE, s.close_reason::text,
			jsonb_build_object('year', s.year, 'month', s.month, 'account_count', COUNT(*))
		FROM active.staff_month_balance_snapshots s
		GROUP BY s.tenant_id, s.closed_at, s.closed_by, s.year, s.month, s.close_reason
		UNION ALL
		SELECT s.tenant_id, s.reopened_at, 'month_reopen'::text, s.id, s.staff_id,
			ARRAY[s.staff_id], s.reopened_by, FALSE, s.reopen_reason::text,
			jsonb_build_object(
				'year', s.year, 'month', s.month,
				'closing_balance_minutes', s.closing_balance_minutes
			)
		FROM active.staff_month_balance_snapshots s
		WHERE s.reopened_at IS NOT NULL
		UNION ALL
		SELECT d.tenant_id, d.occurred_at, 'deletion'::text, d.id, d.staff_id,
			ARRAY[d.staff_id], d.deleted_by, FALSE, d.note::text,
			jsonb_build_object(
				'deleted_source', d.source, 'source_id', d.source_id, 'payload', d.payload
			)
		FROM audit.time_tracking_deletions d
		UNION ALL
		SELECT vo.tenant_id, vo.decided_at, 'vacation_opening'::text, vo.id, vo.staff_id,
			ARRAY[vo.staff_id], vo.decided_by, FALSE, vo.note::text,
			jsonb_build_object(
				'opening_id', vo.id, 'year', vo.year,
				'effective_date', to_char(vo.effective_date, 'YYYY-MM-DD'),
				'taken_before_days', vo.taken_before_days,
				'entered_remaining_days', vo.entered_remaining_days
			)
		FROM active.staff_vacation_openings vo
		UNION ALL
		SELECT c.tenant_id, c.created_at, 'absence_type_allowance'::text, c.id, c.staff_id,
			ARRAY[c.staff_id], c.changed_by, FALSE, c.reason::text,
			jsonb_build_object(
				'absence_type_id', c.absence_type_id, 'absence_type_label', sat.name,
				'year', c.year, 'old_entitled_days', c.old_entitled_days,
				'new_entitled_days', c.new_entitled_days
			)
		FROM active.staff_absence_type_allowance_changes c
		JOIN active.staff_absence_types sat
			ON sat.tenant_id = c.tenant_id AND sat.id = c.absence_type_id
		UNION ALL
		SELECT c.tenant_id, c.occurred_at, 'personnel_number'::text, c.id, c.staff_id,
			ARRAY[c.staff_id], c.changed_by, FALSE, c.note::text,
			jsonb_build_object('old_value', c.old_value, 'new_value', c.new_value)
		FROM audit.personnel_number_changes c;

		GRANT SELECT ON audit.time_tracking_audit_log TO phoenix_tenant;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("create Audit command views: %w", err)
	}
	return nil
}

func auditCommandViewsDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		DROP VIEW IF EXISTS audit.time_tracking_audit_log;
		DROP VIEW IF EXISTS audit.guardian_financial_change_ledger;
		DROP VIEW IF EXISTS audit.file_event_ledger;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("drop Audit command views: %w", err)
	}
	return nil
}
