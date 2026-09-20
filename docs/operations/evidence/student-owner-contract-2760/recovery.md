# Local pre-Contract database restore observation

Observed 2026-09-20 on the disposable local Compose project
`project-phoenix-contract-evidence`, PostgreSQL 17.11 aarch64, port 5531.
`fsync`, `synchronous_commit` and `full_page_writes` were all `on`.
`shared_preload_libraries` was `pg_stat_statements`.

No production data or production connection was used. The source was created
from the retained local pre-Contract template `phoenix_test_49b653ad6475`,
which has the 1.15.398 compatibility view. The source and restored databases
are `student_contract_recovery_source_2760` and
`student_contract_recovery_restored_2760`.

## Executed sequence

1. `createdb -U postgres -T phoenix_test_49b653ad6475 student_contract_recovery_source_2760`
2. Run [recovery-fixture.sql](recovery-fixture.sql) with `psql -X`,
   `ON_ERROR_STOP` enabled. It inserted 12 schools, 1,448 people/students and
   2,830 historical status rows. The fixture intentionally uses the old view;
   its compatibility counters are nonzero and it is not operational
   zero-access evidence authorizing Contract.
3. Run [recovery-check.sql](recovery-check.sql) and retain
   [recovery-before.txt](recovery-before.txt).
4. `pg_dump -U postgres -d student_contract_recovery_source_2760 -Fc`
   wrote `/tmp/student-contract-recovery-2760.dump` with restrictive umask.
   Size: 1,827,589 bytes. SHA-256:
   `8345e854edff848c99c0b343b332cc53c818d5780ffe26e24a5b1a1849e6ceec`.
5. `createdb -U postgres -T template0 student_contract_recovery_restored_2760`
6. `pg_restore -U postgres --exit-on-error -d student_contract_recovery_restored_2760`
   consumed that exact dump and exited 0.
7. Run the same check SQL and retain
   [recovery-after.txt](recovery-after.txt). `diff -u` against the before file
   exited 0 with no differences.

All PostgreSQL commands ran through `docker exec` on
`project-phoenix-contract-evidence-postgres-test-1`. The dump remains local,
not in Git. Every row fingerprint includes every column, ordered by identity.
Observed: 1,448 rows in each of profile, membership, care, archive and person
storage; 2,830 history rows; 16 sick timestamps/flags, no excused flags; all
three owner tables have enabled and forced RLS; all eight outgoing owner
foreign keys validated; the restored rollback view returns 1,448 rows.

## Coverage boundary

This proves a real custom-format dump can restore the synthetic pre-Contract
database, with the measured rows and protections preserved. It uses an
existing PostgreSQL cluster whose global roles already exist. It does not
test restoring cluster-global roles, production data distributions, or a fresh
production backup. The Contract transaction
and its rollback-on-error are verified separately by the migration tests;
this restore exercise did not execute Contract against this source database.

## Prior-image startup on the restore

The actual prior server image `ed76e88` was exported read-only with
`docker image save` from `moto-app-server`, then loaded locally. No production
container was stopped or changed. Registry identity observed on that server:
`ghcr.io/moto-nrw/phoenix-server@sha256:d2ed5fee7a8665deba06dd7e19896701a4ddee334872c0464e150a808ab7e029`.
The exported configuration blob hashes to
`91e473903865b188afde544f4456f4640b0d09f83ca9c2f80cee7888e609397d`, matching
the server's image ID; all eight exported uncompressed layer hashes matched.
The local OCI manifest after save/load is
`sha256:c54190d2fc64afb0d4c312c8e06e81b859b253815269efea3229ca8a880bf5e9`.
Different compressed/transport manifests do not change the verified
configuration and filesystem layers. The image revision is
`ed76e883668985014427f8cd860e1fe7efb24ed0`.
See [identity checks](prior-image-identity.txt).

The image ran as `recovery-server` in the isolated local Compose project,
bound only to `127.0.0.1:18086`, using the restored database. It used generated
local credentials, development configuration, mock mail and disabled
schedulers, not production secrets or configuration. Startup first failed on
missing required metrics-token and portal-host configuration; after supplying
those local values it stayed running. PostgreSQL showed five connections as
`phoenix_auth`; [the health request](prior-image-health.txt) returned `OK` /
HTTP 200. [Sanitized startup logs](prior-image-startup.txt) are retained.

The [post-start checks](recovery-after-image-start.txt) still match every
pre-backup fingerprint and protection check. The local application and
database services were stopped after inspection. This establishes prior-image
startup and basic health against the restored synthetic database, not an
authenticated end-to-end school-day simulation or a frontend-image restore.
