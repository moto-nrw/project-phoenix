// Per-offering booking summary for the ADMIN flows (#2186). Capacity and
// availability rules are enforced on save; without this the only way an admin
// learns an offering is full is the error message after submitting a whole
// manual enrollment.
//
// Aggregate-only by design: counts per grade level, never child identities.

import { createLogger } from "~/lib/logger";
import { readEnrollmentError } from "~/lib/enrollment-error-messages";
import type { CareOfferingBookingGradeCounts } from "~/lib/care-offering-availability";

const logger = createLogger({ component: "CareOfferingBookingStatsAPI" });

export interface CareOfferingBookingStats extends CareOfferingBookingGradeCounts {
  offering_id: string;
  /** Absent/null when the offering has no capacity limit. */
  capacity?: number | null;
  /**
   * Peak number of children holding a slot at the same time inside the
   * phase's remaining service window — the same number the backend capacity
   * gate compares against `capacity` when a booking is saved.
   */
  booked: number;
}

interface BackendEnvelope<T> {
  status?: string;
  data?: T;
}

/**
 * Loads booking stats for every offering of a phase. Returns them keyed by
 * offering id, which is how every consumer looks them up.
 */
export async function fetchCareOfferingBookingStats(
  phaseId: string,
): Promise<Record<string, CareOfferingBookingStats>> {
  const response = await fetch(
    `/api/enrollment/care-offerings/booking-stats?phase_id=${encodeURIComponent(phaseId)}`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw await readEnrollmentError(
      response,
      "Belegung konnte nicht geladen werden",
      logger,
      "care_offering_booking_stats_failed",
    );
  }
  const raw = (await response.json()) as
    BackendEnvelope<CareOfferingBookingStats[]> | CareOfferingBookingStats[];
  const list = Array.isArray(raw) ? raw : (raw.data ?? []);
  const byID: Record<string, CareOfferingBookingStats> = {};
  for (const entry of list) {
    byID[entry.offering_id] = entry;
  }
  return byID;
}

/**
 * German one-liner for an offering's occupancy, e.g. "18 von 20 Plätzen
 * belegt". Returns null when there is nothing informative to show, so callers
 * render no line at all rather than an empty one.
 */
export function formatOfferingOccupancy(
  stats: CareOfferingBookingStats | null | undefined,
): string | null {
  if (!stats) return null;
  if (stats.capacity == null) {
    return stats.booked > 0
      ? `${stats.booked} gebucht · unbegrenzte Plätze`
      : "Unbegrenzte Plätze";
  }
  if (stats.booked >= stats.capacity) {
    return `Ausgebucht (${stats.booked} von ${stats.capacity} Plätzen belegt)`;
  }
  return `${stats.booked} von ${stats.capacity} Plätzen belegt`;
}

/** True when one more booking would exceed the offering's capacity. */
export function offeringIsFull(
  stats: CareOfferingBookingStats | null | undefined,
): boolean {
  if (!stats || stats.capacity == null) return false;
  return stats.booked >= stats.capacity;
}
