# Request-child storage Expand: deployment evidence

Migration `1.15.375` implements the schema portion of #2712. It creates empty
submitted-choice and effective-booking tables. All application traffic still
uses `enrollment.request_child_offerings`. There is no backfill or dual write.

The approved supporting unique indexes on `enrollment.request_children` and
`enrollment.care_offerings` enforce tenant-safe references. Their creation can
wait for locks and block writes to those parent tables. They are part of the
transaction and are removed on rollback. Existing table ownership is unchanged.

## Evidence still required on staging

Local tests are not staging evidence. Before closing #2712, attach the following
to the issue or deployment record:

1. Previous and candidate image digests, migration start/end timestamps, elapsed
   duration, and migration result from the deployment log.
2. Sample lock waits and blocked sessions throughout migration, including
   zero-result samples. From a separate database connection, record the output
   of the query below at a fixed interval. Sampling bounds the observation;
   it does not prove that no shorter wait occurred.
3. Record `pg_stat_database.deadlocks` and `stats_reset` before and after the
   migration. Report the difference only when the counters were not reset.
4. Record table/index bytes and zero row counts for both target tables.
5. Keep the previous image running against the expanded database long enough to
   exercise ordinary enrollment reads, submission, and booking changes through
   that image. Record results and error/latency comparison with the pre-migration
   window. Confirm target row counts remain zero. Do not replace this check with
   direct SQL writes or the new image's tests.

Use the existing staging deployment process. The following queries are read-only;
run them on its database connection, not against a production API.

```sql
SELECT clock_timestamp() AS sampled_at, pid, application_name,
       wait_event_type, wait_event, query_start,
       pg_blocking_pids(pid) AS blocking_pids
FROM pg_stat_activity
WHERE datname = current_database()
  AND (wait_event_type = 'Lock' OR cardinality(pg_blocking_pids(pid)) > 0);

SELECT clock_timestamp() AS sampled_at, deadlocks, stats_reset
FROM pg_stat_database WHERE datname = current_database();

SELECT c.oid::regclass AS relation,
       pg_table_size(c.oid) AS table_bytes,
       pg_indexes_size(c.oid) AS index_bytes,
       pg_total_relation_size(c.oid) AS total_bytes
FROM pg_class c
WHERE c.oid IN (
  'enrollment.request_child_offering_selections'::regclass,
  'enrollment.care_offering_bookings'::regclass
);

SELECT pg_relation_size('enrollment.request_children_expand_tenant_id') AS child_key_bytes,
       pg_relation_size('enrollment.care_offerings_expand_tenant_id') AS offering_key_bytes;

SELECT (SELECT count(*) FROM enrollment.request_child_offering_selections) AS selections,
       (SELECT count(*) FROM enrollment.care_offering_bookings) AS bookings;
```

## Rollback and later Cutover

The down migration locks both targets, checks they are empty, then drops them
and the two supporting indexes in one transaction. It refuses populated targets
and never deletes legacy rows. Do not bypass that guard after Cutover.

Cutover must introduce typed date fields and real seed coverage, removing the
two no-runtime-model date classifications and two empty-target seed exemptions.
It must also define how legacy effective state yields the immutable submitted
choice; Expand deliberately makes no such data conversion.
