-- Read-only audit: children in active care at booking-led schools with no
-- care-counting booking today and no open completion task (#6505, class c).
--
-- Before PR #6549 deleting a phase removed its requests and bookings
-- (ON DELETE CASCADE) but kept the child in care and created no completion
-- task (users.care_withdrawal_completions). Since then a phase with bookings
-- that still apply cannot be deleted (ADR 0202). The booking evaluation
-- mirrored here is the one of Care Plan (EvaluateCareBookingStates):
--   - booking-led: config.setting_values enrollment.bookings_authoritative is
--     true (school level only, registry default false);
--   - care source: a booking of an approved request child of the student
--     (created_student_id, else matched_student_id) in an offering with
--     counts_as_care, with at least one booked weekday (fixed offerings: the
--     offering's days, else manual and automatic selection) that falls into
--     its half-open window [valid_from, valid_until);
--   - care today: such a booking with valid_from <= today < valid_until.
-- Active care is status active within enrolled_from..enrolled_until; a child
-- activated ahead of its start (enrolled_from after today) is not selected.
--
-- finding:
--   no_booking_at_all: the student has no care source at all. This is the
--     state a pre-#6549 phase deletion leaves; a child the school added
--     without enrollment looks the same (the deleted rows leave no trace).
--   booking_ended: care sources exist but all ended on or before today. The
--     booking evaluation would plan a completion task; the reconciliation did
--     not run for this child.
--   booking_starts_later: the next care source starts after today; the child
--     has a gap until then.
--
-- Contains identifiers and dates only, no names. Run with a connection that
-- bypasses RLS, read-only. The tables force row-level security, so any other
-- role silently sees no rows: first check that
--   SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user;
-- returns true.
--
--   PGOPTIONS='-c default_transaction_read_only=on' psql "$DB_DSN" \
--     -f backend/database/audits/booking_led_care_without_source_or_task.sql

WITH today AS (
    SELECT (now() AT TIME ZONE 'Europe/Berlin')::date AS day
),
booking_led AS (
    SELECT value.tenant_id
    FROM config.setting_values AS value
    WHERE value.setting_key = 'enrollment.bookings_authoritative'
        AND value.value = 'true'::jsonb
),
care_sources AS (
    SELECT
        booking.tenant_id,
        COALESCE(rc.created_student_id, rc.matched_student_id) AS student_id,
        booking.valid_from,
        booking.valid_until
    FROM enrollment.care_offering_bookings AS booking
    INNER JOIN enrollment.request_children AS rc
        ON rc.tenant_id = booking.tenant_id
        AND rc.id = booking.request_child_id
    INNER JOIN enrollment.care_offerings AS offering
        ON offering.tenant_id = booking.tenant_id
        AND offering.id = booking.care_offering_id
    INNER JOIN booking_led
        ON booking_led.tenant_id = booking.tenant_id
    WHERE rc.status = 'approved'
        AND offering.counts_as_care
        AND COALESCE(rc.created_student_id, rc.matched_student_id) IS NOT NULL
        AND EXISTS (
            SELECT 1
            FROM jsonb_array_elements_text(
                CASE
                    WHEN offering.days_of_week_mode = 'fixed' THEN offering.available_days
                    ELSE COALESCE(booking.manual_selected_days, '[]'::jsonb)
                        || COALESCE(booking.automatic_selected_days, '[]'::jsonb)
                END
            ) AS day(name)
            CROSS JOIN LATERAL (
                SELECT array_position(
                    ARRAY['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'],
                    lower(btrim(day.name))
                ) AS iso
            ) AS weekday
            WHERE weekday.iso IS NOT NULL
                AND (
                    booking.valid_from IS NULL
                    OR booking.valid_until IS NULL
                    OR booking.valid_from
                        + ((weekday.iso - EXTRACT(ISODOW FROM booking.valid_from)::int + 7) % 7)
                        < booking.valid_until
                )
        )
),
in_care AS (
    SELECT
        membership.tenant_id,
        student.id AS student_id,
        membership.enrolled_from,
        membership.enrolled_until
    FROM users.student_profiles AS student
    INNER JOIN users.student_school_memberships AS membership
        ON membership.tenant_id = student.tenant_id
        AND membership.student_profile_id = student.id
        AND membership.deleted_at IS NULL
    INNER JOIN users.student_care_profiles AS care
        ON care.tenant_id = membership.tenant_id
        AND care.membership_id = membership.id
    INNER JOIN booking_led
        ON booking_led.tenant_id = membership.tenant_id
    CROSS JOIN today
    WHERE membership.status = 'active'
        AND (membership.enrolled_from IS NULL OR membership.enrolled_from <= today.day)
        AND (membership.enrolled_until IS NULL OR membership.enrolled_until >= today.day)
)
SELECT
    in_care.tenant_id AS school_id,
    in_care.student_id,
    in_care.enrolled_from,
    in_care.enrolled_until,
    MAX(source.valid_until) FILTER (WHERE source.valid_until <= today.day) AS last_booking_end,
    MIN(source.valid_from) FILTER (WHERE source.valid_from > today.day) AS next_booking_start,
    CASE
        WHEN COUNT(source.student_id) = 0 THEN 'no_booking_at_all'
        WHEN COUNT(*) FILTER (WHERE source.valid_from > today.day) > 0 THEN 'booking_starts_later'
        ELSE 'booking_ended'
    END AS finding
FROM in_care
CROSS JOIN today
LEFT JOIN care_sources AS source
    ON source.tenant_id = in_care.tenant_id
    AND source.student_id = in_care.student_id
WHERE NOT EXISTS (
        SELECT 1
        FROM care_sources AS covering
        WHERE covering.tenant_id = in_care.tenant_id
            AND covering.student_id = in_care.student_id
            AND (covering.valid_from IS NULL OR covering.valid_from <= today.day)
            AND (covering.valid_until IS NULL OR covering.valid_until > today.day)
    )
    AND NOT EXISTS (
        SELECT 1
        FROM users.care_withdrawal_completions AS task
        WHERE task.tenant_id = in_care.tenant_id
            AND task.student_id = in_care.student_id
            AND task.state = 'pending'
    )
GROUP BY in_care.tenant_id, in_care.student_id, in_care.enrolled_from, in_care.enrolled_until, today.day
ORDER BY in_care.tenant_id, in_care.student_id;
