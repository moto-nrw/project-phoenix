/**
 * Namen der Einstellungen einer Anmeldephase. Die Anmeldephasen der Schule
 * und die Anmeldephasen des Trägers (#4704) zeigen dieselben Wörter.
 */
import type {
  CareOfferingSelectionMode,
  PhaseAudience,
  PhaseCareOverflowMode,
  PhaseKind,
} from "./enrollment-phase-api";

export const PHASE_KIND_LABELS: Record<PhaseKind, string> = {
  school_year: "Schuljahr",
  holiday: "Ferienbetreuung",
  custom: "Sonstiges",
};

export const PHASE_OVERFLOW_LABELS: Record<PhaseCareOverflowMode, string> = {
  waitlist: "Auf Warteliste setzen",
  reject: "Anmeldung blockieren",
  allow: "Ohne Hinweis akzeptieren",
};

export const PHASE_CARE_SELECTION_LABELS: Record<
  CareOfferingSelectionMode,
  string
> = {
  optional: "Optional",
  at_least_one: "Mindestens ein Angebot",
  exactly_one: "Genau ein Angebot",
};

export const PHASE_AUDIENCE_LABELS: Record<PhaseAudience, string> = {
  open: "Offen für alle",
  new_students: "Nur neue Kinder",
  existing_students: "Nur bereits angemeldete Kinder",
  linked_parents: "Nur Eltern mit Konto an der Schule",
};
