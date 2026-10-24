// Belegung je Betreuungsangebot (#4782): dieselbe Zelle und derselbe Status
// in der Angebotsliste und auf der Seite der Anmeldephase. Nur Summen, nie
// Namen von Kindern.

import { StatusBadge } from "~/components/ui/status-badge";
import type { OfferingOccupancy } from "~/lib/care-offering-booking-stats";

/** „7 von 10“ und „3 frei“; ohne Kapazitätsgrenze nur die Belegung. */
export function OfferingOccupancyCell({
  occupancy,
}: Readonly<{ occupancy: OfferingOccupancy | null }>) {
  // Ohne geladene Belegung keine Zahl: „0“ wäre falsch.
  if (!occupancy) return null;
  if (occupancy.capacity == null) {
    return <span className="text-sm text-gray-700">{occupancy.booked}</span>;
  }
  return (
    <div className="min-w-0">
      <p className="text-sm text-gray-900">
        {occupancy.booked} von {occupancy.capacity}
      </p>
      <p className="mt-0.5 text-xs text-gray-500">
        {occupancy.full ? "Keine frei" : `${occupancy.free} frei`}
      </p>
    </div>
  );
}

/** Status „Voll“ für ein Angebot ohne freien Platz. */
export function OfferingFullBadge({
  occupancy,
}: Readonly<{ occupancy: OfferingOccupancy | null }>) {
  if (!occupancy?.full) return null;
  return (
    <StatusBadge label="Voll" tone="orange" title="Alle Plätze sind belegt." />
  );
}

/** Sortierwert der Spalte „Belegt“. */
export function occupancySortValue(
  occupancy: OfferingOccupancy | null,
): number | null {
  return occupancy?.booked ?? null;
}
