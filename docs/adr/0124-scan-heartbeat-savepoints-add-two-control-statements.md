---
status: accepted
---

# Scan heartbeat isolation uses the existing savepoint runtime

PR #5040 promises that a failed optional session heartbeat cannot discard a
successful scan. PostgreSQL statement errors abort the request transaction;
logging alone does not recover it. The device-scan workflow therefore runs
every heartbeat statement in one savepoint of the existing tenant runtime,
through its consumer-owned unit-of-work port: the lookup of the device's
session in the scanned room, the lookup of its running session and the
update. No heartbeat lookup runs before the savepoint, because a failed
statement there would abort the transaction the savepoint needs. Ordinary operation errors roll
back to that savepoint; savepoint-control failures fail the request.

The `backend/test/query_budgets.go` entries this PR adds therefore include
the savepoint: `api.iot.checkin.check_in` 38, `api.iot.checkin.check_out` 26
and `api.iot.checkin.room_change` 44, each exactly two statements above the
count without it (36, 24, 42). The two statements are one `SAVEPOINT` and one
`RELEASE SAVEPOINT` per successful scan. The existing tests
still count every statement from BEGIN through COMMIT and require flat counts
with three and eight children. Failure-path regression tests verify that real
PostgreSQL lookup and update errors, including an error in the room lookup of
a detailed checkout, leave the preceding attendance or visit write committed.

The runtime binding also adds exactly nine production lines in
`modules/devicescan/compose`: the `errors` import and `unitOfWork.BestEffort`,
which maps only savepoint-control failures back to the workflow. In
`backend/test/module_legacy_budget_ratchet_test.go`, raise
`moduleComposeBudgets["modules/devicescan/compose"]` from 531 to 540 and
`moduleComposeBudgetTotal` from 12,647 to 12,656. Keeping this adapter in the
composition preserves the application's consumer-owned transaction port;
importing the tenant runtime directly would bypass that seam. No unrelated
composition, data-query count, pass-through or other budget changes.

An independent heartbeat transaction would break the same-request transaction
contract. Omitting savepoint statements from measurement would hide actual
work, and unrelated query optimizations would broaden this correctness fix.
