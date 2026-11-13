-- Read-only audit: open class switches and weekly plans of an approved
-- renewal that no longer follow the start of their phase (#6505, class b).
--
-- The approval of a renewal for a child in running care plans the class
-- switch (class_switch_on) and the weekly plan (weekly_plan_on) for the phase
-- start. Before PR #6549 a phase edit moved only weekly_plan_on; since then it
-- moves both in one statement for every request child of the phase. Both
-- columns are cleared when their tick applies them, so a non-null value is an
-- open plan, and an open plan off the phase start predates the fix.
--
-- finding:
--   unambiguous: the request child is approved, its student is active or
--     pending and still sits in class_switch_from (the tick would apply the
--     switch), and no other open class switch exists for the student.
--     Correction target: the phase start.
--   ambiguous_several_switches: another request child of the same student
--     also has an open class switch.
--   no_effect: the tick would drop the switch anyway (request child no
--     longer approved or without created_student_id, student not
--     active/pending, or class changed since the approval); only the date
--     is stale.
--   weekly_plan_only: no class switch is open, only weekly_plan_on is off the
--     phase start.
--
-- Contains identifiers, dates and class labels only, no names. Run with a
-- connection that bypasses RLS, read-only. The tables force row-level
-- security, so any other role silently sees no rows: first check that
--   SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user;
-- returns true.
--
--   PGOPTIONS='-c default_transaction_read_only=on' psql "$DB_DSN" \
--     -f backend/database/audits/phase_edit_stale_class_switches.sql

WITH open_switches AS (
    SELECT
        rc.tenant_id,
        COALESCE(rc.created_student_id, rc.matched_student_id) AS student_id,
        COUNT(*) AS open_switches
    FROM enrollment.request_children AS rc
    WHERE rc.class_switch_on IS NOT NULL
    GROUP BY 1, 2
)
SELECT
    rc.tenant_id AS school_id,
    COALESCE(rc.created_student_id, rc.matched_student_id) AS student_id,
    rc.id AS request_child_id,
    rc.status AS request_child_status,
    phase.id AS phase_id,
    phase.service_start_date AS phase_start,
    rc.class_switch_on,
    rc.weekly_plan_on,
    rc.class_switch_from,
    rc.class_switch_to,
    membership.status AS student_status,
    membership.school_class AS current_class,
    CASE
        WHEN rc.class_switch_on IS NULL THEN 'weekly_plan_only'
        WHEN rc.status <> 'approved'
            OR membership.status IS NULL
            OR membership.status NOT IN ('active', 'pending')
            OR membership.school_class IS DISTINCT FROM rc.class_switch_from
            THEN 'no_effect'
        WHEN COALESCE(open_switches.open_switches, 0) > 1 THEN 'ambiguous_several_switches'
        ELSE 'unambiguous'
    END AS finding
FROM enrollment.request_children AS rc
INNER JOIN enrollment.requests AS request
    ON request.tenant_id = rc.tenant_id
    AND request.id = rc.request_id
INNER JOIN enrollment.phases AS phase
    ON phase.tenant_id = request.tenant_id
    AND phase.id = request.phase_id
LEFT JOIN users.student_school_memberships AS membership
    ON membership.tenant_id = rc.tenant_id
    AND membership.student_profile_id = rc.created_student_id
    AND membership.deleted_at IS NULL
LEFT JOIN open_switches
    ON open_switches.tenant_id = rc.tenant_id
    AND open_switches.student_id = COALESCE(rc.created_student_id, rc.matched_student_id)
WHERE (rc.class_switch_on IS NOT NULL AND rc.class_switch_on <> phase.service_start_date)
    OR (rc.weekly_plan_on IS NOT NULL AND rc.weekly_plan_on <> phase.service_start_date)
ORDER BY rc.tenant_id, student_id, rc.id;
