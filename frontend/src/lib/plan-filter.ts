// Gruppen- und Personenfilter des Betreuungsplans (#4766). Beide Werte reisen
// als URL-Parameter `group` und `staff` (wie `staff` im Dienstplan), damit
// eine gefilterte Ansicht teilbar ist. Der Backend-Endpunkt filtert; diese
// Datei übersetzt nur zwischen URL, Anfrage und Cache-Schlüssel.

/** Aktiver Filter; `null` heißt: nicht gefiltert. */
export interface PlanFilter {
  readonly groupId: string | null;
  readonly staffId: string | null;
}

export const NO_PLAN_FILTER: PlanFilter = { groupId: null, staffId: null };

const maxInt64 = "9223372036854775807";

/**
 * Liest eine Filter-ID aus der URL. Nur positive ganze Zahlen zählen; alles
 * andere fällt still auf „nicht gefiltert“ zurück, wie ein ungültiges `d`.
 */
export function parsePlanFilterId(
  raw: string | null | undefined,
): string | null {
  if (
    raw == null ||
    !/^[1-9]\d*$/.test(raw) ||
    raw.length > maxInt64.length ||
    (raw.length === maxInt64.length && raw > maxInt64)
  ) {
    return null;
  }
  return raw;
}

export function isPlanFilterActive(filter: PlanFilter): boolean {
  return filter.groupId !== null || filter.staffId !== null;
}

/** Query-Parameter für GET /api/timetable/instances. */
export function planFilterQuery(filter: PlanFilter): Record<string, string> {
  return {
    ...(filter.groupId ? { group_id: filter.groupId } : {}),
    ...(filter.staffId ? { staff_id: filter.staffId } : {}),
  };
}

/**
 * Teil des SWR-Schlüssels: ohne Filter leer, damit der ungefilterte Plan
 * denselben Schlüssel behält wie bisher.
 */
export function planFilterKey(filter: PlanFilter): string {
  if (!isPlanFilterActive(filter)) return "";
  return `-g${filter.groupId ?? ""}-p${filter.staffId ?? ""}`;
}
