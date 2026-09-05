/**
 * Bedarfsquellen-Zuordnung des Betreuungsplan-Kopfzeilen-Chips.
 *
 * Reine Funktion ohne Datenabruf: Der Aufrufer lädt die Anmeldephasen
 * (`listPhases()`) und übergibt den sichtbaren Planungszeitraum plus das
 * sichtbare Datum. Die Y5-Kette verlangt, dass ablesbar ist, woher der Bedarf
 * stammt. Die Zuordnung ist bewusst clientseitig und lose gekoppelt: es gibt
 * keinen Endpunkt, der alle Planungszeiträume einer Phase auflistet (US-A2,
 * fehlt noch). Der Chip macht die lose Kopplung sichtbar, er behebt sie nicht.
 */

import type { Phase } from "~/lib/enrollment-phase-api";

/** Zielroute für den Nicht-eindeutig-/Nicht-verknüpft-Fall. */
const ENROLLMENT_PHASES_HREF = "/enrollment-phases";

export interface DemandOrigin {
  /** Chip-Text, z. B. "Bedarf: Anmeldung HJ1". */
  readonly label: string;
  /** Wenn gesetzt, verlinkt der Chip dorthin (Pflege der Zuordnung). */
  readonly href?: string;
}

const AMBIGUOUS: DemandOrigin = {
  label: "Bedarf: nicht eindeutig zugeordnet",
  href: ENROLLMENT_PHASES_HREF,
};

const NONE: DemandOrigin = {
  label: "Bedarf: keine Anmeldung verknüpft",
  href: ENROLLMENT_PHASES_HREF,
};

/**
 * Löst die Bedarfsquelle des sichtbaren Zeitraums auf:
 *
 * (a) Genau eine Phase mit `calendar_period_id` == sichtbarer Zeitraum →
 *     "Bedarf: {Phasenname}".
 * (b) Sonst Rückfall: genau eine Phase, deren Servicezeitraum
 *     (`service_start_date`..`service_end_date`) das sichtbare Datum enthält →
 *     deren Name.
 * (c) Mehrere Treffer (direkt oder im Rückfall) → "nicht eindeutig zugeordnet"
 *     mit Link auf `/enrollment-phases`.
 * (d) Keine Treffer → "keine Anmeldung verknüpft" mit Link auf
 *     `/enrollment-phases`.
 *
 * @param phases        Alle Anmeldephasen (`listPhases()`).
 * @param periodId      ID des sichtbaren Planungszeitraums, oder null.
 * @param visibleDateISO Sichtbares Datum als "YYYY-MM-DD".
 */
export function resolveDemandOrigin(
  phases: readonly Phase[],
  periodId: string | null,
  visibleDateISO: string,
): DemandOrigin {
  const direct = periodId
    ? phases.filter((phase) => phase.calendar_period_id === periodId)
    : [];
  if (direct.length === 1) {
    return { label: `Bedarf: ${direct[0]!.name}` };
  }
  if (direct.length > 1) {
    return AMBIGUOUS;
  }

  const dateMatches = phases.filter(
    (phase) =>
      phase.service_start_date <= visibleDateISO &&
      visibleDateISO <= phase.service_end_date,
  );
  if (dateMatches.length === 1) {
    return { label: `Bedarf: ${dateMatches[0]!.name}` };
  }
  if (dateMatches.length > 1) {
    return AMBIGUOUS;
  }

  return NONE;
}
