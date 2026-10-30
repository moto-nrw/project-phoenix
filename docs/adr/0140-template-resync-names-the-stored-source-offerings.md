---
status: accepted
---

# Template resync names the stored source offerings

Issue [#5313](https://github.com/moto-nrw/project-phoenix/issues/5313) lets a
school save a Regeltermin whose stored source offering turned inactive: the
inactive source contributes nothing from today on, as on every resync since
[#5305](https://github.com/moto-nrw/project-phoenix/issues/5305), while a
newly selected inactive offering stays refused. Care Plan's resync can only
tell the two apart when it knows which source ids the template stored before
the save. Timetable writes the new ids before the resync, so it has to pass
the old ones along.

## Decision

1. `timetable.OfferingRosterResyncInput` and `careplan.OfferingRosterResync`
   carry `StoredOfferingIDs`, the template's source ids before the save. The
   template update and the split ("Dieser und alle folgenden") set them; create,
   connect and the template end leave them empty and stay strict. A split with
   a future cut also resyncs the capped predecessor from today with its stored
   ids when the guard refuses them as a fresh selection, so an inactive
   source leaves before the cut as on every save; a predecessor whose stored
   sources still pass, or are refused even with them, keeps its rows. The
   root's adapter `timetableOfferingRosterResync` forwards the field; it
   decides nothing.
2. Care Plan keeps the rule: an inactive offering among the stored ids is
   skipped while another source stays valid; every other refusal, a newly
   selected inactive offering and a template without a valid source keep
   their refusal. The pre-write guard `ValidateTemplateOfferingSource`
   applies the same rule to the ids it already receives.
3. The changed closures keep their classification in
   `backend/architecture/command-effects.json`: the two
   `ResyncTemplateOfferingRoster` targets stay `debt` (the coordination
   review ADR 0098 left open), the function-valued `ResyncOfferingRoster`
   boundaries stay `unresolved`. This ADR is their approval; a changed
   closure needs a new one.

## Consequences

No new port, dependency, policy rule, baseline or ratchet budget. The
protocol review of the template resync stays open as before; moving the
coordination into a workflow later moves the stored ids along.
