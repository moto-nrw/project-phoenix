---
paths:
  - "backend/**/*_test.go"
  - "backend/test/**"
  - "frontend/**/*.test.*"
  - "frontend/**/*.spec.*"
  - "frontend/src/test/**"
  - "scripts/**"
---

# Preserve test contracts

A failing test is evidence to investigate, not an instruction to change its
expected result. Do not weaken assertions, skip tests, or grow allowlists to
hide a regression.

## When tests fail

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
Report failed or unavailable checks accurately.
