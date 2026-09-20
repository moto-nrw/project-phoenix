-- Run on the target database as the migration/inspection superuser at the
-- reviewed observation endpoint. Retain this result with the raw query review.
-- Historical calls are intentionally included. Their later re-execution must
-- change the fingerprint even when no new pg_stat_statements entry is created.
SELECT encode(sha256(convert_to(coalesce(string_agg(
    jsonb_build_array(s.userid, s.queryid, s.toplevel, s.calls, s.rows, s.stats_since)::text,
    '' ORDER BY s.userid, s.queryid, s.toplevel), ''), 'UTF8')), 'hex')
    AS old_query_fingerprint_end
FROM public.pg_stat_statements s
JOIN pg_roles r ON r.oid = s.userid
WHERE s.dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
AND NOT r.rolsuper
AND replace(s.query, '"', '') ~* '\m(students|students_legacy|expired_privacy_consents)\M';
