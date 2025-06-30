-- Test data for Project Phoenix
-- This creates active groups, supervisors, and student visits for testing

-- First, let's check if we have the necessary users
DO $$
DECLARE
    v_teacher_id INTEGER;
    v_staff_id INTEGER;
    v_group1_id INTEGER;
    v_group2_id INTEGER;
    v_group3_id INTEGER;
BEGIN
    -- Get the test teacher's staff ID
    SELECT s.id INTO v_staff_id
    FROM users.staff s
    JOIN users.persons p ON s.person_id = p.id
    JOIN auth.accounts a ON p.account_id = a.id
    WHERE a.email = 'test@teachers.com';
    
    IF v_staff_id IS NULL THEN
        RAISE NOTICE 'test@teachers.com not found. Please ensure this user exists.';
        RETURN;
    END IF;
    
    -- Get the teacher ID
    SELECT id INTO v_teacher_id
    FROM users.teachers
    WHERE staff_id = v_staff_id;
    
    IF v_teacher_id IS NULL THEN
        RAISE NOTICE 'Teacher record not found for test@teachers.com. Please ensure this user is a teacher.';
        RETURN;
    END IF;
    
    -- Get group IDs
    SELECT id INTO v_group1_id FROM education.groups WHERE name = 'Klasse 1a';
    SELECT id INTO v_group2_id FROM education.groups WHERE name = 'Klasse 1b';
    SELECT id INTO v_group3_id FROM education.groups WHERE name = 'Klasse 2a';
    
    RAISE NOTICE 'Using teacher ID: %, staff ID: %', v_teacher_id, v_staff_id;
    RAISE NOTICE 'Group IDs - 1a: %, 1b: %, 2a: %', v_group1_id, v_group2_id, v_group3_id;
END $$;

-- Clear existing test data
DELETE FROM active.visits WHERE student_id IN (
    SELECT s.id FROM users.students s
    JOIN users.persons p ON s.person_id = p.id
    WHERE p.first_name IN ('Juna', 'Jakob', 'Pia', 'Paula')
    AND p.last_name IN ('Günther', 'Schäfer', 'Hartmann')
);

DELETE FROM active.supervisors WHERE staff_id IN (
    SELECT s.id FROM users.staff s
    JOIN users.persons p ON s.person_id = p.id
    JOIN auth.accounts a ON p.account_id = a.id
    WHERE a.email = 'test@teachers.com'
);

DELETE FROM active.groups WHERE id IN (12, 13, 14, 15, 16, 17);

-- Create active groups
INSERT INTO active.groups (id, group_id, room_id, device_id, start_time, end_time, created_at, updated_at) VALUES
(12, (SELECT id FROM education.groups WHERE name = 'Klasse 1a'), 1, NULL, NOW() - INTERVAL '2 hours', NULL, NOW(), NOW()),
(13, (SELECT id FROM education.groups WHERE name = 'Klasse 1b'), 2, NULL, NOW() - INTERVAL '2 hours', NULL, NOW(), NOW()),
(14, (SELECT id FROM education.groups WHERE name = 'Klasse 2a'), 3, NULL, NOW() - INTERVAL '2 hours', NULL, NOW(), NOW()),
(15, (SELECT id FROM education.groups WHERE name = 'Klasse 2b'), 4, NULL, NOW() - INTERVAL '2 hours', NULL, NOW(), NOW()),
(16, (SELECT id FROM education.groups WHERE name = 'Klasse 3a'), 5, NULL, NOW() - INTERVAL '2 hours', NULL, NOW(), NOW()),
(17, (SELECT id FROM education.groups WHERE name = 'Klasse 3b'), 6, NULL, NOW() - INTERVAL '2 hours', NULL, NOW(), NOW());

-- Reset sequence if needed
SELECT setval('active.groups_id_seq', (SELECT MAX(id) FROM active.groups) + 1, true);

-- Create supervisor assignments for test@teachers.com
INSERT INTO active.supervisors (active_group_id, staff_id, start_time, end_time, created_at, updated_at)
SELECT 
    ag.id,
    s.id,
    NOW() - INTERVAL '2 hours',
    NULL,
    NOW(),
    NOW()
FROM active.groups ag
CROSS JOIN users.staff s
JOIN users.persons p ON s.person_id = p.id
JOIN auth.accounts a ON p.account_id = a.id
WHERE a.email = 'test@teachers.com'
AND ag.id IN (12, 13, 14); -- Supervising first 3 active groups

-- Also ensure educational group supervision
INSERT INTO education.group_teacher (group_id, teacher_id)
SELECT DISTINCT eg.id, t.id
FROM education.groups eg
CROSS JOIN users.teachers t
JOIN users.staff s ON t.staff_id = s.id
JOIN users.persons p ON s.person_id = p.id
JOIN auth.accounts a ON p.account_id = a.id
WHERE a.email = 'test@teachers.com'
AND eg.name IN ('Klasse 1a', 'Klasse 1b', 'Klasse 2a')
ON CONFLICT (group_id, teacher_id) DO NOTHING;

-- Create activity group assignments for rooms
INSERT INTO activities.groups (activity_id, group_id, created_at, updated_at)
SELECT 
    (SELECT id FROM activities.activities WHERE name = 'Nachmittagsbetreuung' LIMIT 1),
    eg.id,
    NOW(),
    NOW()
FROM education.groups eg
WHERE eg.name IN ('Klasse 1a', 'Klasse 1b', 'Klasse 2a')
ON CONFLICT (activity_id, group_id) DO NOTHING;

-- Create activity group supervisor assignments
INSERT INTO activities.group_supervisors (activity_group_id, teacher_id, created_at, updated_at)
SELECT DISTINCT
    ag.id,
    t.id,
    NOW(),
    NOW()
FROM activities.groups ag
JOIN education.groups eg ON ag.group_id = eg.id
CROSS JOIN users.teachers t
JOIN users.staff s ON t.staff_id = s.id
JOIN users.persons p ON s.person_id = p.id
JOIN auth.accounts a ON p.account_id = a.id
WHERE a.email = 'test@teachers.com'
AND eg.name IN ('Klasse 1a', 'Klasse 1b', 'Klasse 2a')
ON CONFLICT (activity_group_id, teacher_id) DO NOTHING;

-- Create student visits
-- Juna Günther (ID: 3) and Jakob Günther (ID: 2) in Gruppenraum Blau (room 1)
INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at)
VALUES 
    (3, 12, NOW() - INTERVAL '90 minutes', NULL, NOW(), NOW()),  -- Juna in active group 12
    (2, 12, NOW() - INTERVAL '70 minutes', NULL, NOW(), NOW());  -- Jakob in active group 12

-- Pia Schäfer (ID: 12) in room 101 (room 2)
INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at)
VALUES 
    (12, 13, NOW() - INTERVAL '50 minutes', NULL, NOW(), NOW());  -- Pia in active group 13

-- Paula Hartmann (ID: 13) in room 102 (room 3)
INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at)
VALUES 
    (13, 14, NOW() - INTERVAL '30 minutes', NULL, NOW(), NOW());  -- Paula in active group 14

-- Create some attendance records for today
INSERT INTO active.attendance (student_id, check_in_time, check_out_time, created_at, updated_at)
SELECT 
    v.student_id,
    v.entry_time,
    NULL,
    NOW(),
    NOW()
FROM active.visits v
WHERE v.exit_time IS NULL
ON CONFLICT (student_id, check_in_time) DO NOTHING;

-- Display summary
DO $$
DECLARE
    v_active_groups INTEGER;
    v_active_visits INTEGER;
    v_supervisors INTEGER;
BEGIN
    SELECT COUNT(*) INTO v_active_groups FROM active.groups WHERE end_time IS NULL;
    SELECT COUNT(*) INTO v_active_visits FROM active.visits WHERE exit_time IS NULL;
    SELECT COUNT(*) INTO v_supervisors FROM active.supervisors WHERE end_time IS NULL;
    
    RAISE NOTICE '';
    RAISE NOTICE '=== Test Data Created Successfully ===';
    RAISE NOTICE 'Active Groups: %', v_active_groups;
    RAISE NOTICE 'Active Visits: %', v_active_visits;
    RAISE NOTICE 'Active Supervisors: %', v_supervisors;
    RAISE NOTICE '';
    RAISE NOTICE 'Students currently checked in:';
    
    FOR r IN 
        SELECT 
            p.first_name || ' ' || p.last_name AS student_name,
            rm.name AS room_name,
            v.entry_time
        FROM active.visits v
        JOIN users.students s ON v.student_id = s.id
        JOIN users.persons p ON s.person_id = p.id
        JOIN active.groups ag ON v.active_group_id = ag.id
        JOIN facilities.rooms rm ON ag.room_id = rm.id
        WHERE v.exit_time IS NULL
        ORDER BY v.entry_time
    LOOP
        RAISE NOTICE '  - % in % (since %)', r.student_name, r.room_name, r.entry_time::time;
    END LOOP;
END $$;