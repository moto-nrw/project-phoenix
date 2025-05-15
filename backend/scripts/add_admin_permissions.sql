-- Grant admin permissions to a user
-- How to use: 
-- 1. Replace 'your.email@example.com' with the actual email address
-- 2. Run this SQL in your database, for example:
--    docker exec -i project-phoenix-postgres-1 psql -U postgres < add_admin_permissions.sql

-- Set search path
SET search_path TO auth, public;

-- Create admin permissions if they don't exist
INSERT INTO auth.permissions (name, description, resource, action)
VALUES ('admin:*', 'Full administrator access to all resources', 'admin', '*')
ON CONFLICT (name) DO NOTHING;

INSERT INTO auth.permissions (name, description, resource, action)
VALUES ('*:*', 'Full system access to all resources', '*', '*') 
ON CONFLICT (name) DO NOTHING;

-- Create room permissions if they don't exist
INSERT INTO auth.permissions (name, description, resource, action) 
VALUES ('rooms:create', 'Permission to create rooms', 'rooms', 'create')
ON CONFLICT (name) DO NOTHING;

INSERT INTO auth.permissions (name, description, resource, action)
VALUES ('rooms:read', 'Permission to read room data', 'rooms', 'read')
ON CONFLICT (name) DO NOTHING;

INSERT INTO auth.permissions (name, description, resource, action)
VALUES ('rooms:update', 'Permission to update rooms', 'rooms', 'update')
ON CONFLICT (name) DO NOTHING;

INSERT INTO auth.permissions (name, description, resource, action)
VALUES ('rooms:delete', 'Permission to delete rooms', 'rooms', 'delete')
ON CONFLICT (name) DO NOTHING;

INSERT INTO auth.permissions (name, description, resource, action)
VALUES ('rooms:list', 'Permission to list rooms', 'rooms', 'list')
ON CONFLICT (name) DO NOTHING;

INSERT INTO auth.permissions (name, description, resource, action)
VALUES ('rooms:manage', 'Permission to manage all room operations', 'rooms', 'manage')
ON CONFLICT (name) DO NOTHING;

-- Grant all permissions to the specified user
-- Replace 'your.email@example.com' with the actual email
DO $$
DECLARE
    user_id int;
    perm_id int;
    perm_name text;
BEGIN
    -- Get the user ID
    SELECT id INTO user_id FROM auth.accounts WHERE email = 'christan.kamann119@gmail.com';

    IF user_id IS NULL THEN
        RAISE EXCEPTION 'User with email christan.kamann119@gmail.com not found';
    END IF;

    -- Grant admin permission
    SELECT id INTO perm_id FROM auth.permissions WHERE name = 'admin:*';
    IF perm_id IS NOT NULL THEN
        INSERT INTO auth.account_permissions (account_id, permission_id)
        VALUES (user_id, perm_id)
        ON CONFLICT (account_id, permission_id) DO NOTHING;
        RAISE NOTICE 'Granted admin:* permission';
    ELSE
        RAISE NOTICE 'admin:* permission not found in database';
    END IF;

    -- Grant full access permission
    SELECT id INTO perm_id FROM auth.permissions WHERE name = '*:*';
    IF perm_id IS NOT NULL THEN
        INSERT INTO auth.account_permissions (account_id, permission_id)
        VALUES (user_id, perm_id)
        ON CONFLICT (account_id, permission_id) DO NOTHING;
        RAISE NOTICE 'Granted *:* permission';
    ELSE
        RAISE NOTICE '*:* permission not found in database';
    END IF;

    -- Grant all room permissions
    FOR perm_name, perm_id IN (SELECT name, id FROM auth.permissions WHERE name LIKE 'rooms:%')
    LOOP
        INSERT INTO auth.account_permissions (account_id, permission_id)
        VALUES (user_id, perm_id)
        ON CONFLICT (account_id, permission_id) DO NOTHING;
        RAISE NOTICE 'Granted % permission', perm_name;
    END LOOP;
END $$;

-- Show permissions for the user
SELECT a.email, p.name as permission
FROM auth.accounts a
JOIN auth.account_permissions ap ON a.id = ap.account_id
JOIN auth.permissions p ON p.id = ap.permission_id
WHERE a.email = 'christan.kamann119@gmail.com'
ORDER BY p.name;