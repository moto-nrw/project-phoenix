-- Simple test data for student location testing
-- Run this after the seed command has been executed

DO $$
DECLARE
    v_account_id INTEGER;
    v_person_id INTEGER;
    v_staff_id INTEGER;
    v_teacher_id INTEGER;
    v_student_ids INTEGER[] := ARRAY[]::INTEGER[];
    v_student_id INTEGER;
    v_group_ids INTEGER[] := ARRAY[]::INTEGER[];
    v_group_id INTEGER;
    v_room_id INTEGER;
BEGIN
    -- 1. Create test@teachers.com account if it doesn't exist
    INSERT INTO auth.accounts (email, password_hash, active, created_at, updated_at)
    VALUES ('test@teachers.com', '$2a$12$LQv3c1yqBwLFaT.UU.5uPeKHYyFHdGcJ8..KJjdHsX2Yyh.1qBq0S', TRUE, NOW(), NOW())
    ON CONFLICT (email) DO UPDATE SET
        active = TRUE,
        updated_at = NOW()
    RETURNING id INTO v_account_id;
    
    -- Get the account ID if it already existed
    IF v_account_id IS NULL THEN
        SELECT id INTO v_account_id FROM auth.accounts WHERE email = 'test@teachers.com';
    END IF;
    
    -- 2. Create person record
    INSERT INTO users.persons (account_id, first_name, last_name, tag_id, created_at, updated_at)
    VALUES (v_account_id, 'Test', 'Teacher', NULL, NOW(), NOW())
    ON CONFLICT (account_id) DO UPDATE SET
        first_name = 'Test',
        last_name = 'Teacher',
        updated_at = NOW()
    RETURNING id INTO v_person_id;
    
    -- Get the person ID if it already existed
    IF v_person_id IS NULL THEN
        SELECT id INTO v_person_id FROM users.persons WHERE account_id = v_account_id;
    END IF;
    
    -- 3. Create staff record
    INSERT INTO users.staff (person_id, staff_notes, created_at, updated_at)
    VALUES (v_person_id, 'Test teacher account', NOW(), NOW())
    ON CONFLICT (person_id) DO UPDATE SET
        staff_notes = 'Test teacher account',
        updated_at = NOW()
    RETURNING id INTO v_staff_id;
    
    -- Get the staff ID if it already existed
    IF v_staff_id IS NULL THEN
        SELECT id INTO v_staff_id FROM users.staff WHERE person_id = v_person_id;
    END IF;
    
    -- 4. Create teacher record
    INSERT INTO users.teachers (staff_id, specialization, role, created_at, updated_at)
    VALUES (v_staff_id, 'Grundschullehramt', 'Klassenlehrer', NOW(), NOW())
    ON CONFLICT (staff_id) DO UPDATE SET
        specialization = 'Grundschullehramt',
        role = 'Klassenlehrer',
        updated_at = NOW()
    RETURNING id INTO v_teacher_id;
    
    -- Get the teacher ID if it already existed
    IF v_teacher_id IS NULL THEN
        SELECT id INTO v_teacher_id FROM users.teachers WHERE staff_id = v_staff_id;
    END IF;
    
    -- 5. Assign teacher to first few educational groups
    INSERT INTO education.group_teacher (group_id, teacher_id)
    SELECT g.id, v_teacher_id
    FROM education.groups g
    WHERE g.name LIKE 'Klasse %'
    ORDER BY g.id
    LIMIT 3
    ON CONFLICT (group_id, teacher_id) DO NOTHING;
    
    -- 6. Clear existing active data to start fresh
    DELETE FROM active.visits;
    DELETE FROM active.group_supervisors;
    DELETE FROM active.groups;
    
    -- 7. Create active groups for rooms 1, 2, 3 (101, 102, 103)
    INSERT INTO active.groups (group_id, room_id, device_id, start_time, end_time, created_at, updated_at)
    SELECT g.id, g.room_id, NULL, NOW() - INTERVAL '2 hours', NULL, NOW(), NOW()
    FROM education.groups g
    WHERE g.room_id IN (1, 2, 3)
    ORDER BY g.room_id;
    
    -- 8. Create supervisor assignments for test teacher
    INSERT INTO active.group_supervisors (group_id, staff_id, role, start_date, end_date, created_at, updated_at)
    SELECT ag.id, v_staff_id, 'supervisor', CURRENT_DATE, NULL, NOW(), NOW()
    FROM active.groups ag
    JOIN education.groups eg ON ag.group_id = eg.id
    WHERE eg.room_id IN (1, 2, 3);
    
    -- 9. Get some student IDs from each group (we'll create visits in next steps)
    
    -- 10. Create visits for students in different rooms
    -- Students in room 1 (101)
    INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at)
    SELECT s.id, ag.id, NOW() - INTERVAL '90 minutes', NULL, NOW(), NOW()
    FROM users.students s
    JOIN education.groups g ON s.group_id = g.id
    JOIN active.groups ag ON ag.group_id = g.id
    WHERE g.room_id = 1
    ORDER BY s.id
    LIMIT 2;
    
    -- Students in room 2 (102)
    INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at)
    SELECT s.id, ag.id, NOW() - INTERVAL '60 minutes', NULL, NOW(), NOW()
    FROM users.students s
    JOIN education.groups g ON s.group_id = g.id
    JOIN active.groups ag ON ag.group_id = g.id
    WHERE g.room_id = 2
    ORDER BY s.id
    LIMIT 2;
    
    -- Students in room 3 (103)
    INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at)
    SELECT s.id, ag.id, NOW() - INTERVAL '30 minutes', NULL, NOW(), NOW()
    FROM users.students s
    JOIN education.groups g ON s.group_id = g.id
    JOIN active.groups ag ON ag.group_id = g.id
    WHERE g.room_id = 3
    ORDER BY s.id
    LIMIT 2;
    
    RAISE NOTICE 'Test data setup completed successfully!';
    RAISE NOTICE 'Teacher ID: %, Staff ID: %', v_teacher_id, v_staff_id;
    
    -- Show current visits
    RAISE NOTICE 'Current active visits created';
    
END $$;