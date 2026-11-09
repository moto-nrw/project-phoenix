import {
  StatusPillBase,
  type StatusBadgeSize,
  type StatusPillDataAttributes,
} from "./status-badge";
import { getLocationBadgeTone } from "~/lib/location-helper";

// The kit pill (StatusPillBase, #5584) for a data-driven colour from
// LOCATION_COLORS, e.g. an amber status: color={LOCATION_COLORS.WARNING}. Use
// it whenever a status/type chip's color comes from data instead of
// hand-rolling tinted pills; for a fixed set of semantic tones use
// StatusBadge.
//
// The label carries the accessible shade of the color: the palette hexes are
// fills, and at this size (12px) several of them miss the text minimum on the
// pill — brand green lands at 2.0:1. Same hue, only dark enough to read.
//
// The shade is measured against the pill's own gray-50, not against white. The
// difference decides: brand orange darkened for white reaches 4.48:1 here and
// misses.
//
// Formerly StatusDotBadge: the leading dot carried the raw hex and went with
// #2476, because tint and label already name the status.
export function StatusColorBadge({
  label,
  color,
  title,
  size,
  compact,
  wrap,
  ...dataAttributes
}: StatusPillDataAttributes & {
  readonly label: string;
  readonly color: string;
  readonly title?: string;
  readonly size?: StatusBadgeSize;
  readonly compact?: boolean;
  readonly wrap?: boolean;
}) {
  const tone = getLocationBadgeTone(color);

  return (
    <StatusPillBase
      label={label}
      title={title}
      size={size}
      compact={compact}
      wrap={wrap}
      backgroundColor={tone.backgroundColor}
      textColor={tone.textColor}
      {...dataAttributes}
    />
  );
}
