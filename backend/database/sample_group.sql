-- Insert a sample room first (groups can optionally reference rooms)
INSERT INTO facilities.rooms (name, building, floor, capacity, category, color)
VALUES ('OGS Room 1', 'Main Building', 1, 25, 'OGS', '#FF5733');

-- Insert a sample OGS group
INSERT INTO education.groups (name, room_id)
VALUES ('OGS Group 1', (SELECT id FROM facilities.rooms WHERE name = 'OGS Room 1'));