"use client";

// lib/pickup-helpers.ts
// Shared pickup time urgency helpers used across OGS groups, active supervisions,
// and student search pages.

import { useState, useEffect } from "react";

const PICKUP_URGENCY_SOON_MINUTES = 30;

export type PickupUrgency = "overdue" | "soon" | "normal" | "none";

/**
 * Hook that returns a Date updated every 60 seconds.
 * Used for pickup time urgency calculations across pages.
 */
export function useMinuteClock(): Date {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 60_000);
    return () => clearInterval(id);
  }, []);
  return now;
}

/**
 * Combine separate notes and day notes into a single display string.
 * Used to normalize the two data paths: bulk pickup API (separate fields)
 * and student list API (pre-combined pickup_notes).
 */
export function combinePickupNotes(
  notes?: string,
  dayNotes?: ReadonlyArray<{ content: string }>,
): string | undefined {
  const combined = [notes, ...(dayNotes?.map((n) => n.content) ?? [])]
    .filter(Boolean)
    .join(", ");
  return combined || undefined;
}

export function getPickupUrgency(
  pickupTimeStr: string | undefined,
  now: Date,
): PickupUrgency {
  if (!pickupTimeStr) return "none";

  const [hours, minutes] = pickupTimeStr.split(":").map(Number);
  const pickupDate = new Date(now);
  pickupDate.setHours(hours ?? 0, minutes ?? 0, 0, 0);

  const diffMs = pickupDate.getTime() - now.getTime();
  const diffMinutes = diffMs / 60000;

  if (diffMinutes < 0) return "overdue";
  if (diffMinutes <= PICKUP_URGENCY_SOON_MINUTES) return "soon";
  return "normal";
}
