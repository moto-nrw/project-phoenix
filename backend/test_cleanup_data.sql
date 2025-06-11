-- Test data for cleanup feature testing
-- Run this after migrations to create test data
-- NOTE: This assumes the PostgreSQL schemas already exist from migrations

-- Create test education groups first (for students)
INSERT INTO education.groups (id, name, created_at, updated_at) VALUES
(1, 'Test Class 1', NOW(), NOW()),
(2, 'Test Class 2', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Create test activities (for active groups)
INSERT INTO activities.categories (id, name, created_at, updated_at) VALUES
(1, 'Test Category', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO activities.groups (id, name, max_participants, is_open, category_id, created_at, updated_at) VALUES
(1, 'Test Activity Group 1', 30, true, 1, NOW(), NOW()),
(2, 'Test Activity Group 2', 30, true, 1, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Create test students
INSERT INTO users.persons (id, first_name, last_name, created_at, updated_at) VALUES
(1001, 'Test', 'Student1', NOW(), NOW()),
(1002, 'Test', 'Student2', NOW(), NOW()),
(1003, 'Test', 'Student3', NOW(), NOW());

INSERT INTO users.students (id, person_id, school_class, guardian_name, guardian_contact, group_id, created_at, updated_at) VALUES
(1001, 1001, '5a', 'Test Guardian 1', 'guardian1@test.com', 1, NOW(), NOW()),
(1002, 1002, '5b', 'Test Guardian 2', 'guardian2@test.com', 1, NOW(), NOW()),
(1003, 1003, '5c', 'Test Guardian 3', 'guardian3@test.com', 2, NOW(), NOW());

-- Create privacy consents with different retention periods
INSERT INTO users.privacy_consents (student_id, policy_version, accepted, accepted_at, data_retention_days, created_at, updated_at) VALUES
(1001, '1.0', true, NOW() - INTERVAL '60 days', 7, NOW(), NOW()),  -- 7 days retention
(1002, '1.0', true, NOW() - INTERVAL '60 days', 14, NOW(), NOW()), -- 14 days retention
(1003, '1.0', true, NOW() - INTERVAL '60 days', 30, NOW(), NOW()); -- 30 days retention

-- Create test devices first (required for active groups)
INSERT INTO iot.devices (id, name, device_type, device_id, status, created_at, updated_at) VALUES
(1001, 'Test Device 1', 'rfid_reader', 'TEST-001', 'active', NOW(), NOW()),
(1002, 'Test Device 2', 'rfid_reader', 'TEST-002', 'active', NOW(), NOW()),
(1003, 'Test Device 3', 'rfid_reader', 'TEST-003', 'active', NOW(), NOW())
ON CONFLICT (device_id) DO NOTHING;

-- Create test rooms (if they don't exist)
INSERT INTO facilities.rooms (id, name, building, capacity, created_at, updated_at) VALUES
(1, 'Test Room 1', 'Building A', 30, NOW(), NOW()),
(2, 'Test Room 2', 'Building A', 30, NOW(), NOW()),
(3, 'Test Room 3', 'Building B', 30, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Create active groups for visits
INSERT INTO active.groups (id, device_id, group_id, room_id, start_time, created_at, updated_at) VALUES
(1001, 1001, 1, 1, NOW() - INTERVAL '40 days', NOW() - INTERVAL '40 days', NOW()),
(1002, 1002, 1, 2, NOW() - INTERVAL '20 days', NOW() - INTERVAL '20 days', NOW()),
(1003, 1003, 2, 3, NOW() - INTERVAL '10 days', NOW() - INTERVAL '10 days', NOW());

-- Create visits with different ages for each student
-- Student 1001 (7 day retention)
INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at) VALUES
-- Old visits (should be deleted) - explicitly old dates
(1001, 1001, '2025-05-01 10:00:00+00', '2025-05-01 12:00:00+00', '2025-05-01 10:00:00+00', '2025-05-01 10:00:00+00'),
(1001, 1001, '2025-05-15 14:00:00+00', '2025-05-15 15:00:00+00', '2025-05-15 14:00:00+00', '2025-05-15 14:00:00+00'),
(1001, 1001, '2025-05-25 09:00:00+00', '2025-05-25 12:00:00+00', '2025-05-25 09:00:00+00', '2025-05-25 09:00:00+00'),
(1001, 1001, '2025-05-28 13:00:00+00', '2025-05-28 15:00:00+00', '2025-05-28 13:00:00+00', '2025-05-28 13:00:00+00'),
-- Recent visits (should NOT be deleted) - within 7 days
(1001, 1003, NOW() - INTERVAL '6 days', NOW() - INTERVAL '6 days' + INTERVAL '1 hour', NOW() - INTERVAL '6 days', NOW()),
(1001, 1003, NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days' + INTERVAL '2 hours', NOW() - INTERVAL '3 days', NOW()),
(1001, 1003, NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day' + INTERVAL '4 hours', NOW() - INTERVAL '1 day', NOW());

-- Student 1002 (14 day retention)
INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at) VALUES
-- Old visits (should be deleted) - older than 14 days
(1002, 1001, '2025-05-01 11:00:00+00', '2025-05-01 13:00:00+00', '2025-05-01 11:00:00+00', '2025-05-01 11:00:00+00'),
(1002, 1001, '2025-05-10 15:00:00+00', '2025-05-10 16:00:00+00', '2025-05-10 15:00:00+00', '2025-05-10 15:00:00+00'),
(1002, 1001, '2025-05-20 10:00:00+00', '2025-05-20 13:00:00+00', '2025-05-20 10:00:00+00', '2025-05-20 10:00:00+00'),
-- Recent visits (should NOT be deleted) - within 14 days
(1002, 1002, NOW() - INTERVAL '13 days', NOW() - INTERVAL '13 days' + INTERVAL '1 hour', NOW() - INTERVAL '13 days', NOW()),
(1002, 1002, NOW() - INTERVAL '7 days', NOW() - INTERVAL '7 days' + INTERVAL '2 hours', NOW() - INTERVAL '7 days', NOW()),
(1002, 1003, NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days' + INTERVAL '3 hours', NOW() - INTERVAL '2 days', NOW());

-- Student 1003 (30 day retention)
INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at) VALUES
-- Old visits (should be deleted) - older than 30 days
(1003, 1001, '2025-04-01 12:00:00+00', '2025-04-01 14:00:00+00', '2025-04-01 12:00:00+00', '2025-04-01 12:00:00+00'),
(1003, 1001, '2025-04-15 16:00:00+00', '2025-04-15 17:00:00+00', '2025-04-15 16:00:00+00', '2025-04-15 16:00:00+00'),
-- Recent visits (should NOT be deleted) - within 30 days
(1003, 1002, NOW() - INTERVAL '29 days', NOW() - INTERVAL '29 days' + INTERVAL '3 hours', NOW() - INTERVAL '29 days', NOW()),
(1003, 1002, NOW() - INTERVAL '15 days', NOW() - INTERVAL '15 days' + INTERVAL '2 hours', NOW() - INTERVAL '15 days', NOW()),
(1003, 1003, NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days' + INTERVAL '1 hour', NOW() - INTERVAL '5 days', NOW()),
(1003, 1003, NOW() - INTERVAL '1 day', NULL, NOW() - INTERVAL '1 day', NOW()); -- Active visit (no exit time)

-- Summary of expected deletions:
-- Student 1001: 4 old visits (older than 7 days) - 4 from May 2025
-- Student 1002: 3 old visits (older than 14 days) - 3 from early/mid May 2025
-- Student 1003: 2 old visits (older than 30 days) - 2 from April 2025
-- Total: 9 visits should be deleted