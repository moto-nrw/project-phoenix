// The chrome every care-offering row shares. Extracted so the selectable rows
// in the parent/manual enrollment form and the blocked rows in the admin
// adjustment dialog cannot drift apart (#2186 review) — the same reason
// FeaturePill lives beside this file.
//
// A selectable row is the kit's `ChoiceTile` (`as="div"`: the checkbox inside
// drives the selection); a row the child cannot pick is a dashed
// `InsetPanel`, the kit's look for something that is not available (#6137).
// It stays in components/enrollment/ rather than components/ui/ because "an
// offering row" is enrollment vocabulary, not a general-purpose primitive.

import type { ReactNode } from "react";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { InsetPanel } from "~/components/ui/inset-panel";
import {
  type CareOfferingBookingStats,
  formatOfferingOccupancy,
  offeringIsFull,
} from "~/lib/care-offering-booking-stats";

/**
 * - `available`: the person can tick the row.
 * - `selected`: the row is ticked or added automatically (green).
 * - `locked`: required or held, cannot be changed (grey).
 * - `invalid`: the row needs a fix, e.g. a missing day (red).
 * - `blocked`: not selectable for this child (dashed).
 */
export type OfferingRowState =
  "available" | "selected" | "locked" | "invalid" | "blocked";

const CHOICE_TONE = {
  available: "green",
  selected: "green",
  locked: "gray",
  invalid: "red",
} as const;

export function OfferingRowShell({
  state,
  children,
}: Readonly<{
  state: OfferingRowState;
  children: ReactNode;
}>) {
  if (state === "blocked") {
    return (
      <InsetPanel dashed className="text-sm">
        {children}
      </InsetPanel>
    );
  }
  return (
    <ChoiceTile
      as="div"
      tone={CHOICE_TONE[state]}
      selected={state !== "available"}
      aria-invalid={state === "invalid" ? true : undefined}
      className="block p-3 font-normal"
    >
      {children}
    </ChoiceTile>
  );
}

/**
 * Occupancy under an offering in the admin flows (#2186); nothing without
 * stats, so the parent form and a failed stats load show no line.
 */
export function OfferingOccupancyLine({
  stats,
}: Readonly<{ stats: CareOfferingBookingStats | undefined }>) {
  const label = formatOfferingOccupancy(stats);
  if (!label) return null;
  return (
    <span
      className={`mt-1 block text-xs ${
        offeringIsFull(stats) ? "text-moto-amber-strong" : "text-gray-500"
      }`}
    >
      {label}
    </span>
  );
}
