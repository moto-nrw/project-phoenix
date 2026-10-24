// Belegung je Betreuungsangebot (#4782): dieselben Spalten in der
// Angebotsliste und auf der Seite der Anmeldephase, damit beide dieselben
// Zahlen zeigen. Nur Summen, nie Namen von Kindern.

import {
  type DataTableColumn,
  DataTableStatusBadge,
} from "~/components/ui/data-table";
import { StatusBadge } from "~/components/ui/status-badge";
import type { CareOffering } from "~/lib/care-offering-api";
import {
  type CareOfferingBookingStats,
  type OfferingOccupancy,
  offeringOccupancy,
} from "~/lib/care-offering-booking-stats";

type StatsById = Readonly<Record<string, CareOfferingBookingStats>>;

/** „7 von 10“ und „3 frei“; ohne Kapazitätsgrenze nur die Belegung. */
function OfferingOccupancyCell({
  occupancy,
  isActive,
}: Readonly<{ occupancy: OfferingOccupancy | null; isActive: boolean }>) {
  // Ohne Belegung keine Zahl: „0“ wäre falsch.
  if (!occupancy) return <span className="text-sm text-gray-500">–</span>;
  if (occupancy.capacity == null) {
    return <span className="text-sm text-gray-700">{occupancy.booked}</span>;
  }
  return (
    <div className="whitespace-nowrap">
      <p className="text-sm text-gray-900">
        {occupancy.booked} von {occupancy.capacity}
      </p>
      {isActive ? (
        <p className="mt-0.5 text-xs text-gray-500">
          {occupancy.full ? "Keine frei" : `${occupancy.free} frei`}
        </p>
      ) : null}
    </div>
  );
}

/** Spalte „Kapazität“: „Unbegrenzt“, „1 Platz“, „12 Plätze“. */
function formatOfferingCapacity(capacity: number | null | undefined): string {
  if (capacity == null) return "Unbegrenzt";
  return capacity === 1 ? "1 Platz" : `${capacity} Plätze`;
}

export function offeringCapacityColumn(): DataTableColumn<CareOffering> {
  return {
    key: "capacity",
    header: "Kapazität",
    sortValue: (offering) => offering.capacity ?? Number.POSITIVE_INFINITY,
    render: (offering) => (
      <span className="text-sm text-gray-700">
        {formatOfferingCapacity(offering.capacity)}
      </span>
    ),
  };
}

export function offeringOccupancyColumn(
  stats: StatsById,
): DataTableColumn<CareOffering> {
  return {
    key: "occupancy",
    header: "Belegt",
    sortValue: (offering) => stats[offering.id]?.booked ?? null,
    render: (offering) => (
      <OfferingOccupancyCell
        occupancy={offeringOccupancy(stats[offering.id])}
        isActive={offering.is_active}
      />
    ),
  };
}

/** Aktiv oder Inaktiv, dazu „Voll“ für ein Angebot ohne freien Platz. */
export function offeringStatusColumn(
  stats: StatsById,
): DataTableColumn<CareOffering> {
  return {
    key: "status",
    header: "Status",
    sortValue: (offering) => (offering.is_active ? 1 : 0),
    render: (offering) => (
      <div className="flex flex-wrap items-center gap-1.5">
        <DataTableStatusBadge active={offering.is_active} />
        {offering.is_active && offeringOccupancy(stats[offering.id])?.full ? (
          <StatusBadge
            label="Voll"
            tone="orange"
            title="Alle Plätze sind belegt."
          />
        ) : null}
      </div>
    ),
  };
}
