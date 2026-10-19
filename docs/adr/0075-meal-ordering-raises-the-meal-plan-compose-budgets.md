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
registration and cancellation deadlines, behaviour of late sick notes), lets
parents cancel a whole week, and lets the school calendar close days and
suspend the weekly pattern in the holidays. The school's list of a day also
names the children who do not eat, so staff can register them late. The
orders of a day or a week are fixed at the deadline and later changes are
numbered; school, caterer and exports read them through one engine method. The parent portal reaches the Meal Plan only
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
- The calendar adapter only forwards the window and the timing; the
  conversion of the day sets lives in the domain (`NewCalendarDays`).
- The orders reach the public module through one engine method. Naming and
  sorting the children happens in the application layer, so the engine only
  copies fields; a child is converted as a whole struct.
- Whether the school knows a child and whether it is in care on a day is
  decided in the application layer from the two directory lookups the
  engine already binds, not by a third adapter method.
- The error mapping of the engine is a table instead of one branch per error.

What remains in this change, after development moved the engine of the
Meal Plan from `modules/mealplan/compose` into its application layer and
measured the registers again:

- `modules/mealplan/compose`: the school calendar and the names of past
  orders as dependencies, and the ordering settings as one value: 5 lines.
- `workflows/parentportal/compose`: the week binding method and the new fields
  of the participation plan: 25 lines.

## Decision

1. `moduleComposeBudgets["modules/mealplan/compose"]` rises from 84 to 89.
2. `moduleComposeBudgets["workflows/parentportal/compose"]` rises from 450 to
   475.
3. `moduleComposeBudgetTotal` rises by the same 30 lines, from 20,119 to
   20,149.
4. `modulePassthroughOwnerBudgets["modules/mealplan"]` stays at 15 and the
   total with it: the rule methods replaced two delegations, and the engine
   methods of the orders and of their settling add one each.
5. The raise covers exactly the methods named above. Every other number keeps
   falling under ADR 0057 point 5.

## Consequences

- The ordering rules have the same shape as the rest of the Meal Plan: the
  public module, the engine and the parent binding.
- If the engine later drops its per-method forwarding, these methods go with
  the rest and the budgets fall again.
