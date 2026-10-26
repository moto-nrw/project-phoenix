---
status: accepted
---

# Scan heartbeat isolation uses the existing savepoint runtime

PR #5040 promises that a failed optional session heartbeat cannot discard a
successful scan. PostgreSQL statement errors abort the request transaction;
logging alone does not recover it. The device-scan workflow therefore runs
its heartbeat lookup and update in the existing tenant runtime's savepoint,
through its consumer-owned unit-of-work port. Ordinary operation errors roll
back to that savepoint; savepoint-control failures fail the request.

Under ADR 0070 point 4, raise only these `backend/test/query_budgets.go`
entries: `api.iot.checkin.check_in` 36 → 38,
`api.iot.checkin.check_out` 24 → 26, and
`api.iot.checkin.room_change` 42 → 44. The added statements are exactly one
`SAVEPOINT` and one `RELEASE SAVEPOINT` per successful scan. The existing tests
still count every statement from BEGIN through COMMIT and require flat counts
with three and eight children. Failure-path regression tests verify that real
PostgreSQL lookup and update errors leave the preceding attendance or visit
write committed.

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
