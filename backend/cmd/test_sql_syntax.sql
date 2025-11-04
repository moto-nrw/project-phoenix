-- Test SQL syntax for seed data
-- This file tests if the SQL queries used in seed.go are valid

-- Test Room insertion
INSERT INTO facilities.rooms (name, building, floor, capacity, category, color, created_at, updated_at) 
VALUES ('Test Room', 'Test Building', 1, 30, 'Classroom', '#4A90E2', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) 
RETURNING id;

-- Test Group insertion with NULL room_id
INSERT INTO education.groups (name, room_id, created_at, updated_at) 
VALUES ('Test Group', NULL, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) 
RETURNING id;

-- Test Group insertion with room_id
INSERT INTO education.groups (name, room_id, created_at, updated_at) 
VALUES ('Test Group 2', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) 
RETURNING id;

-- Test Person insertion
INSERT INTO users.persons (first_name, last_name, tag_id, created_at, updated_at) 
VALUES ('John', 'Doe', 'RFID-001', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) 
RETURNING id;

-- Test Staff insertion
INSERT INTO users.staff (person_id, staff_notes, created_at, updated_at) 
VALUES (1, 'Test notes', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) 
RETURNING id;

-- Test Teacher insertion
INSERT INTO users.teachers (staff_id, specialization, role, qualifications, created_at, updated_at) 
VALUES (1, 'Mathematics', 'Teacher', 'B.Ed.', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) 
RETURNING id;

-- Test Student insertion
INSERT INTO users.students (person_id, school_class, bus, in_house, wc, school_yard, 
                           guardian_name, guardian_contact, guardian_email, guardian_phone, group_id, created_at, updated_at) 
VALUES (1, '1A', false, true, false, false,
        'Jane Doe', '+1 555-555-5555', 'jane.doe@email.com', '+1 555-555-5555', 1,
        CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) 
RETURNING id;

-- Test retrieving person last name
SELECT last_name FROM users.persons WHERE id = 1;