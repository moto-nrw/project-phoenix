import { getLocationBadgeTone } from "~/lib/location-helper";

// Generic label+color status pill following the DataTableStatusBadge recipe
// (gray-50 pill, label tinted via inline style). Use it whenever a status/type
// chip's color is data-driven from LOCATION_COLORS instead of hand-rolling
// tinted Tailwind pills; for a fixed set of semantic tones use StatusBadge.
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
}: {
  readonly label: string;
  readonly color: string;
}) {
  const tone = getLocationBadgeTone(color);

  return (
    <span
      className="inline-flex items-center rounded-full px-3 py-1 text-xs font-medium whitespace-nowrap"
      style={{
        backgroundColor: tone.backgroundColor,
        color: tone.textColor,
      }}
    >
      {label}
    </span>
  );
}
