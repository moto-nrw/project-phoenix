import { SURFACE_GRAY_50, getAccessibleTextColor } from "~/lib/location-helper";

// Generic label+color status pill following the DataTableStatusBadge recipe
// (gray-50 pill, colored dot, label tinted via inline style). Use it whenever
// a status/type chip's color is data-driven from LOCATION_COLORS instead of
// hand-rolling tinted Tailwind pills.
//
// The dot carries the raw color, the label its accessible shade: the palette
// hexes are fills, and at this size (12px) several of them miss the text
// minimum on the pill — brand green lands at 2.0:1. Same hue, only dark enough
// to read. The color identifying the status stays the dot's.
//
// The shade is measured against the pill's own gray-50, not against white. The
// difference decides: brand orange darkened for white reaches 4.48:1 here and
// misses.
export function StatusDotBadge({
  label,
  color,
}: {
  readonly label: string;
  readonly color: string;
}) {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full bg-gray-50 px-3 py-1 text-xs font-medium">
      <span
        aria-hidden
        className="inline-block h-1.5 w-1.5 rounded-full"
        style={{ backgroundColor: color }}
      />
      <span style={{ color: getAccessibleTextColor(color, SURFACE_GRAY_50) }}>
        {label}
      </span>
    </span>
  );
}
