-- Read-only audit for template splits created before migration 1.15.183.
--
-- The old schema did not store a predecessor ID, so matching schedule
-- boundaries alone is unsafe: unrelated templates can end/start on the same
-- school date. This query reports only the stronger fingerprint left by a
-- split transaction:
--
--   * the same approved request child/student occurs in both templates;
--   * the candidate schedule and sourced enrollment start at the predecessor
--     schedule's exclusive end boundary; and
--   * the candidate group, schedule, and sourced enrollment share the exact
--     transaction timestamp.
--
-- Results are manual-review candidates, not proof. In particular, one child
-- can belong to multiple templates ending on the same date. Do not turn this
-- into an UPDATE without confirming the intended predecessor for every row.
-- Childless and unsourced historical splits have no reliable fingerprint and
-- require domain/operator review.

WITH RECURSIVE selected_links AS (
    SELECT DISTINCT
        rco.tenant_id,
        rco.request_child_id,
        rc.created_student_id AS student_id,
        co.id AS care_offering_id,
        co.activity_group_id AS linked_group_id
    FROM enrollment.request_child_offerings AS rco
    INNER JOIN enrollment.request_children AS rc
        ON rc.tenant_id = rco.tenant_id
        AND rc.id = rco.request_child_id
    INNER JOIN enrollment.care_offerings AS co
        ON co.tenant_id = rco.tenant_id
        AND co.id = rco.care_offering_id
    INNER JOIN activities.groups AS linked_group
        ON linked_group.tenant_id = rco.tenant_id
        AND linked_group.id = co.activity_group_id
        AND linked_group.is_template = TRUE
    WHERE rc.created_student_id IS NOT NULL
      AND co.activity_group_id IS NOT NULL
), roots AS (
    SELECT DISTINCT
        selected.tenant_id,
        selected.request_child_id,
        selected.student_id,
        selected.care_offering_id,
        selected.linked_group_id,
        selected.linked_group_id AS current_group_id,
        0 AS depth,
        NULL::date AS split_boundary,
        ARRAY[selected.linked_group_id]::bigint[] AS path
    FROM selected_links AS selected
    INNER JOIN activities.student_enrollments AS enrollment
        ON enrollment.tenant_id = selected.tenant_id
        AND enrollment.enrollment_request_child_id = selected.request_child_id
        AND enrollment.student_id = selected.student_id
        AND enrollment.activity_group_id = selected.linked_group_id
), legacy_chain AS (
    SELECT *
    FROM roots

    UNION

    SELECT
        chain.tenant_id,
        chain.request_child_id,
        chain.student_id,
        chain.care_offering_id,
        chain.linked_group_id,
        candidate_group.id,
        chain.depth + 1,
        predecessor_schedule.valid_until,
        chain.path || candidate_group.id
    FROM legacy_chain AS chain
    INNER JOIN activities.schedules AS predecessor_schedule
        ON predecessor_schedule.tenant_id = chain.tenant_id
        AND predecessor_schedule.activity_group_id = chain.current_group_id
        AND predecessor_schedule.valid_until IS NOT NULL
    INNER JOIN activities.schedules AS candidate_schedule
        ON candidate_schedule.tenant_id = chain.tenant_id
        AND candidate_schedule.valid_from = predecessor_schedule.valid_until
    INNER JOIN activities.groups AS candidate_group
        ON candidate_group.tenant_id = chain.tenant_id
        AND candidate_group.id = candidate_schedule.activity_group_id
        AND candidate_group.is_template = TRUE
    INNER JOIN activities.student_enrollments AS candidate_enrollment
        ON candidate_enrollment.tenant_id = chain.tenant_id
        AND candidate_enrollment.enrollment_request_child_id = chain.request_child_id
        AND candidate_enrollment.student_id = chain.student_id
        AND candidate_enrollment.activity_group_id = candidate_group.id
        AND candidate_enrollment.valid_from = predecessor_schedule.valid_until
    WHERE candidate_group.id <> ALL(chain.path)
      AND candidate_group.created_at = candidate_schedule.created_at
      AND candidate_group.created_at = candidate_enrollment.created_at
      AND NOT EXISTS (
          SELECT 1
          FROM enrollment.request_child_offerings AS selected_rco
          INNER JOIN enrollment.care_offerings AS selected_offering
              ON selected_offering.tenant_id = selected_rco.tenant_id
              AND selected_offering.id = selected_rco.care_offering_id
          WHERE selected_rco.tenant_id = chain.tenant_id
            AND selected_rco.request_child_id = chain.request_child_id
            AND selected_offering.activity_group_id = candidate_group.id
      )
)
SELECT DISTINCT
    chain.tenant_id,
    chain.request_child_id,
    chain.student_id,
    chain.care_offering_id,
    chain.linked_group_id,
    chain.current_group_id AS candidate_group_id,
    candidate_group.name AS candidate_group_name,
    chain.depth,
    chain.split_boundary
FROM legacy_chain AS chain
INNER JOIN activities.groups AS candidate_group
    ON candidate_group.tenant_id = chain.tenant_id
    AND candidate_group.id = chain.current_group_id
WHERE chain.depth > 0
  AND candidate_group.series_root_id IS NULL
ORDER BY
    chain.tenant_id,
    chain.request_child_id,
    chain.care_offering_id,
    chain.depth;
