/**
 * Legende der Planungsspuren für die Werkzeugflächen mit Wochenraster
 * (Betreuungsplan, Vertretung).
 *
 * Bauart 3, Regel 3: jede Fläche mit Farbcodierung trägt eine `PlanLegend`.
 * Damit Betreuungsplan und Vertretung dieselben Farben mit denselben Wörtern
 * erklären, liegt die Ableitung hier und nicht in einer der beiden Ansichten.
 */

import type { PlanLegendEntry } from "~/components/ui/plan-legend";
import type { EnrichedInstance } from "~/lib/timetable-types";

export function buildPlanningTrackLegend(
  instances: readonly EnrichedInstance[],
): PlanLegendEntry[] {
  const used = new Map<string, PlanLegendEntry & { sortOrder: number }>();
  let hasUnassigned = false;
  for (const instance of instances) {
    if (!instance.planningTrackId || !instance.planningTrackName) {
      hasUnassigned = true;
      continue;
    }
    used.set(instance.planningTrackId, {
      key: instance.planningTrackId,
      label: instance.planningTrackName,
      color: instance.planningTrackColor,
      sortOrder: instance.planningTrackSortOrder ?? Number.MAX_SAFE_INTEGER,
    });
  }
  const entries = [...used.values()]
    .sort(
      (left, right) =>
        left.sortOrder - right.sortOrder ||
        left.label.localeCompare(right.label, "de"),
    )
    .map(({ sortOrder: _sortOrder, ...entry }) => entry);
  if (hasUnassigned) {
    entries.push({ key: "unassigned", label: "Ohne Planungsspur" });
  }
  return entries;
}
