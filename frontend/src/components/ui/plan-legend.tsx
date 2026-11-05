import { COLOR_BAR_FALLBACK, ColorBar } from "./color-bar";
import { StatusDot } from "./status-dot";

/**
 * PlanLegend resolves the categories and block states of a planning grid into a
 * compact wrapping legend. Category entries render a vertical edge-bar swatch that mirrors
 * the 3px PlanBlock edge; state entries render a 12px glyph that echoes a
 * PlanBlock status. An entry may carry a `value` (Zähler) and a round `dot`
 * swatch; `size="md"` is the legend under a chart or a list, outside a plan.
 *
 * The hatched glyph is drawn with SVG lines, NOT a CSS repeating gradient:
 * the sanctioned hatch stays encapsulated in plan-block.tsx (Abschnitt 5,
 * Verbot 1; enforced by plan-design-guards.test.ts).
 * No data fetching — every entry arrives as a prop.
 */

/** Mirrors the PlanBlock statuses that need a distinct legend glyph. */
type PlanLegendEntryVariant = "bar" | "dot" | "hatched" | "cancelled";

export interface PlanLegendEntry {
  readonly key: string;
  readonly label: string;
  /** Count shown before the label. */
  readonly value?: number;
  /** Swatch color as a 6-digit hex ("#RRGGBB"); used by "bar" and "dot". */
  readonly color?: string;
  /** Glyph variant. Defaults to the colored edge bar. */
  readonly variant?: PlanLegendEntryVariant;
}

type PlanLegendSize = "sm" | "md";

interface PlanLegendProps {
  readonly entries: readonly PlanLegendEntry[];
  readonly size?: PlanLegendSize;
  readonly className?: string;
  readonly "aria-label"?: string;
}

/** Grays reused for the state glyphs (cancelled edge / hatch strokes). */
const CANCELLED_GLYPH_COLOR = "#9CA3AF";
const HATCH_STROKE_COLOR = "#6B7280";
const HEX6_RE = /^#[0-9a-fA-F]{6}$/;

function LegendGlyph({
  entry,
  size,
}: {
  entry: PlanLegendEntry;
  size: PlanLegendSize;
}) {
  const variant = entry.variant ?? "bar";

  if (variant === "hatched") {
    return (
      <svg
        width="12"
        height="12"
        viewBox="0 0 12 12"
        aria-hidden="true"
        className="shrink-0 rounded-sm border border-gray-200"
      >
        <line x1="0" y1="8" x2="8" y2="0" stroke={HATCH_STROKE_COLOR} />
        <line x1="4" y1="12" x2="12" y2="4" stroke={HATCH_STROKE_COLOR} />
      </svg>
    );
  }

  if (variant === "cancelled") {
    return (
      <svg
        width="12"
        height="12"
        viewBox="0 0 12 12"
        aria-hidden="true"
        className="shrink-0"
      >
        <rect
          x="0.5"
          y="0.5"
          width="11"
          height="11"
          rx="2"
          fill="none"
          stroke={CANCELLED_GLYPH_COLOR}
        />
        <line x1="1" y1="11" x2="11" y2="1" stroke={CANCELLED_GLYPH_COLOR} />
      </svg>
    );
  }

  const color =
    entry.color != null && HEX6_RE.test(entry.color)
      ? entry.color
      : COLOR_BAR_FALLBACK;

  if (variant === "dot") {
    return (
      <StatusDot color={color} size={size === "md" ? "lg" : "md"} decorative />
    );
  }

  return <ColorBar color={color} />;
}

const SM_TEXT = "text-[11px]";

const SIZE_CLASSES = {
  sm: {
    list: "gap-x-4",
    label: `${SM_TEXT} text-gray-600`,
    value: `${SM_TEXT} font-semibold text-gray-900 tabular-nums`,
  },
  md: {
    list: "gap-x-5",
    label: "text-sm text-gray-500",
    value: "text-sm font-medium text-gray-900 tabular-nums",
  },
} as const;

export function PlanLegend({
  entries,
  size = "sm",
  className,
  "aria-label": ariaLabel,
}: PlanLegendProps) {
  return (
    <div
      aria-label={ariaLabel}
      className={`flex flex-wrap items-center gap-y-1 ${SIZE_CLASSES[size].list} ${className ?? ""}`}
    >
      {entries.map((entry) => (
        <span key={entry.key} className="inline-flex items-center gap-1.5">
          <LegendGlyph entry={entry} size={size} />
          {entry.value === undefined ? null : (
            <span className={SIZE_CLASSES[size].value}>{entry.value}</span>
          )}
          <span className={SIZE_CLASSES[size].label}>{entry.label}</span>
        </span>
      ))}
    </div>
  );
}
