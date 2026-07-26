-- Read-only audit for care offerings whose NAME names a single weekday but
-- whose available_days allow more days than that (#1885).
--
-- Background: until #1885 the admin editor silently pre-selected Mo-Fr for
-- every new offering. In production this produced offerings like
-- "Montags: Betreuung bis zum Beginn des privaten Musikunterrichts"
-- (tenant 5, offering 8) that were technically configured Mo-Fr, so the
-- public form correctly offered - and the backend correctly accepted -
-- Tuesday..Friday picks. Two child rows with non-Monday days were stored
-- that way (request_child_offering_id 14 and 18, see
-- https://github.com/moto-nrw/project-phoenix/issues/1834#issuecomment-4893612999).
--
-- Detection heuristic:
--   * the offering name contains exactly ONE German weekday word
--     (Montag/Montags, Dienstag(s), ..., also Sonnabend), case-insensitive,
--     on a word boundary; and
--   * available_days contains at least one day other than the named one.
--
-- Results are manual-review candidates, not proof. A name like
-- "Fussball (startet am Montag, 1.9.)" legitimately names a weekday without
-- restricting the offering to it, and offerings whose restriction lives only
-- in the DESCRIPTION are not caught at all. Do not turn this into an UPDATE:
-- whether an offering's available_days (and any stored child selections)
-- should be narrowed is a per-school decision made with that school.
--
-- Run (read-only) via psql against the target database, e.g.:
--   psql "$DB_DSN" -f backend/database/audits/care_offering_weekday_name_vs_available_days.sql

-- ---------------------------------------------------------------------------
-- Part 1: offerings whose name names exactly one weekday but whose
-- available_days include other days.
-- ---------------------------------------------------------------------------
WITH weekday_words(day_key, pattern) AS (
    VALUES
        ('mon', '\mmontags?\M'),
        ('tue', '\mdienstags?\M'),
        ('wed', '\mmittwochs?\M'),
        ('thu', '\mdonnerstags?\M'),
        ('fri', '\mfreitags?\M'),
        ('sat', '\m(samstags?|sonnabends?)\M'),
        ('sun', '\msonntags?\M')
), named_offerings AS (
    SELECT
        co.tenant_id,
        co.id AS care_offering_id,
        co.phase_id,
        co.name,
        co.days_of_week_mode,
        co.available_days,
        co.created_at,
        ww.day_key AS named_day
    FROM enrollment.care_offerings AS co
    INNER JOIN weekday_words AS ww
        ON co.name ~* ww.pattern
), single_day_offerings AS (
    -- Names mentioning two or more different weekdays ("Montag und Mittwoch")
    -- carry no single-day expectation; skip them.
    SELECT *
    FROM named_offerings
    WHERE (
        SELECT count(*)
        FROM named_offerings AS other
        WHERE other.tenant_id = named_offerings.tenant_id
          AND other.care_offering_id = named_offerings.care_offering_id
    ) = 1
), suspicious_offerings AS (
    SELECT so.*
    FROM single_day_offerings AS so
    WHERE EXISTS (
        SELECT 1
        FROM jsonb_array_elements_text(so.available_days) AS day(value)
        WHERE day.value <> so.named_day
    )
)
SELECT
    so.tenant_id,
    p.name AS phase_name,
    so.care_offering_id,
    so.name,
    so.named_day,
    so.days_of_week_mode,
    so.available_days,
    so.created_at
FROM suspicious_offerings AS so
LEFT JOIN enrollment.phases AS p
    ON p.tenant_id = so.tenant_id
    AND p.id = so.phase_id
ORDER BY so.tenant_id, so.care_offering_id;

-- ---------------------------------------------------------------------------
-- Part 2: stored child selections on those offerings that picked days
-- outside the weekday named by the offering. These are the rows a school
-- may want to correct manually (covers the two known production rows).
-- ---------------------------------------------------------------------------
WITH weekday_words(day_key, pattern) AS (
    VALUES
        ('mon', '\mmontags?\M'),
        ('tue', '\mdienstags?\M'),
        ('wed', '\mmittwochs?\M'),
        ('thu', '\mdonnerstags?\M'),
        ('fri', '\mfreitags?\M'),
        ('sat', '\m(samstags?|sonnabends?)\M'),
        ('sun', '\msonntags?\M')
), named_offerings AS (
    SELECT
        co.tenant_id,
        co.id AS care_offering_id,
        co.name,
        ww.day_key AS named_day
    FROM enrollment.care_offerings AS co
    INNER JOIN weekday_words AS ww
        ON co.name ~* ww.pattern
), single_day_offerings AS (
    SELECT *
    FROM named_offerings
    WHERE (
        SELECT count(*)
        FROM named_offerings AS other
        WHERE other.tenant_id = named_offerings.tenant_id
          AND other.care_offering_id = named_offerings.care_offering_id
    ) = 1
)
SELECT
    rco.tenant_id,
    so.care_offering_id,
    so.name AS offering_name,
    so.named_day,
    rco.id AS request_child_offering_id,
    rco.request_child_id,
    rc.request_id,
    rc.status,
    r.submission_source,
    rco.selected_days,
    rco.created_at
FROM single_day_offerings AS so
INNER JOIN enrollment.request_child_offerings AS rco
    ON rco.tenant_id = so.tenant_id
    AND rco.care_offering_id = so.care_offering_id
INNER JOIN enrollment.request_children AS rc
    ON rc.tenant_id = rco.tenant_id
    AND rc.id = rco.request_child_id
INNER JOIN enrollment.requests AS r
    ON r.tenant_id = rc.tenant_id
    AND r.id = rc.request_id
WHERE rco.selected_days IS NOT NULL
  AND EXISTS (
      SELECT 1
      FROM jsonb_array_elements_text(rco.selected_days) AS day(value)
      WHERE day.value <> so.named_day
  )
ORDER BY rco.tenant_id, so.care_offering_id, rco.id;
