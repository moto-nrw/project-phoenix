BEGIN READ ONLY;
SET LOCAL statement_timeout = '10s';
SELECT jsonb_build_object(
    'database', current_database(),
    'system_identifier', (SELECT system_identifier::text FROM pg_control_system()),
    'database_oid', (SELECT oid::bigint FROM pg_database WHERE datname = current_database()),
    'compatibility_objects', (SELECT count(*) FROM pg_class WHERE oid IN
        (to_regclass('users.student_compatibility_reads'), to_regclass('users.student_compatibility_writes'))),
    'reads', coalesce(pg_sequence_last_value(to_regclass('users.student_compatibility_reads')), 0),
    'writes', coalesce(pg_sequence_last_value(to_regclass('users.student_compatibility_writes')), 0),
    'statistics_reset', stats_reset,
    'statistics_dealloc', dealloc,
    'tracking_all', (current_setting('pg_stat_statements.track') = 'all'
        AND current_setting('pg_stat_statements.track_utility')::boolean AND NOT EXISTS (
        SELECT 1 FROM pg_db_role_setting settings, unnest(settings.setconfig) setting
        WHERE settings.setdatabase IN (0, (SELECT oid FROM pg_database WHERE datname = current_database()))
        AND ((setting LIKE 'pg_stat_statements.track=%' AND setting <> 'pg_stat_statements.track=all')
        OR (setting LIKE 'pg_stat_statements.track_utility=%' AND setting <> 'pg_stat_statements.track_utility=on')))),
    'old_query_fingerprint', (SELECT encode(sha256(convert_to(coalesce(string_agg(
        jsonb_build_array(s.userid, s.queryid, s.toplevel, s.calls, s.rows, s.stats_since)::text,
        '' ORDER BY s.userid, s.queryid, s.toplevel), ''), 'UTF8')), 'hex')
        FROM public.pg_stat_statements s JOIN pg_roles r ON r.oid = s.userid
        WHERE s.dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
        AND NOT r.rolsuper
        AND replace(s.query, '"', '') ~* '\m(students|students_legacy|expired_privacy_consents)\M'))
FROM public.pg_stat_statements_info;
COMMIT;
