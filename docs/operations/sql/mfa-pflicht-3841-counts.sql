-- Read-only inventory. Output: school IDs and counts only, never account IDs.
-- Pre-release compatibility: optional erasure marker arrives only in 1.15.464.
-- Missing JSON keys and SQL NULL both mean no recorded erasure.
-- Keep the named MFA minimum in mfa_privilege.go aligned (classification test).
WITH privileged_permission(name) AS (
 VALUES
 ('admin:*'),
 ('*:*'),
 ('auth:manage'),
 ('config:manage'),
 ('system:manage'),
 ('users:manage'),
 ('roles:create'),
 ('roles:read'),
 ('roles:update'),
 ('roles:delete'),
 ('permissions:create'),
 ('permissions:read'),
 ('permissions:update'),
 ('permissions:delete'),
 ('staff:manage'),
 ('staff:stammdaten'),
 ('staff:documents'),
 ('staff:financial'),
 ('time_tracking:manage'),
 ('vacation:approve'),
 ('staff_documents:health'),
 ('student_documents:health'),
 ('student_accidents:read'),
 ('student_accidents:manage'),
 ('enrollment_uploads:read')
), targets AS (
 SELECT name FROM privileged_permission
 UNION SELECT resource || ':' || action FROM auth.permissions WHERE action = 'health'
), privileged_accounts AS (
 SELECT DISTINCT mapping.tenant_id, account.id, account.email
 FROM auth.account_tenants mapping
 JOIN auth.accounts account ON account.id = mapping.account_id
 JOIN auth.account_roles ar ON ar.account_id = account.id AND ar.tenant_id = mapping.tenant_id
 JOIN auth.role_permissions rp ON rp.role_id = ar.role_id
 JOIN auth.permissions grant_permission ON grant_permission.id = rp.permission_id
 WHERE mapping.status = 'active' AND account.active
 AND to_jsonb(account) ->> 'personal_data_erased_at' IS NULL
 AND EXISTS (
  SELECT FROM targets target
  WHERE (grant_permission.resource = split_part(target.name, ':', 1)
   OR (right(grant_permission.resource, 1) = '*'
    AND starts_with(split_part(target.name, ':', 1), left(grant_permission.resource, length(grant_permission.resource) - 1))))
  AND (grant_permission.action = split_part(target.name, ':', 2)
   OR (right(grant_permission.action, 1) = '*'
    AND starts_with(split_part(target.name, ':', 2), left(grant_permission.action, length(grant_permission.action) - 1))))
 )
), enrollment AS (
 SELECT p.tenant_id, p.id,
  NULLIF(trim(p.email), '') IS NULL AS no_email,
  EXISTS (SELECT FROM auth.mfa_credentials m WHERE m.account_id = p.id) AS email_mfa,
  EXISTS (SELECT FROM auth.passkey_credentials k WHERE k.account_id = p.id AND k.revoked_at IS NULL) AS passkey
 FROM privileged_accounts p
)
SELECT school.id AS school_id,
 count(e.id) AS privileged_accounts,
 count(e.id) FILTER (WHERE NOT e.email_mfa) AS without_email_mfa,
 count(e.id) FILTER (WHERE NOT e.email_mfa AND NOT e.passkey) AS without_any_second_factor,
 count(e.id) FILTER (WHERE e.no_email) AS without_email_address
FROM platform.schools school
LEFT JOIN enrollment e ON e.tenant_id = school.id
WHERE school.active AND school.deleted_at IS NULL
GROUP BY school.id ORDER BY school.id;
