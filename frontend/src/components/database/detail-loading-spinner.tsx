"use client";

import { LOCATION_COLORS } from "~/lib/location-helper";

interface DetailLoadingSpinnerProps {
  label: string;
}

// Intentionally separate from ui/Loading: detail panes need a compact centered
// spinner with visible context text, not the shared skeleton layout.
export function DetailLoadingSpinner({ label }: DetailLoadingSpinnerProps) {
  return (
    <div
      role="status"
      aria-live="polite"
      className="flex h-full min-h-[240px] items-center justify-center"
    >
      <div className="flex flex-col items-center gap-4">
        <div
          aria-hidden="true"
          className="h-10 w-10 animate-spin rounded-full border-2 border-gray-200"
          style={{ borderTopColor: LOCATION_COLORS.OTHER_ROOM }}
        />
        <p className="text-sm text-gray-500">{label}</p>
      </div>
    </div>
  );
}
