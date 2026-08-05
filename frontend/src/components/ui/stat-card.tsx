// Dense stat tile: uppercase label, one large figure, an optional hint line and
// an optional progress bar. The shape the staff Übersicht/Zeiterfassung tabs and
// the Einrichtungs-Übersicht all needed and each hand-rolled separately, with a
// different surface every time (#2165). InfoCard stays the answer for the
// icon + heading + content card; this one is for a single number.
//
// Tones use the StatusBadge vocabulary. The COLORS, though, are the ones the
// Zeiterfassung applies to a standalone figure: the brand hexes themselves
// (LOCATION_COLORS) plus #70b525 for green, not StatusBadge's darkened label
// colors. Those darkened tones exist to sit legibly on a tinted pill; on a
// large bold number on white they read brown rather than orange. Callers
// holding the older "amber" spelling (getDeltaStatus) map it to "orange" at
// the boundary.

export type StatCardTone = "blue" | "green" | "orange" | "red" | "gray";

const TONE_VALUE_COLOR: Record<StatCardTone, string> = {
  blue: "#5080D8",
  green: "#70b525",
  orange: "#F78C10",
  red: "#FF3130",
  gray: "#374151",
};

const TONE_BAR_COLOR: Record<StatCardTone, string> = {
  blue: "#5080D8",
  green: "#83CD2D",
  orange: "#F78C10",
  red: "#FF3130",
  gray: "#9CA3AF",
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
        style={{ color: TONE_VALUE_COLOR[tone] }}
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
              backgroundColor: TONE_BAR_COLOR[tone],
            }}
          />
        </div>
      ) : null}
    </div>
  );
}
