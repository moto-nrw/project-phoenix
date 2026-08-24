// Tagesstatus-Beschriftung der Klassenansicht — eine Quelle für die
// Detaillisten und den Abweichungsblock (#2294). Zwei Stellen mit eigenen
// Wörtern für denselben Status wären genau der Missverständnis-Fall, den
// .claude/rules/verstaendlichkeit.md ausschließt.

import { LOCATION_COLORS } from "~/lib/location-helper";

export const STATUS_LABELS: Record<string, string> = {
  sick: "Krank",
  excused: "Entschuldigt",
  class_trip: "Klassenfahrt",
  // Abhol-Ausnahme ohne Zeit: die Betreuung für diesen Tag wurde abgesagt.
  cancelled: "Heute abgemeldet",
};

// Status colors come from the brand table, never as re-typed hexes:
// SICK amber, CLASS_TRIP blue, EXCUSED purple, CANCELLED neutral gray.
export const STATUS_COLORS: Record<string, string> = {
  sick: LOCATION_COLORS.SICK,
  class_trip: LOCATION_COLORS.CLASS_TRIP,
  excused: LOCATION_COLORS.EXCUSED,
  cancelled: LOCATION_COLORS.UNKNOWN,
};

export function statusLabel(status: string): string {
  return STATUS_LABELS[status] ?? status;
}

export function statusColor(status: string): string {
  return STATUS_COLORS[status] ?? LOCATION_COLORS.UNKNOWN;
}
