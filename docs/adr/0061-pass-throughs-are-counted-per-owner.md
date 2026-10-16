---
status: accepted
---

# Pass-throughs are counted per owner

Decision of 2026-10-16 under
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580), for
[#4084](https://github.com/moto-nrw/project-phoenix/issues/4084).

## Context

`TestModulePassthroughRatchet` counted single-call delegations in two
shrink-only registers: one per module for the application layers, and one
per composition since #4071. #4084 moves domain logic out of the
compositions into the application layers. A method that moves keeps its
shape, so the composition entry falls and the application entry rises. Each
move would fail the ratchet and need a new ADR, although no delegation was
added.

## Decision

1. One register, `modulePassthroughOwnerBudgets`, counts the application
   layer and the compositions of a module directory together. It replaces
   `modulePassthroughBudgets` and `modulePassthroughComposeBudgets`, and
   `modulePassthroughOwnerBudgetTotal` replaces their two totals.
2. Each entry starts as exactly the sum of the module's two former entries,
   without headroom: 3,109 in total, the former 717 and 2,392.
3. The guard stays: an entry and the total may only fall. A method moved
   between the application layer and a composition of the same module leaves
   the entry unchanged. A method moved into another module raises that
   module's entry and fails like any other growth.

## Consequences

- #4084 can move code within an owner without raising a budget.
- The register no longer tells how much of an owner's forwarding sits in its
  composition. The compose LOC budget of #4071 still measures that.
