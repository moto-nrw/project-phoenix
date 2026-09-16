package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	vacationQuotaChangesVersion     = "1.15.389"
	vacationQuotaChangesDescription = "Record reasoned changes of the vacation entitlement in the time-tracking audit log (#3256)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     vacationQuotaChangesVersion,
		Description: vacationQuotaChangesDescription,
		DependsOn:   []string{auditCommandViewsVersion, absenceAllowanceAlwaysBlockVersion},
	})
	Migrations.MustRegister(vacationQuotaChangesUp, vacationQuotaChangesDown)
}

// vacationQuotaChangesUp adds the reason trail for the Urlaubsanspruch, the
// counterpart of staff_absence_type_allowance_changes, and shows it in the
// time-tracking audit log. The view body is 1.15.358 plus one branch.
func vacationQuotaChangesUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS active.staff_vacation_quota_changes (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			staff_id BIGINT NOT NULL,
			year INT NOT NULL,
			old_entitled_days NUMERIC(5,1),
			new_entitled_days NUMERIC(5,1) NOT NULL,
			old_carryover_days NUMERIC(5,1),
			new_carryover_days NUMERIC(5,1) NOT NULL,
			reason TEXT NOT NULL,
			changed_by BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_svq_change_staff FOREIGN KEY (tenant_id, staff_id)
				REFERENCES users.staff(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT fk_svq_change_actor FOREIGN KEY (tenant_id, changed_by)
				REFERENCES users.staff(tenant_id, id) ON DELETE RESTRICT,
			CONSTRAINT chk_svq_change_year CHECK (year BETWEEN 2000 AND 2100),
			CONSTRAINT chk_svq_change_reason CHECK (length(btrim(reason)) > 0)
		);
		CREATE INDEX IF NOT EXISTS idx_svq_changes_staff
			ON active.staff_vacation_quota_changes (tenant_id, staff_id, created_at DESC);
		GRANT SELECT, INSERT ON active.staff_vacation_quota_changes TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE active.staff_vacation_quota_changes_id_seq TO phoenix_tenant;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("create vacation quota changes: %w", err)
	}
	if err := provisionTenantRLS(ctx, db, "active.staff_vacation_quota_changes"); err != nil {
		return err
	}
	if _, err := db.NewRaw(vacationQuotaAuditViewSQL).Exec(ctx); err != nil {
		return fmt.Errorf("extend time-tracking audit log with vacation quota changes: %w", err)
	}
	return nil
}

func vacationQuotaChangesDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		DROP VIEW IF EXISTS audit.time_tracking_audit_log;
		` + previousTimeTrackingAuditViewSQL + `
		GRANT SELECT ON audit.time_tracking_audit_log TO phoenix_tenant;
		DROP TABLE IF EXISTS active.staff_vacation_quota_changes;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("drop vacation quota changes: %w", err)
	}
	return nil
}

const vacationQuotaAuditViewSQL = `
		CREATE OR REPLACE VIEW audit.time_tracking_audit_log WITH (security_invoker = true) AS
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
		FROM audit.personnel_number_changes c
		UNION ALL
		SELECT q.tenant_id, q.created_at, 'vacation_quota'::text, q.id, q.staff_id,
			ARRAY[q.staff_id], q.changed_by, FALSE, q.reason::text,
			jsonb_build_object(
				'year', q.year,
				'old_entitled_days', q.old_entitled_days, 'new_entitled_days', q.new_entitled_days,
				'old_carryover_days', q.old_carryover_days, 'new_carryover_days', q.new_carryover_days
			)
		FROM active.staff_vacation_quota_changes q;

		GRANT SELECT ON audit.time_tracking_audit_log TO phoenix_tenant;
`

// previousTimeTrackingAuditViewSQL is the 1.15.358 view body.
const previousTimeTrackingAuditViewSQL = `
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
`
