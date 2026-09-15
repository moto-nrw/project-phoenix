// Tinted status pill, the enrollment-page badge recipe (#1629). Tones map to
// the brand hexes from LOCATION_COLORS with matching tinted backgrounds and
// darkened label colors. Use this whenever a status is one of a fixed set of
// semantic outcomes; for data-driven colors (a raw hex from LOCATION_COLORS)
// use StatusColorBadge instead.
//
// No leading dot: it only repeated what tint and label already say and made
// rows of pills busier without adding meaning (#2476).
import type { LucideIcon } from "lucide-react";

import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";

export type StatusBadgeTone = "blue" | "green" | "orange" | "red" | "gray";

const TONES: Record<StatusBadgeTone, { bg: string; text: string }> = {
  blue: {
    bg: MOTO_COLOR_PALETTE.blue.soft,
    text: MOTO_COLOR_PALETTE.blue.strong,
  },
  green: {
    bg: MOTO_COLOR_PALETTE.green.soft,
    text: MOTO_COLOR_PALETTE.green.strong,
  },
  orange: {
    bg: MOTO_COLOR_PALETTE.orange.soft,
    text: MOTO_COLOR_PALETTE.orange.strong,
  },
  red: {
    bg: MOTO_COLOR_PALETTE.red.soft,
    text: MOTO_COLOR_PALETTE.red.strong,
  },
  // Muss mit LOCATION_BADGE_TONES[neutral.base] uebereinstimmen, sonst
  // driften der graue StatusBadge und der graue StatusColorBadge auseinander.
  gray: {
    bg: MOTO_COLOR_PALETTE.neutral.soft,
    text: "#4B5563",
  },
};

export function StatusBadge({
  label,
  tone,
  title,
  icon: Icon,
  accessibleLabel,
  compact = false,
}: {
  readonly label: string;
  readonly tone: StatusBadgeTone;
  /**
   * Native tooltip on the badge itself. Use when the short label needs the
   * full context on hover (e.g. "Feiertag" → the holiday's name). Set it here
   * rather than on a wrapper so the tooltip belongs to the element that
   * carries the text.
   */
  readonly title?: string;
  /** Optional state icon. Use with a distinct label, never as the only cue. */
  readonly icon?: LucideIcon;
  /** Extra screen-reader context for a short visible label. */
  readonly accessibleLabel?: string;
  /** Matches compact informational labels that sit inside dense cards. */
  readonly compact?: boolean;
}) {
  const styles = TONES[tone];
  return (
    <span
      title={title}
      className={`inline-flex items-center rounded-full text-xs font-medium whitespace-nowrap ${
        Icon ? "gap-1" : ""
      } ${compact ? "px-2 py-0.5" : "px-2.5 py-1"}`}
      style={{ backgroundColor: styles.bg, color: styles.text }}
    >
      {Icon ? <Icon className="h-3 w-3 shrink-0" aria-hidden="true" /> : null}
      {accessibleLabel ? (
        <span className="sr-only">{accessibleLabel}</span>
      ) : null}
      {label}
    </span>
  );
}
