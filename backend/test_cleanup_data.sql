-- Test data for cleanup feature testing
-- Run this after migrations to create test data
-- NOTE: This assumes the PostgreSQL schemas already exist from migrations
-- Uses PL/pgSQL DO block to eliminate duplicate string literals (SonarCloud S1192)

DO $$
DECLARE
    -- String constants (eliminates S1192 duplicate literal violations)
    v_policy_version CONSTANT TEXT := '1.0';
    v_device_type CONSTANT TEXT := 'rfid_reader';
    v_device_status CONSTANT device_status := 'active'::device_status;
    v_building_a CONSTANT TEXT := 'Building A';

    -- Timestamp constants for consistent time references
    v_now CONSTANT TIMESTAMPTZ := NOW();
    v_60_days_ago CONSTANT TIMESTAMPTZ := NOW() - INTERVAL '60 days';
    v_40_days_ago CONSTANT TIMESTAMPTZ := NOW() - INTERVAL '40 days';
    v_20_days_ago CONSTANT TIMESTAMPTZ := NOW() - INTERVAL '20 days';
    v_10_days_ago CONSTANT TIMESTAMPTZ := NOW() - INTERVAL '10 days';

    -- Base dates for historical visit records (May 2025)
    v_may_01_10h CONSTANT TIMESTAMPTZ := '2025-05-01 10:00:00+00'::TIMESTAMPTZ;
    v_may_01_11h CONSTANT TIMESTAMPTZ := '2025-05-01 11:00:00+00'::TIMESTAMPTZ;
    v_may_01_12h CONSTANT TIMESTAMPTZ := '2025-05-01 12:00:00+00'::TIMESTAMPTZ;
    v_may_01_13h CONSTANT TIMESTAMPTZ := '2025-05-01 13:00:00+00'::TIMESTAMPTZ;
    v_may_01_14h CONSTANT TIMESTAMPTZ := '2025-05-01 14:00:00+00'::TIMESTAMPTZ;
    v_may_10_15h CONSTANT TIMESTAMPTZ := '2025-05-10 15:00:00+00'::TIMESTAMPTZ;
    v_may_10_16h CONSTANT TIMESTAMPTZ := '2025-05-10 16:00:00+00'::TIMESTAMPTZ;
    v_may_15_14h CONSTANT TIMESTAMPTZ := '2025-05-15 14:00:00+00'::TIMESTAMPTZ;
    v_may_15_15h CONSTANT TIMESTAMPTZ := '2025-05-15 15:00:00+00'::TIMESTAMPTZ;
    v_may_20_10h CONSTANT TIMESTAMPTZ := '2025-05-20 10:00:00+00'::TIMESTAMPTZ;
    v_may_20_13h CONSTANT TIMESTAMPTZ := '2025-05-20 13:00:00+00'::TIMESTAMPTZ;
    v_may_25_09h CONSTANT TIMESTAMPTZ := '2025-05-25 09:00:00+00'::TIMESTAMPTZ;
    v_may_25_12h CONSTANT TIMESTAMPTZ := '2025-05-25 12:00:00+00'::TIMESTAMPTZ;
    v_may_28_13h CONSTANT TIMESTAMPTZ := '2025-05-28 13:00:00+00'::TIMESTAMPTZ;
    v_may_28_15h CONSTANT TIMESTAMPTZ := '2025-05-28 15:00:00+00'::TIMESTAMPTZ;

    -- Base dates for historical visit records (April 2025)
    v_apr_01_12h CONSTANT TIMESTAMPTZ := '2025-04-01 12:00:00+00'::TIMESTAMPTZ;
    v_apr_01_14h CONSTANT TIMESTAMPTZ := '2025-04-01 14:00:00+00'::TIMESTAMPTZ;
    v_apr_15_16h CONSTANT TIMESTAMPTZ := '2025-04-15 16:00:00+00'::TIMESTAMPTZ;
    v_apr_15_17h CONSTANT TIMESTAMPTZ := '2025-04-15 17:00:00+00'::TIMESTAMPTZ;

    -- Relative time offsets for recent visits
    v_6_days_ago CONSTANT TIMESTAMPTZ := NOW() - INTERVAL '6 days';
    v_3_days_ago CONSTANT TIMESTAMPTZ := NOW() - INTERVAL '3 days';
    v_2_days_ago CONSTANT TIMESTAMPTZ := NOW() - INTERVAL '2 days';
    v_1_day_ago CONSTANT TIMESTAMPTZ := NOW() - INTERVAL '1 day';
    v_7_days_ago CONSTANT TIMESTAMPTZ := NOW() - INTERVAL '7 days';
    v_13_days_ago CONSTANT TIMESTAMPTZ := NOW() - INTERVAL '13 days';
    v_15_days_ago CONSTANT TIMESTAMPTZ := NOW() - INTERVAL '15 days';
    v_29_days_ago CONSTANT TIMESTAMPTZ := NOW() - INTERVAL '29 days';
    v_5_days_ago CONSTANT TIMESTAMPTZ := NOW() - INTERVAL '5 days';
BEGIN
    -- Create test education groups first (for students)
    INSERT INTO education.groups (id, name, created_at, updated_at) VALUES
    (1, 'Test Class 1', v_now, v_now),
    (2, 'Test Class 2', v_now, v_now)
    ON CONFLICT (id) DO NOTHING;

    -- Create test activities (for active groups)
    INSERT INTO activities.categories (id, name, created_at, updated_at) VALUES
    (1, 'Test Category', v_now, v_now)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO activities.groups (id, name, max_participants, is_open, category_id, created_at, updated_at) VALUES
    (1, 'Test Activity Group 1', 30, true, 1, v_now, v_now),
    (2, 'Test Activity Group 2', 30, true, 1, v_now, v_now)
    ON CONFLICT (id) DO NOTHING;

    -- Create test students
    INSERT INTO users.persons (id, first_name, last_name, created_at, updated_at) VALUES
    (1001, 'Test', 'Student1', v_now, v_now),
    (1002, 'Test', 'Student2', v_now, v_now),
    (1003, 'Test', 'Student3', v_now, v_now);

    INSERT INTO users.students (id, person_id, school_class, guardian_name, guardian_contact, group_id, created_at, updated_at) VALUES
    (1001, 1001, '5a', 'Test Guardian 1', 'guardian1@test.com', 1, v_now, v_now),
    (1002, 1002, '5b', 'Test Guardian 2', 'guardian2@test.com', 1, v_now, v_now),
    (1003, 1003, '5c', 'Test Guardian 3', 'guardian3@test.com', 2, v_now, v_now);

    -- Create privacy consents with different retention periods
    INSERT INTO users.privacy_consents (student_id, policy_version, accepted, accepted_at, data_retention_days, created_at, updated_at) VALUES
    (1001, v_policy_version, true, v_60_days_ago, 7, v_now, v_now),   -- 7 days retention
    (1002, v_policy_version, true, v_60_days_ago, 14, v_now, v_now),  -- 14 days retention
    (1003, v_policy_version, true, v_60_days_ago, 30, v_now, v_now);  -- 30 days retention

    -- Create test devices first (required for active groups)
    INSERT INTO iot.devices (id, name, device_type, device_id, status, created_at, updated_at) VALUES
    (1001, 'Test Device 1', v_device_type, 'TEST-001', v_device_status, v_now, v_now),
    (1002, 'Test Device 2', v_device_type, 'TEST-002', v_device_status, v_now, v_now),
    (1003, 'Test Device 3', v_device_type, 'TEST-003', v_device_status, v_now, v_now)
    ON CONFLICT (device_id) DO NOTHING;

    -- Create test rooms (if they don't exist)
    INSERT INTO facilities.rooms (id, name, building, capacity, created_at, updated_at) VALUES
    (1, 'Test Room 1', v_building_a, 30, v_now, v_now),
    (2, 'Test Room 2', v_building_a, 30, v_now, v_now),
    (3, 'Test Room 3', 'Building B', 30, v_now, v_now)
    ON CONFLICT (id) DO NOTHING;

    -- Create active groups for visits
    INSERT INTO active.groups (id, device_id, group_id, room_id, start_time, created_at, updated_at) VALUES
    (1001, 1001, 1, 1, v_40_days_ago, v_40_days_ago, v_now),
    (1002, 1002, 1, 2, v_20_days_ago, v_20_days_ago, v_now),
    (1003, 1003, 2, 3, v_10_days_ago, v_10_days_ago, v_now);

    -- Create visits with different ages for each student
    -- Student 1001 (7 day retention)
    INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at) VALUES
    -- Old visits (should be deleted) - explicitly old dates
    (1001, 1001, v_may_01_10h, v_may_01_12h, v_may_01_10h, v_may_01_10h),
    (1001, 1001, v_may_15_14h, v_may_15_15h, v_may_15_14h, v_may_15_14h),
    (1001, 1001, v_may_25_09h, v_may_25_12h, v_may_25_09h, v_may_25_09h),
    (1001, 1001, v_may_28_13h, v_may_28_15h, v_may_28_13h, v_may_28_13h),
    -- Recent visits (should NOT be deleted) - within 7 days
    (1001, 1003, v_6_days_ago, v_6_days_ago + INTERVAL '1 hour', v_6_days_ago, v_now),
    (1001, 1003, v_3_days_ago, v_3_days_ago + INTERVAL '2 hours', v_3_days_ago, v_now),
    (1001, 1003, v_1_day_ago, v_1_day_ago + INTERVAL '4 hours', v_1_day_ago, v_now);

    -- Student 1002 (14 day retention)
    INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at) VALUES
    -- Old visits (should be deleted) - older than 14 days
    (1002, 1001, v_may_01_11h, v_may_01_13h, v_may_01_11h, v_may_01_11h),
    (1002, 1001, v_may_10_15h, v_may_10_16h, v_may_10_15h, v_may_10_15h),
    (1002, 1001, v_may_20_10h, v_may_20_13h, v_may_20_10h, v_may_20_10h),
    -- Recent visits (should NOT be deleted) - within 14 days
    (1002, 1002, v_13_days_ago, v_13_days_ago + INTERVAL '1 hour', v_13_days_ago, v_now),
    (1002, 1002, v_7_days_ago, v_7_days_ago + INTERVAL '2 hours', v_7_days_ago, v_now),
    (1002, 1003, v_2_days_ago, v_2_days_ago + INTERVAL '3 hours', v_2_days_ago, v_now);

    -- Student 1003 (30 day retention)
    INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at) VALUES
    -- Old visits (should be deleted) - older than 30 days
    (1003, 1001, v_apr_01_12h, v_apr_01_14h, v_apr_01_12h, v_apr_01_12h),
    (1003, 1001, v_apr_15_16h, v_apr_15_17h, v_apr_15_16h, v_apr_15_16h),
    -- Recent visits (should NOT be deleted) - within 30 days
    (1003, 1002, v_29_days_ago, v_29_days_ago + INTERVAL '3 hours', v_29_days_ago, v_now),
    (1003, 1002, v_15_days_ago, v_15_days_ago + INTERVAL '2 hours', v_15_days_ago, v_now),
    (1003, 1003, v_5_days_ago, v_5_days_ago + INTERVAL '1 hour', v_5_days_ago, v_now),
    (1003, 1003, v_1_day_ago, NULL, v_1_day_ago, v_now); -- Active visit (no exit time)
END $$;

-- Summary of expected deletions:
-- Student 1001: 4 old visits (older than 7 days) - 4 from May 2025
-- Student 1002: 3 old visits (older than 14 days) - 3 from early/mid May 2025
-- Student 1003: 2 old visits (older than 30 days) - 2 from April 2025
-- Total: 9 visits should be deleted
