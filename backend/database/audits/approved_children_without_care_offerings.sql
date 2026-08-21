-- Read-only audit for approved enrollment children without a care-offering
-- booking that overlaps their phase's service period (#2423).
--
-- A row in an optional phase is not automatically corrupt: OGS care exists
-- independently of additional offerings. Those rows are labelled
-- review_optional so each school can confirm the approval was intentional.
-- Rows in at_least_one/exactly_one phases violate the phase rule and are
-- labelled missing_required.
--
-- The result deliberately contains identifiers, not child names. Run it with
-- the migration/admin connection and hand the tenant-specific rows only to
-- the school responsible for them:
--
--   psql "$DB_DSN" -f backend/database/audits/approved_children_without_care_offerings.sql

SELECT
    rc.tenant_id,
    school.name AS school_name,
    phase.id AS phase_id,
    phase.name AS phase_name,
    phase.care_offering_selection_mode,
    request.id AS request_id,
    rc.id AS request_child_id,
    rc.created_student_id,
    rc.reviewed_at,
    CASE
        WHEN phase.care_offering_selection_mode = 'optional'
            THEN 'review_optional'
        ELSE 'missing_required'
    END AS finding
FROM enrollment.request_children AS rc
INNER JOIN enrollment.requests AS request
    ON request.tenant_id = rc.tenant_id
    AND request.id = rc.request_id
INNER JOIN enrollment.phases AS phase
    ON phase.tenant_id = request.tenant_id
    AND phase.id = request.phase_id
INNER JOIN platform.schools AS school
    ON school.id = rc.tenant_id
WHERE rc.status = 'approved'
  AND NOT EXISTS (
      SELECT 1
      FROM enrollment.request_child_offerings AS link
      INNER JOIN enrollment.care_offerings AS offering
          ON offering.tenant_id = link.tenant_id
          AND offering.id = link.care_offering_id
          AND offering.phase_id = phase.id
      WHERE link.tenant_id = rc.tenant_id
        AND link.request_child_id = rc.id
        AND (link.valid_from IS NULL OR link.valid_from <= phase.service_end_date)
        AND (link.valid_until IS NULL OR link.valid_until > phase.service_start_date)
  )
ORDER BY rc.tenant_id, phase.service_start_date, phase.id, rc.id;
