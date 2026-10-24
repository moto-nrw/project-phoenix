// Per-offering booking summary for the ADMIN flows (#2186). Capacity and
// availability rules are enforced on save; without this the only way an admin
// learns an offering is full is the error message after submitting a whole
// manual enrollment.
//
// Aggregate-only by design: counts per grade level, never child identities.

import { unavailableApiError } from "~/lib/api-error";
import { createLogger } from "~/lib/logger";
import { readEnrollmentError } from "~/lib/enrollment-api-error";
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

/** A request that never reached the API counts as unavailable (#2515). */
async function send(input: string, init?: RequestInit): Promise<Response> {
  try {
    return await fetch(input, init);
  } catch (cause) {
    throw unavailableApiError(cause);
  }
}

/**
 * Loads booking stats for every offering of a phase. Returns them keyed by
 * offering id, which is how every consumer looks them up.
 */
export async function fetchCareOfferingBookingStats(
  phaseId: string,
): Promise<Record<string, CareOfferingBookingStats>> {
  const response = await send(
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
  return offeringOccupancy(stats)?.full ?? false;
}

/** An offering's occupancy for the list displays (#4782). */
export interface OfferingOccupancy {
  booked: number;
  /** null: the offering has no capacity limit. */
  capacity: number | null;
  /**
   * Places still free, never negative: a booking kept under Bestandsschutz
   * can push `booked` above a lowered capacity. null without a limit.
   */
  free: number | null;
  full: boolean;
}

/** Occupancy of one offering; null while its stats are not loaded. */
export function offeringOccupancy(
  stats: CareOfferingBookingStats | null | undefined,
): OfferingOccupancy | null {
  if (!stats) return null;
  const capacity = stats.capacity ?? null;
  if (capacity == null) {
    return { booked: stats.booked, capacity: null, free: null, full: false };
  }
  return {
    booked: stats.booked,
    capacity,
    free: Math.max(0, capacity - stats.booked),
    full: stats.booked >= capacity,
  };
}

/**
 * Free places summed over the active offerings that have a capacity limit:
 * an inactive offering cannot be booked, so its places are not free. null
 * when none has a limit, or when the stats of one of them are missing: an
 * incomplete sum would read as fewer free places than there are.
 */
export function totalFreePlaces(
  offerings: readonly {
    readonly id: string;
    readonly capacity?: number | null;
    readonly is_active: boolean;
  }[],
  statsById: Readonly<Record<string, CareOfferingBookingStats>>,
): number | null {
  let total = 0;
  let limited = false;
  for (const offering of offerings) {
    if (!offering.is_active || offering.capacity == null) continue;
    const free = offeringOccupancy(statsById[offering.id])?.free;
    if (free == null) return null;
    limited = true;
    total += free;
  }
  return limited ? total : null;
}
