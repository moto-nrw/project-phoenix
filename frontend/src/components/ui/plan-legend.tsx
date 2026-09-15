/**
 * PlanLegend resolves the categories and block states of a planning grid into a
 * compact wrapping legend. Category entries render a vertical edge-bar swatch that mirrors
 * the 3px PlanBlock edge; state entries render a 12px glyph that echoes a
 * PlanBlock status.
 *
 * The hatched glyph is drawn with SVG lines, NOT a CSS repeating gradient:
 * the sanctioned hatch stays encapsulated in plan-block.tsx (Abschnitt 5,
 * Verbot 1; enforced by plan-design-guards.test.ts).
 * No data fetching — every entry arrives as a prop.
 */

/** Mirrors the PlanBlock statuses that need a distinct legend glyph. */
type PlanLegendEntryVariant = "bar" | "hatched" | "cancelled";

export interface PlanLegendEntry {
  readonly key: string;
  readonly label: string;
  /** Swatch color as a 6-digit hex ("#RRGGBB"); used by the "bar" variant. */
  readonly color?: string;
  /** Glyph variant. Defaults to the colored edge bar. */
  readonly variant?: PlanLegendEntryVariant;
}

interface PlanLegendProps {
  readonly entries: readonly PlanLegendEntry[];
  readonly className?: string;
  readonly "aria-label"?: string;
}

/** Neutral fallback edge (UNTYPED_EDGE_COLOR), matching PlanBlock. */
const FALLBACK_BAR_COLOR = "#D1D5DB";
/** Grays reused for the state glyphs (cancelled edge / hatch strokes). */
const CANCELLED_GLYPH_COLOR = "#9CA3AF";
const HATCH_STROKE_COLOR = "#6B7280";
const HEX6_RE = /^#[0-9a-fA-F]{6}$/;

function LegendGlyph({ entry }: { entry: PlanLegendEntry }) {
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

  return (
    <span
      aria-hidden="true"
      className="h-3 w-1 shrink-0 rounded-full"
      style={{
        backgroundColor:
          entry.color != null && HEX6_RE.test(entry.color)
            ? entry.color
            : FALLBACK_BAR_COLOR,
      }}
    />
  );
}

export function PlanLegend({
  entries,
  className,
  "aria-label": ariaLabel,
}: PlanLegendProps) {
  return (
    <div
      aria-label={ariaLabel}
      className={`flex flex-wrap items-center gap-x-4 gap-y-1 ${className ?? ""}`}
    >
      {entries.map((entry) => (
        <span key={entry.key} className="inline-flex items-center gap-1.5">
          <LegendGlyph entry={entry} />
          <span className="text-[11px] text-gray-600">{entry.label}</span>
        </span>
      ))}
    </div>
  );
}
