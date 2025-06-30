-- Remove admin role from test@teachers.com and give appropriate teacher permissions

DO $$
DECLARE
    v_account_id INTEGER;
    v_user_role_id INTEGER;
BEGIN
    -- Get account ID for test@teachers.com
    SELECT id INTO v_account_id FROM auth.accounts WHERE email = 'test@teachers.com';
    
    -- Get the regular user role
    SELECT id INTO v_user_role_id FROM auth.roles WHERE name = 'user';
    
    RAISE NOTICE 'Account ID: %, User Role ID: %', v_account_id, v_user_role_id;
    
    -- Remove all existing roles for this account
    DELETE FROM auth.account_roles WHERE account_id = v_account_id;
    
    -- Assign user role instead of admin
    IF v_user_role_id IS NOT NULL THEN
        INSERT INTO auth.account_roles (account_id, role_id, created_at, updated_at)
        VALUES (v_account_id, v_user_role_id, NOW(), NOW());
        RAISE NOTICE 'Assigned user role to test@teachers.com';
    END IF;
    
    -- Grant specific permissions that a teacher needs
    -- First check what permissions the user role already has
    RAISE NOTICE 'User role has permissions';
    
    -- If account_permissions table exists, grant additional teacher-specific permissions
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'auth' AND table_name = 'account_permissions') THEN
        -- Clear existing direct permissions
        DELETE FROM auth.account_permissions WHERE account_id = v_account_id;
        
        -- Grant teacher-specific permissions that might not be in user role
        INSERT INTO auth.account_permissions (account_id, permission_id, created_at, updated_at)
        SELECT v_account_id, p.id, NOW(), NOW()
        FROM auth.permissions p
        WHERE p.name IN (
            'groups:read',
            'groups:list',
            'students:read',
            'students:list',
            'rooms:read',
            'rooms:list',
            'visits:read',
            'visits:list',
            'visits:create',
            'visits:update'
        )
        AND NOT EXISTS (
            -- Don't add if user role already has this permission
            SELECT 1 FROM auth.role_permissions rp 
            WHERE rp.role_id = v_user_role_id AND rp.permission_id = p.id
        )
        ON CONFLICT (account_id, permission_id) DO NOTHING;
        
        RAISE NOTICE 'Granted additional teacher permissions to test@teachers.com';
    END IF;
END $$;

-- Show what roles the user now has
SELECT 
    a.email,
    r.name as role_name,
    r.description
FROM auth.accounts a
JOIN auth.account_roles ar ON a.id = ar.account_id
JOIN auth.roles r ON ar.role_id = r.id
WHERE a.email = 'test@teachers.com';

-- Show all permissions the user now has (from role + direct)
SELECT DISTINCT
    a.email,
    p.name as permission_name,
    CASE 
        WHEN ap.account_id IS NOT NULL THEN 'Direct'
        WHEN rp.role_id IS NOT NULL THEN 'From Role'
    END as source
FROM auth.accounts a
LEFT JOIN auth.account_roles ar ON a.id = ar.account_id
LEFT JOIN auth.role_permissions rp ON ar.role_id = rp.role_id
LEFT JOIN auth.permissions p ON rp.permission_id = p.id OR p.id IN (
    SELECT permission_id FROM auth.account_permissions WHERE account_id = a.id
)
LEFT JOIN auth.account_permissions ap ON a.id = ap.account_id AND p.id = ap.permission_id
WHERE a.email = 'test@teachers.com' 
AND p.name IS NOT NULL
ORDER BY p.name;