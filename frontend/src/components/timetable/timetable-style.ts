import { LOCATION_COLORS } from "~/lib/location-helper";

export const timetableSurface =
  "moto-content-surface rounded-2xl border border-gray-200 bg-white shadow-sm";

export const timetableSurfacePadded = `${timetableSurface} p-4 sm:p-5`;

export const timetableNestedSurface =
  "rounded-xl border border-gray-200 bg-white shadow-sm";

export const timetableMutedSurface =
  "rounded-xl border border-gray-200 bg-gray-50/70";

export const timetablePopoverSurface =
  "overflow-hidden rounded-xl border border-gray-200 bg-white shadow-lg";

export const timetableSelectClass =
  "moto-select block h-10 w-full rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 ring-inset transition-colors hover:ring-gray-300 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500";

export const timetableSearchClass =
  "block h-10 w-full rounded-lg border-0 bg-white py-2 pr-3 pl-9 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 ring-inset transition-colors placeholder:text-gray-400 hover:ring-gray-300 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400";

export const timetableTextAreaClass =
  "w-full rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 ring-inset transition-colors hover:ring-gray-300 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400";

export const timetableDangerPanel =
  "rounded-xl border border-[#FF3130]/20 bg-[#FF3130]/5 p-3";

export const timetableRequiredMark = "ml-0.5 text-[#FF3130]";

/** Readable dark-amber text pairing with white/gray-50 backgrounds. */
export const timetableWarningText = "text-[#92400E]";

/**
 * Shared 5-tone palette for small status/ratio indicators across the
 * timetable feature (issue #1838). Supersedes two previously-separate,
 * incompatible local palettes (the 4-tone StatPill in
 * instance-detail-modal.tsx had no "danger", TimetableStatCard's
 * neutral/success/warning/danger had no "info") — this is the union of both,
 * mapped onto the existing LOCATION_COLORS semantic set. No new hex values.
 */
export type TimetableTone =
  "neutral" | "success" | "info" | "warning" | "danger";

export const timetableToneColors: Record<TimetableTone, string> = {
  neutral: LOCATION_COLORS.UNKNOWN,
  success: LOCATION_COLORS.GROUP_ROOM,
  info: LOCATION_COLORS.OTHER_ROOM,
  warning: LOCATION_COLORS.SICK,
  danger: LOCATION_COLORS.HOME,
} as const;
