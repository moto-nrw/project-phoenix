// Kinderzahl einer laufenden Aktivität als Abzeichen, mit Grenze „66 / 45
// Kinder" (#3634). Über der Grenze steht daneben ein rotes „Überbucht": der
// Zustand ist Text, nicht nur Farbe. Den ausführlichen Hinweis trägt das
// Abzeichen als Tooltip; wo Platz ist, zeigt ihn die Fläche selbst an.
import {
  formatChildCount,
  OVERBOOKED_LABEL,
  overbookedHintFor,
  type Occupancy,
} from "~/lib/activity-occupancy";

import { StatusBadge } from "./status-badge";

interface OccupancyBadgeProps {
  readonly occupancy: Occupancy;
  /** Ob die Schule Tablets nutzt; steuert den Satz zum Tablet im Hinweis. */
  readonly nfcEnabled: boolean;
  readonly compact?: boolean;
}

/** Rotes „Überbucht" mit dem Hinweis als Tooltip; nichts bis zur Grenze. */
export function OverbookedBadge({
  occupancy,
  nfcEnabled,
  compact = false,
}: OccupancyBadgeProps) {
  const hint = overbookedHintFor(occupancy, nfcEnabled);
  if (hint === null) return null;
  return (
    <StatusBadge
      label={OVERBOOKED_LABEL}
      tone="red"
      title={hint}
      compact={compact}
    />
  );
}

/** „66 / 45 Kinder" (ohne Grenze „66 Kinder") und darüber „Überbucht". */
export function OccupancyBadges({
  occupancy,
  nfcEnabled,
  compact = false,
}: OccupancyBadgeProps) {
  return (
    <>
      <StatusBadge
        label={formatChildCount(occupancy.count, occupancy.limit)}
        tone="gray"
        compact={compact}
      />
      <OverbookedBadge
        occupancy={occupancy}
        nfcEnabled={nfcEnabled}
        compact={compact}
      />
    </>
  );
}
