---
status: accepted
---

# Principle: a test reaches its owner, the typed roots and the shared kernel

Principle ADR of 2026-10-19 under
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580), written for
[#4091](https://github.com/moto-nrw/project-phoenix/issues/4091). It states the
test boundaries and the shared-kernel rules that still hold. The ADRs it cites
keep their context and history; this ADR changes none of them.

## Decision

1. **Test roles name the seam, and test scopes are separate.** Every package
   declares a production role and both test roles (`module-internal-test`,
   `module-behavior-test`, `workflow-decision-test`,
   `workflow-integration-test`, `adapter-test` or `e2e-test`). Production
   permissions do not leak into test scopes, and a test declaration cannot
   authorize production growth.
2. **A test may use the infrastructure it verifies.** Every test role may
   import the ORM, the HTTP router and the test framework directly, through
   owner-agnostic `external.<class>.<role>` rules. This grants no other
   external class and no first-party target
   ([ADR 0014](0014-test-roles-may-import-test-infrastructure.md)).
3. **A test of one owner uses that owner.** Its test packages may import the
   owner's `public`, `contract`, `compose`, `port`, `adapter` and
   `test-support` roles; the owner's test support may import its own
   `public`, `contract`, `compose`, `port` and `adapter`. Other owners appear
   as fakes of consumer-owned ports or as rows from the shared fixtures. Their
   implementation roles stay forbidden
   ([ADR 0053](0053-owners-compose-themselves-and-the-roots-compose-the-owners.md)
   point 7.1).
4. **A test of several owners builds the typed root.** Every test package may
   import `root-composition`'s `compose` role. API tests build route-sized
   routers or the typed roots; there is no shared test service graph
   (ADR 0053 point 7.2,
   [ADR 0059](0059-the-roots-hold-no-service-graph.md) point 5).
5. **The shared fixtures create rows; they do not stand in for owners.** The
   fixture owner `test-support` (`test`, `api/testutil`, `internal/testdb`,
   the end-to-end suites) maps the rows it writes in its own types and reaches
   other owners only through their surfaces. Its standing targets are its own
   tooling, `tenant-runtime/public`, `security-runtime/contract` and
   `public`, `settings-platform/public`, the Transaction Runtime bootstrap
   (`transaction-runtime/compose`, from the fixture package's `test-support`
   role in production only) and the session token runtime
   ([ADR 0039](0039-shared-fixtures-reach-their-standing-targets.md),
   [ADR 0047](0047-shared-fixtures-reach-the-security-and-settings-contracts.md)
   with its addenda, ADR 0053 point 7.3). Anything else the fixtures import
   from an implementation role is exact debt, never a rule. `api/testutil`
   keeps only owner-agnostic HTTP helpers (ADR 0053 point 7.4).
6. **A test that drives another owner's implementation moves to that owner's
   suite**, keeping its assertions (ADR 0047).
7. **Every module's public contract has a named contract test.**
   `backend/test/module_contract_inventory.json` pins each of the 18 domain
   and 10 platform modules
   ([ADR 0066](0066-principle-every-module-table-and-cross-owner-read-has-one-owner.md))
   to its contract package and an exact test function, 28 of 28.
   `TestModuleContractInventoryRatchet` checks the signatures and the
   observable assertions and rejects empty, success-only and
   method-presence-only tests; a missing entry has a zero ceiling
   ([#4090](https://github.com/moto-nrw/project-phoenix/issues/4090)). An entry
   proves a named contract test, not exhaustive API coverage.
8. **Every role may import the shared kernel.** `sharedkernel/calendar` is
   `shared-kernel`/`contract` and holds the closed list `Date`, `WallClock`,
   `TenantID` and `CorrelationID`. One owner-agnostic rule,
   `shared-kernel.contract`, admits it in all three scopes. The kernel gains no
   type and no other role, and a `same_owner` rule never selects it
   ([ADR 0040](0040-every-role-may-import-the-shared-kernel.md)). The
   `internal/timezone` wrapper is deleted
   ([#4081](https://github.com/moto-nrw/project-phoenix/issues/4081)).

## Consequences

- An owner can be tested on its own: its suites need its own roles, the
  kernel, test infrastructure and fakes of its ports.
- How a reviewed epoch accepts the finite test and fixture rule shapes above is
  the procedure of
  [ADR 0070](0070-principle-only-the-target-is-a-rule-and-debt-leaves-with-evidence.md).

## References

ADR 0014, ADR 0039, ADR 0040, ADR 0047 and ADR 0053 point 7 carry the
decisions; ADR 0059 point 5 the test side of the typed roots. The full rule map
is the
[traceability appendix](../../backend/architecture/README.md#appendix-principle-traceability).
