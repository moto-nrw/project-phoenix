import type { SelectAllControl } from "~/components/students/school-checkin-select-all";

/**
 * Was ein Klick im An- und Abmelde-Modus tut (#6766). Leiste am Desktop,
 * Leiste am Handy und runder Knopf nutzen dieselben Sätze.
 */
export const CHECKIN_HINTS = {
  start: "Startet den Modus, in dem jeder Tipp ein Kind an- oder abmeldet.",
  finish: "Beendet den Modus, die bisherigen An- und Abmeldungen bleiben.",
  selectAll: "Wählt alle angezeigten Kinder aus, gemeldet wird noch nichts.",
  clearSelection: "Hebt die Auswahl auf, an den Kindern ändert sich nichts.",
  checkIn: "Meldet die ausgewählten Kinder in der Schule an.",
  checkOut:
    "Meldet die ausgewählten Kinder ab, bei Raumbesuchen fragt moto vorher nach.",
  noneSelected: "Wählen Sie zuerst mindestens ein Kind aus.",
} as const;

/** Warum „Alle N auswählen“ gerade nicht geht, sonst `undefined`. */
export function selectAllBlockedReason(
  control: Pick<SelectAllControl, "count" | "allSelected">,
): string | undefined {
  if (control.count === 0) return "In der Liste steht gerade kein Kind.";
  if (control.allSelected)
    return "Alle angezeigten Kinder sind schon ausgewählt.";
  return undefined;
}
