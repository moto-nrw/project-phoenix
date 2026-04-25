"use client";

/**
 * ConflictWarningsBanner — top-of-page yellow banner summarising the soft
 * planning conflicts (room overlap, staff double-booking, student
 * double-booking) detected for the visible week.
 *
 * The MVP backend GET /instances returns conflict_warnings as an empty
 * array (embedding is deferred to a follow-up PR). This banner stays in
 * the layout so wiring the real conflict counts later requires no UI
 * changes — it just hides when count is 0.
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
    <div className="flex items-center gap-3 rounded-lg border border-[#FDE68A] bg-[#FFFBEB] px-4 py-3">
      <TriangleAlert className="h-5 w-5 shrink-0 text-[#A16207]" />
      <div className="flex-1">
        <div className="text-sm font-bold text-[#713F12]">
          {conflictCount} {conflictCount === 1 ? "Konflikt" : "Konflikte"} diese
          Woche
        </div>
        <div className="text-xs text-[#A16207]">
          Doppelt belegte Räume, Personal oder Schüler. Klicke einen markierten
          Termin zur Detailansicht.
        </div>
      </div>
    </div>
  );
}
