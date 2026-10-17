---
status: accepted
---

# Moved code stays in the pass-through scan

Decision of 2026-10-17 under
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580), for
[#4084](https://github.com/moto-nrw/project-phoenix/issues/4084). It raises a
register that [ADR 0057](0057-only-the-target-is-a-rule.md) point 5 lets only
fall.

## Context

`TestModulePassthroughRatchet` counts single-call delegations in the
application layers and the compositions of each owner
([ADR 0061](0061-pass-throughs-are-counted-per-owner.md)). #4084 moves code
out of the compositions. Two of those moves lowered the Workforce entry
without removing a single delegation:

1. #4239 moved the time-tracking row adapters into
   `internal/application/timetracking` and gave them an error-shape field.
   Each method that called the Workforce capability and built its error
   through that field now made two field calls, so 46 of them no longer met
   condition 4 of the ratchet. They still forward one capability call each;
   the error shape is formatting, not a second collaborator. The entry fell
   from 400 to 354.
2. #4240 moved the 18 Dienstplan row delegations into
   `modules/workforce/internal/planning`, which has the application role but
   lies outside `internal/application/`, so the scan does not reach it. The
   entry fell from 354 to 336.

The same kind of gap exists for the owners' ports packages: #4174, #4182 and
#4227 moved two error types with a forwarding `Error()` method into
`modules/timetable/ports` and `modules/peopledirectory/ports`.

## Decision

1. The row adapters build their error shape through package functions again,
   as they did in the composition; no row adapter holds an error field.
2. The scan also covers the owner packages #4084 moved code into:
   `modules/workforce/internal/planning` and the ports packages of Care Plan,
   Identity & Access, People Directory and Timetable
   (`modulePassthroughMovedTrees`). A later move into another such package
   adds it to the list in the same change.
3. The entries take the measured values: Workforce 336 to 414 (the 46 hidden
   row delegations and planning's 32, of which 18 are the moved rows and 14
   planning methods the scan never reached), Timetable 490 to 491, People
   Directory 307 to 308. `modulePassthroughOwnerBudgetTotal` rises from 3,011
   to 3,091. Every number keeps falling from there.

## Consequences

- A move inside an owner can no longer lower its entry; only a removed or
  deepened delegation can.
- The rest of the unscanned owner code (public facades, HTTP adapters,
  postgres adapters) stays outside the register. Widening the scan to it is a
  separate decision.
