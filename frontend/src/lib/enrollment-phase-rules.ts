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

/** The enrollment window closes after it opens (`phase.go` checks the same). */
export function enrollmentWindowIsOrdered(
  openAt: string | null | undefined,
  closeAt: string | null | undefined,
): boolean {
  return !openAt || !closeAt || new Date(closeAt) > new Date(openAt);
}
