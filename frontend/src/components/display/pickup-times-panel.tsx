"use client";

import type { DashboardPickupBucket } from "~/lib/display-api";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";

interface PickupTimesPanelProps {
  readonly buckets: DashboardPickupBucket[];
}

/**
 * Aggregated pickup times ("15:00 Uhr — 12 Kinder"). Deliberately counts
 * only: per-child pickup times never appear on a publicly visible screen.
 */
export function PickupTimesPanel({ buckets }: PickupTimesPanelProps) {
  return (
    <section className="moto-content-surface rounded-2xl border p-6 shadow-sm lg:p-8">
      <div className="mb-5 flex items-center gap-4">
        <ConceptIconTile concept="pickup" variant="display" />
        <h2 className="text-3xl font-bold text-gray-900">
          Nächste Abholzeiten
        </h2>
      </div>

      {buckets.length === 0 ? (
        <p className="py-8 text-center text-2xl text-gray-400">
          Heute keine weiteren Abholzeiten.
        </p>
      ) : (
        <ul className="space-y-2">
          {buckets.map((bucket) => (
            <li
              key={bucket.time}
              className="flex items-center justify-between rounded-2xl border border-gray-200 bg-white p-4"
            >
              <p
                className="text-3xl font-bold tabular-nums"
                style={{ color: LOCATION_COLORS.DANGER }}
              >
                {bucket.time} Uhr
              </p>
              <p className="text-2xl text-gray-700">
                <span className="font-bold">{bucket.count}</span>{" "}
                {bucket.count === 1 ? "Kind" : "Kinder"}
              </p>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
