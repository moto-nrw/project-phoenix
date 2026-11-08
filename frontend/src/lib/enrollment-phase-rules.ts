import type { Phase } from "~/lib/enrollment-phase-api";

/**
 * Whether the untokenized /anmeldung/{id} form is reachable. The anonymous
 * form gate refuses linked_parents and existing_students, so a plain link to
 * such a phase 404s; those parents use a late-invite token or the parents
 * portal (#1663).
 */
export function phaseHasPublicForm(phase: Pick<Phase, "audience">): boolean {
  return (
    phase.audience !== "linked_parents" &&
    phase.audience !== "existing_students"
  );
}
