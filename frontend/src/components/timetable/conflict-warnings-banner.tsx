"use client";

/**
 * ConflictWarningsBanner — slim top-of-page hint summarising soft planning
 * conflicts (room overlap, staff double-booking, student double-booking)
 * detected for the visible week.
 *
 * Style follows the shadcn idiom used elsewhere on this page: hairline
 * border, no shadow, subtle amber tint reserved for the icon dot. The
 * banner collapses to nothing when conflictCount is 0.
 */

import { TriangleAlert } from "lucide-react";

interface ConflictWarningsBannerProps {
  conflictCount: number;
}

export function ConflictWarningsBanner({
  conflictCount,
}: ConflictWarningsBannerProps) {
  if (conflictCount === 0) {
    return null;
  }

  return (
    <div
      role="status"
      className="flex items-center gap-2.5 rounded-2xl border border-gray-200 bg-white px-3 py-2 shadow-sm"
    >
      <span className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-[#EAB308]/10">
        <TriangleAlert className="h-3.5 w-3.5 text-[#EAB308]" aria-hidden />
      </span>
      <p className="min-w-0 flex-1 text-xs text-gray-600">
        <span className="font-semibold text-gray-900">
          {conflictCount} {conflictCount === 1 ? "Konflikt" : "Konflikte"}
        </span>{" "}
        diese Woche · doppelt belegte Räume, Personal oder Kinder. Klicke einen
        markierten Termin für die Details.
      </p>
    </div>
  );
}
