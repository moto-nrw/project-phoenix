-- Fix permissions for test@teachers.com

-- First, let's check what roles exist
SELECT id, name, description FROM auth.roles;

-- Get the teacher role ID (usually it's the one for teachers)
DO $$
DECLARE
    v_account_id INTEGER;
    v_teacher_role_id INTEGER;
    v_admin_role_id INTEGER;
BEGIN
    -- Get account ID for test@teachers.com
    SELECT id INTO v_account_id FROM auth.accounts WHERE email = 'test@teachers.com';
    
    -- Get the teacher role
    SELECT id INTO v_teacher_role_id FROM auth.roles WHERE name = 'teacher' OR name = 'Teacher';
    
    -- If no teacher role exists, get any role that might work
    IF v_teacher_role_id IS NULL THEN
        SELECT id INTO v_teacher_role_id FROM auth.roles WHERE name ILIKE '%teach%' LIMIT 1;
    END IF;
    
    -- Get admin role as fallback
    SELECT id INTO v_admin_role_id FROM auth.roles WHERE name = 'admin' OR name = 'Admin';
    
    RAISE NOTICE 'Account ID: %, Teacher Role ID: %, Admin Role ID: %', v_account_id, v_teacher_role_id, v_admin_role_id;
    
    -- Remove any existing roles for this account
    DELETE FROM auth.account_roles WHERE account_id = v_account_id;
    
    -- Assign teacher role if found
    IF v_teacher_role_id IS NOT NULL THEN
        INSERT INTO auth.account_roles (account_id, role_id, created_at, updated_at)
        VALUES (v_account_id, v_teacher_role_id, NOW(), NOW());
        RAISE NOTICE 'Assigned teacher role to test@teachers.com';
    ELSE
        -- If no teacher role, assign admin role for testing
        IF v_admin_role_id IS NOT NULL THEN
            INSERT INTO auth.account_roles (account_id, role_id, created_at, updated_at)
            VALUES (v_account_id, v_admin_role_id, NOW(), NOW());
            RAISE NOTICE 'Assigned admin role to test@teachers.com (no teacher role found)';
        END IF;
    END IF;
    
    -- Also grant specific permissions directly if needed
    -- Check if we need to grant permissions directly
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'auth' AND table_name = 'account_permissions') THEN
        -- Grant essential permissions for teachers
        DELETE FROM auth.account_permissions WHERE account_id = v_account_id;
        
        -- Insert permissions
        INSERT INTO auth.account_permissions (account_id, permission_id, created_at, updated_at)
        SELECT v_account_id, p.id, NOW(), NOW()
        FROM auth.permissions p
        WHERE p.name IN (
            'students.read',
            'groups.read',
            'rooms.read',
            'active.read',
            'analytics.read',
            'me.read'
        )
        ON CONFLICT (account_id, permission_id) DO NOTHING;
        
        RAISE NOTICE 'Granted individual permissions to test@teachers.com';
    END IF;
END $$;

-- Show what roles the user has
SELECT 
    a.email,
    r.name as role_name,
    r.description
FROM auth.accounts a
JOIN auth.account_roles ar ON a.id = ar.account_id
JOIN auth.roles r ON ar.role_id = r.id
WHERE a.email = 'test@teachers.com';

-- Show what permissions the user has
SELECT 
    a.email,
    p.name as permission_name,
    p.description
FROM auth.accounts a
LEFT JOIN auth.account_permissions ap ON a.id = ap.account_id
LEFT JOIN auth.permissions p ON ap.permission_id = p.id
WHERE a.email = 'test@teachers.com'
ORDER BY p.name;