---
status: accepted
---

# Meal ordering raises the Meal Plan compose budgets

Decision of 2026-10-18 under
[#3873](https://github.com/moto-nrw/project-phoenix/issues/3873). It raises
registers that [ADR 0057](0057-only-the-target-is-a-rule.md) point 5 lets only
fall.

## Context

#3873 gives a school its own meal ordering rules (registration model,
registration and cancellation deadlines, behaviour of late sick notes) and
lets parents cancel a whole week. The parent portal reaches the Meal Plan only
through its public module, and the public module reaches the application
layer only through the engine in `modules/mealplan/compose`. Every new
operation therefore needs one engine method that parses the dates, calls the
service, maps the errors and converts the result, and one binding method in
`workflows/parentportal/compose` that converts it into the flow's types.

Before raising anything, the change was cut down:

- The ordering settings reach the Meal Plan as one value read in one batch,
  not as thirteen resolvers.
- The parent's week change reuses the day change's command shape and error
  mapping; it adds one engine method and one binding method.
- A shared date-conversion helper in the binding was tried and costs one line
  more than the two loops it replaces, so it was dropped.

What remains in this change:

- `modules/mealplan/compose`: the week engine method, the date conversion of
  its result, the new fields of the participation plan and the settings value:
  27 lines.
- `workflows/parentportal/compose`: the week binding method and the new fields
  of the participation plan: 25 lines.

## Decision

1. `moduleComposeBudgets["modules/mealplan/compose"]` rises from 242 to 269.
2. `moduleComposeBudgets["workflows/parentportal/compose"]` rises from 450 to
   475.
3. `moduleComposeBudgetTotal` rises by the same 52 lines, from 31,146 to
   31,198.
4. `modulePassthroughOwnerBudgets["modules/mealplan"]` falls from 15 to 14 and
   the total with it: the rule methods replaced a delegation.
5. The raise covers exactly the methods named above. Every other number keeps
   falling under ADR 0057 point 5.

## Consequences

- The ordering rules have the same shape as the rest of the Meal Plan: the
  public module, the engine and the parent binding.
- If the engine later drops its per-method forwarding, these methods go with
  the rest and the budgets fall again.
