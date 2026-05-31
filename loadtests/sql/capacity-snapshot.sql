-- Run against staging or production while a load test is active.
-- Example:
-- docker exec -i staging-postgres-1 psql -U postgres -d postgres < loadtests/sql/capacity-snapshot.sql

\timing on

SELECT
  now() AS captured_at,
  count(*) AS connections,
  count(*) FILTER (WHERE state = 'active') AS active,
  count(*) FILTER (WHERE wait_event_type IS NOT NULL) AS waiting,
  count(*) FILTER (WHERE state = 'idle in transaction') AS idle_in_transaction
FROM pg_stat_activity
WHERE datname = current_database();

SELECT
  wait_event_type,
  wait_event,
  count(*) AS sessions
FROM pg_stat_activity
WHERE datname = current_database()
  AND wait_event_type IS NOT NULL
GROUP BY wait_event_type, wait_event
ORDER BY sessions DESC, wait_event_type, wait_event;

SELECT
  relname,
  seq_scan,
  idx_scan,
  n_live_tup,
  n_dead_tup,
  last_vacuum,
  last_autovacuum
FROM pg_stat_user_tables
ORDER BY n_live_tup DESC
LIMIT 25;

SELECT
  round((total_exec_time / 1000)::numeric, 2) AS total_exec_seconds,
  calls,
  round(mean_exec_time::numeric, 2) AS mean_exec_ms,
  round(max_exec_time::numeric, 2) AS max_exec_ms,
  rows,
  left(query, 220) AS query
FROM pg_stat_statements
ORDER BY total_exec_time DESC
LIMIT 25;
