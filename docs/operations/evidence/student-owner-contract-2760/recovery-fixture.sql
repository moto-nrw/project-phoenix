\set ON_ERROR_STOP on
BEGIN;
INSERT INTO platform.schools (organization_id, name, slug, subdomain)
SELECT (SELECT min(id) FROM platform.organizations), 'Contract Recovery ' || n,
       'contract-recovery-' || n, 'contract-recovery-' || n
FROM generate_series(1, 12) n;
INSERT INTO users.persons (tenant_id, first_name, last_name)
SELECT s.id, 'Recovery', 'Synthetic ' || n
FROM platform.schools s
CROSS JOIN LATERAL generate_series(1, CASE WHEN split_part(s.slug, '-', 3)::int <= 8 THEN 121 ELSE 120 END) n
WHERE s.slug LIKE 'contract-recovery-%';
INSERT INTO users.students (tenant_id, person_id, school_class, sick, sick_since)
SELECT tenant_id, id, '3a', row_number() OVER (ORDER BY id) <= 16,
       CASE WHEN row_number() OVER (ORDER BY id) <= 16 THEN timestamp '2026-09-19 08:00:00' END
FROM users.persons WHERE first_name = 'Recovery';
INSERT INTO active.student_status_days (tenant_id, student_id, date, status, reported_at)
SELECT p.tenant_id, p.id, d.day, 'sick', timestamp '2026-09-01 08:00:00'
FROM users.student_profiles p
CROSS JOIN (VALUES (date '2026-09-01'), (date '2026-09-02')) d(day)
ORDER BY p.id, d.day LIMIT 2830;
COMMIT;
