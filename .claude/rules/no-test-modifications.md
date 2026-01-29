# Never Modify Existing Tests to Make Them Pass

**ABSOLUTE RULE: Claude MUST NEVER change existing tests to fix a test failure caused by new or modified code.**

Existing tests encode verified business rules and expected behavior. When new code breaks existing tests, the tests are almost certainly correct and the new code is wrong.

## What to do when tests fail after code changes

1. **STOP immediately** — do not touch the test file
2. **Investigate the failure** — understand what the test expects and why the new code violates that expectation
3. **Explain the conflict to the user** in plain language:
   - What the test expects (the existing business rule)
   - What the new code does differently
   - Why these two things conflict
   - Give a concrete example of the business scenario if possible
4. **Present two options clearly:**
   - **Option A: Fix the new code** to comply with the existing business rule (the test stays as-is)
   - **Option B: Update the test** because the business rule intentionally changed (requires explicit user approval)
5. **Wait for the user's decision** — do NOT proceed until the user explicitly chooses

## Why this rule exists

- Tests represent validated business decisions made by humans
- Silently changing tests to match new code **hides bugs** behind green checkmarks
- A failing test is a signal that new code may violate a business invariant
- Only a human can decide whether a business rule should change

## Examples

### WRONG — Changing the test to match new code
```
Test expects: HTTP 500 when student has no attendance record
New code returns: HTTP 200
Claude thinks: "The test is outdated, let me update it to expect 200"
```
This is FORBIDDEN. The test encodes a business rule: students without attendance records cannot check out.

### CORRECT — Surfacing the conflict to the user
```
"The test TestDailyCheckoutNoActiveVisit expects a 500 error, but the new
handleDailyCheckout code returns 200 success. This means:

- The existing rule says: students need an attendance record before checkout
- The new code says: checkout succeeds even without an attendance record

Which behavior is correct? Should I:
  A) Fix the handler to reject checkout without attendance (keep the test)?
  B) Update the test because the business rule changed (you confirm)?"
```

## Scope

This rule applies to:
- Unit tests
- Integration tests
- API tests (Bruno)
- Any test file that existed before the current task began

This rule does NOT prevent:
- Writing new tests for new functionality
- Adding test cases to existing test files (new `func Test...`)
- Fixing tests that are genuinely broken (e.g., wrong import path after a refactor) — but even then, explain what you are doing and why
