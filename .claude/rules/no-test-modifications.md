---
paths:
  - "backend/**/*_test.go"
  - "backend/test/**"
  - "frontend/**/*.test.*"
  - "frontend/**/*.spec.*"
  - "frontend/src/test/**"
  - "scripts/**"
---

# Changing tests

Tests are code you may reshape: restructure, speed up, merge, parallelize,
move to a cheaper level, thin a matrix, change fixtures, or delete a case
whose behavior another test already asserts. No approval is needed; the PR
names what changed and which tests still assert each behavior.

The **contract** is what a test asserts: the behavior it guarantees. Change a
contract only when the issue or the user asks for that behavior change.
Weakening an assertion, skipping a test, or growing an allowlist to turn a red
test green changes the contract and hides the defect.

## When tests fail

A failing test is evidence to investigate, not an instruction to change its
expected result.

1. Trace the failure to the behavior and compare it with the task's agreed
   requirements. Check the implementation and the test; either can be wrong.
2. If implementation violates an unchanged contract, fix the implementation
   and rerun the relevant tests. No additional approval is needed.
3. If the user already requested the behavior change, update the affected
   expectations to that requirement and explain the old/new contract. Preserve
   failure-mode coverage; a green result alone does not justify a test change.
4. If the business rule is genuinely unresolved, present the conflicting
   expectations and a concrete scenario. Ask for the missing decision before
   changing that contract; continue independent work meanwhile.
5. Mechanical repairs (imports, renamed symbols, fixtures) may proceed when
   behavior and assertion strength stay intact. Explain what changed and why.

This applies to unit and integration tests. Add new regression cases when they
verify the failure; keep backend fixture isolation and all existing ratchets.
Slow tests follow [ci-test-performance](ci-test-performance.md). Report failed
or unavailable checks accurately.
