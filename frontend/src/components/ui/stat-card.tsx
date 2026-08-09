// Dense stat tile: uppercase label, one large figure, an optional hint line and
// an optional progress bar. The shape the staff Übersicht/Zeiterfassung tabs and
// the Einrichtungs-Übersicht all needed and each hand-rolled separately, with a
// different surface every time (#2165). InfoCard stays the answer for the
// icon + heading + content card; this one is for a single number.
//
// Tones use the StatusBadge vocabulary. Callers holding the older "amber"
// spelling (getDeltaStatus) map it to "orange" at the boundary.
//
// A tone is one brand color, used twice: the progress bar takes it raw (a bar
// is a fill), the figure takes its accessible shade from location-helper (a
// figure is text — the raw hexes miss the contrast minimum on white, brand
// green by a factor of two). Both come from LOCATION_COLORS, so this component
// holds no palette values of its own and cannot go stale when the palette moves.

import { LOCATION_COLORS, getAccessibleTextColor } from "~/lib/location-helper";

export type StatCardTone = "blue" | "green" | "orange" | "red" | "gray";

const TONE_COLOR: Record<StatCardTone, string> = {
  blue: LOCATION_COLORS.OTHER_ROOM,
  green: LOCATION_COLORS.GROUP_ROOM,
  orange: LOCATION_COLORS.SCHOOLYARD,
  red: LOCATION_COLORS.DANGER,
  gray: LOCATION_COLORS.UNKNOWN,
};

export function StatCard({
  label,
  value,
  hint,
  progressPct,
  tone = "gray",
  compactValue,
  action,
}: {
  readonly label: string;
  readonly value: string;
  readonly hint?: string;
  /** Renders the progress bar when set. Clamped to 0…100. */
  readonly progressPct?: number;
  readonly tone?: StatCardTone;
  /**
   * School-wide sums run to four digits ("1521h 11min") and otherwise wrap
   * mid-value. One step smaller, no wrap.
   */
  readonly compactValue?: boolean;
  /** Optional control in the top-right corner (edit pencil, info button). */
  readonly action?: React.ReactNode;
}) {
  return (
    <div className="moto-content-surface relative flex h-full flex-col rounded-2xl border p-4 shadow-sm sm:p-5">
      {/* Absolute so a tile carrying an action keeps the same label→value
          rhythm as its neighbours in the row. */}
      {action != null ? (
        <div className="absolute top-2.5 right-2.5">{action}</div>
      ) : null}
      <p className="pr-8 text-xs font-semibold tracking-wider text-gray-500 uppercase">
        {label}
      </p>
      <p
        className={`mt-2 font-bold ${
          compactValue ? "text-xl whitespace-nowrap" : "text-2xl"
        }`}
        style={{ color: getAccessibleTextColor(TONE_COLOR[tone]) }}
      >
        {value}
      </p>
      {hint !== undefined && hint !== "" ? (
        <p className="mt-1 text-xs text-gray-500">{hint}</p>
      ) : null}
      {progressPct !== undefined ? (
        <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-gray-100">
          <div
            className="h-full rounded-full transition-all"
            style={{
              width: `${Math.min(100, Math.max(0, progressPct))}%`,
              backgroundColor: TONE_COLOR[tone],
            }}
          />
        </div>
      ) : null}
    </div>
  );
}
