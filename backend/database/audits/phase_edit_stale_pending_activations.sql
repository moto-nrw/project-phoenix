-- Read-only audit: pending children whose planned activation no longer
-- matches the start of their phase (#6505, class a).
--
-- Before PR #6549 a phase edit moved only weekly_plan_on. The approval had
-- copied the phase start into request_children.activate_on and into the
-- child's enrolled_from; both stayed on the old day, and the hourly
-- activation tick activates on enrolled_from. Since PR #6549 (ADR 0202) a
-- phase edit moves a still pending child's plan and window together, so a row
-- here predates that fix.
--
-- Selected: an approved, scheduled request child of the phase whose student
-- is still pending, whose activate_on differs from the phase start and still
-- lies ahead (an activation whose day has come counts as executed, ADR 0202).
--
-- finding:
--   unambiguous: the child's only approved request child is this one and
--     enrolled_from still equals activate_on, i.e. both are the untouched
--     copy of the old phase start. Correction target: the phase start.
--   ambiguous_several_approvals: the student has approved request children
--     in more than one phase; the window may come from another approval.
--   ambiguous_window_changed: enrolled_from no longer equals activate_on
--     (manual change or another approval); the school decides.
--
-- Contains identifiers and dates only, no names. Run with a connection that
-- bypasses RLS, read-only. The tables force row-level security, so any other
-- role silently sees no rows: first check that
--   SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user;
-- returns true.
--
--   PGOPTIONS='-c default_transaction_read_only=on' psql "$DB_DSN" \
--     -f backend/database/audits/phase_edit_stale_pending_activations.sql

WITH today AS (
    SELECT (now() AT TIME ZONE 'Europe/Berlin')::date AS day
),
approvals AS (
    SELECT
        rc.tenant_id,
        COALESCE(rc.created_student_id, rc.matched_student_id) AS student_id,
        COUNT(DISTINCT request.phase_id) AS approved_phases
    FROM enrollment.request_children AS rc
    INNER JOIN enrollment.requests AS request
        ON request.tenant_id = rc.tenant_id
        AND request.id = rc.request_id
    WHERE rc.status = 'approved'
        AND COALESCE(rc.created_student_id, rc.matched_student_id) IS NOT NULL
    GROUP BY 1, 2
)
SELECT
    rc.tenant_id AS school_id,
    rc.created_student_id AS student_id,
    rc.id AS request_child_id,
    phase.id AS phase_id,
    phase.service_start_date AS phase_start,
    phase.service_end_date AS phase_end,
    rc.activate_on,
    membership.enrolled_from,
    membership.enrolled_until,
    approvals.approved_phases,
    CASE
        WHEN approvals.approved_phases > 1 THEN 'ambiguous_several_approvals'
        WHEN membership.enrolled_from IS DISTINCT FROM rc.activate_on THEN 'ambiguous_window_changed'
        ELSE 'unambiguous'
    END AS finding
FROM enrollment.request_children AS rc
INNER JOIN enrollment.requests AS request
    ON request.tenant_id = rc.tenant_id
    AND request.id = rc.request_id
INNER JOIN enrollment.phases AS phase
    ON phase.tenant_id = request.tenant_id
    AND phase.id = request.phase_id
INNER JOIN users.student_school_memberships AS membership
    ON membership.tenant_id = rc.tenant_id
    AND membership.student_profile_id = rc.created_student_id
    AND membership.deleted_at IS NULL
INNER JOIN approvals
    ON approvals.tenant_id = rc.tenant_id
    AND approvals.student_id = rc.created_student_id
CROSS JOIN today
WHERE rc.status = 'approved'
    AND rc.activation_mode = 'scheduled'
    AND rc.activate_on IS NOT NULL
    AND rc.activate_on <> phase.service_start_date
    AND rc.activate_on > today.day
    AND membership.status = 'pending'
ORDER BY rc.tenant_id, rc.created_student_id, rc.id;
