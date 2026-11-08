"use client";

import type { DashboardPickupBucket } from "~/lib/display-api";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { InsetPanel } from "~/components/ui/inset-panel";
import { SectionCard } from "~/components/ui/section-card";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import { pickupConcept } from "~/lib/concepts/pickup";

interface PickupTimesPanelProps {
  readonly buckets: DashboardPickupBucket[];
}

/**
 * Aggregated pickup times ("15:00 Uhr — 12 Kinder"). Deliberately counts
 * only: per-child pickup times never appear on a publicly visible screen.
 */
export function PickupTimesPanel({ buckets }: PickupTimesPanelProps) {
  return (
    <SectionCard>
      <div className="mb-5 flex items-center gap-4">
        <ConceptIconTile concept={pickupConcept} size="display" />
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
            <InsetPanel
              as="li"
              key={bucket.time}
              tone="white"
              padding="md"
              className="flex items-center justify-between"
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
            </InsetPanel>
          ))}
        </ul>
      )}
    </SectionCard>
  );
}
