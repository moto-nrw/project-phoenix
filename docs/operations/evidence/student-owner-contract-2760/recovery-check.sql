\set ON_ERROR_STOP on
SELECT 'profiles', count(*), encode(sha256(convert_to(coalesce(jsonb_agg(to_jsonb(t) ORDER BY t.id)::text, '[]'), 'UTF8')), 'hex') FROM users.student_profiles t
UNION ALL SELECT 'memberships', count(*), encode(sha256(convert_to(coalesce(jsonb_agg(to_jsonb(t) ORDER BY t.id)::text, '[]'), 'UTF8')), 'hex') FROM users.student_school_memberships t
UNION ALL SELECT 'care', count(*), encode(sha256(convert_to(coalesce(jsonb_agg(to_jsonb(t) ORDER BY t.membership_id)::text, '[]'), 'UTF8')), 'hex') FROM users.student_care_profiles t
UNION ALL SELECT 'archive', count(*), encode(sha256(convert_to(coalesce(jsonb_agg(to_jsonb(t) ORDER BY t.id)::text, '[]'), 'UTF8')), 'hex') FROM users.students_legacy t
UNION ALL SELECT 'history', count(*), encode(sha256(convert_to(coalesce(jsonb_agg(to_jsonb(t) ORDER BY t.id)::text, '[]'), 'UTF8')), 'hex') FROM active.student_status_days t
UNION ALL SELECT 'persons', count(*), encode(sha256(convert_to(coalesce(jsonb_agg(to_jsonb(t) ORDER BY t.id)::text, '[]'), 'UTF8')), 'hex') FROM users.persons t;
SELECT 'absence', count(*) FILTER (WHERE sick), count(*) FILTER (WHERE excused), count(sick_since), count(excused_since) FROM users.student_care_profiles;
SELECT 'owner_rls', relname, relrowsecurity, relforcerowsecurity FROM pg_class
WHERE oid IN ('users.student_profiles'::regclass, 'users.student_school_memberships'::regclass, 'users.student_care_profiles'::regclass) ORDER BY relname;
SELECT 'owner_fks', count(*), bool_and(convalidated) FROM pg_constraint WHERE contype = 'f'
AND conrelid IN ('users.student_profiles'::regclass, 'users.student_school_memberships'::regclass, 'users.student_care_profiles'::regclass);
SELECT 'rollback_view', count(*) FROM users.students;
