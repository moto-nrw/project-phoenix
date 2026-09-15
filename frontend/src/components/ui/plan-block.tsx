"use client";

import type { CSSProperties, MouseEventHandler, ReactNode } from "react";

/**
 * PlanBlock is the shared shift/occupancy card: white surface, a 3px
 * colored left edge, at most a 10% tint of that same color, and at most
 * one status icon.
 * Nothing else in the kit is allowed to reproduce this recipe or the
 * `repeating-linear-gradient` hatch below — see plan-design-guards.test.ts.
 */

type PlanBlockSize = "default" | "compact";
type PlanBlockStatus = "default" | "hatched" | "cancelled";

interface PlanBlockProps {
  /** Zeitspanne, z. B. "12:00–14:00". */
  readonly timeRange: string;
  /** Blocklabel (Titel der Instanz bzw. Schichtart). */
  readonly label: string;
  /**
   * Farbe der Schichtart/Kategorie als 6-stelliger Hex-Wert ("#RRGGBB").
   * Kein anderes Format: die Tönung hängt "1A" (10 % Alpha) an den Wert an,
   * Kurzform-Hex oder rgb() ergäben also stillschweigend eine ungültige
   * CSS-Farbe. Ohne Wert: neutrale Kante.
   */
  readonly color?: string;
  readonly size?: PlanBlockSize;
  readonly status?: PlanBlockStatus;
  /** 10-Prozent-Tönung der Kantenfarbe als Flächenfarbe. Default an. */
  readonly tinted?: boolean;
  readonly selected?: boolean;
  /** Genau ein Status-Icon (14px), absolut rechts oben. */
  readonly statusIcon?: ReactNode;
  /**
   * Optionaler dritter Slot bei size="default", z. B. ein CoverageIndicator.
   * Nur nicht-interaktive Inhalte (Spans, Badges): bei interactive landet der
   * Slot INNERHALB des <button> — verschachtelte interaktive Elemente wären
   * ungültiges HTML.
   */
  readonly footer?: ReactNode;
  /** Rendert <div> statt <button> und deaktiviert das Klick-/Fokusverhalten. */
  readonly interactive?: boolean;
  readonly onClick?: MouseEventHandler<HTMLButtonElement>;
  readonly className?: string;
  /**
   * Positionierungs-Stile des Aufrufers (z. B. absolute `top`/`left`/`height`
   * einer Kalenderspalte). Wird UNTER die Block-Rezeptur gemergt: die
   * Rezeptur-Keys (`borderLeft`, `backgroundColor`, `backgroundImage`) bleiben
   * autoritativ und lassen sich hierüber nicht überschreiben.
   */
  readonly style?: CSSProperties;
  readonly "aria-label"?: string;
}

/** Bestandskonstante `UNTYPED_SHIFT_COLOR` (frühere per-Zell-Wochenansicht) — die
 *  neutrale Kante für Blöcke ohne Schichtart, jetzt hier zentralisiert. */
const UNTYPED_EDGE_COLOR = "#D1D5DB";
const CANCELLED_EDGE_COLOR = "#9CA3AF";

/**
 * The single sanctioned repeating-linear-gradient in the entire kit
 *: a hard-edged hatch for
 * abwesenheitsgestörte Blöcke, never a soft gradient. Do not copy this
 * pattern into another file — plan-design-guards.test.ts enforces that only
 * this file may reference `repeating-linear-gradient`.
 */
const HATCH_BACKGROUND_IMAGE =
  "repeating-linear-gradient(45deg, transparent 0 4px, rgba(107,114,128,0.15) 4px 8px)";

const SIZE_PADDING_CLASS: Record<PlanBlockSize, string> = {
  default: "px-2.5 py-1.5",
  compact: "px-2 py-1",
};

export function PlanBlock({
  timeRange,
  label,
  color,
  size = "default",
  status = "default",
  tinted = true,
  selected,
  statusIcon,
  footer,
  interactive = true,
  onClick,
  className,
  style: positionStyle,
  "aria-label": ariaLabel,
}: PlanBlockProps) {
  const isCancelled = status === "cancelled";
  const edgeColor = isCancelled
    ? CANCELLED_EDGE_COLOR
    : (color ?? UNTYPED_EDGE_COLOR);
  const showTint = tinted && !isCancelled;

  // Aufrufer-Positionierung UNTER die Rezeptur mergen: die Rezeptur-Keys sind
  // autoritativ und dürfen nicht per style-Prop überschrieben werden.
  const style = {
    ...positionStyle,
    borderLeft: `3px solid ${edgeColor}`,
    backgroundColor: showTint ? `${edgeColor}1A` : undefined,
    backgroundImage: status === "hatched" ? HATCH_BACKGROUND_IMAGE : undefined,
  };

  const containerClassName = [
    "relative w-full rounded-md border border-gray-200 bg-white text-left transition-shadow hover:shadow-sm",
    interactive &&
      "focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:outline-none",
    selected && "ring-2 ring-gray-900 ring-offset-1",
    SIZE_PADDING_CLASS[size],
    statusIcon ? "pr-5" : "",
    className ?? "",
  ]
    .filter(Boolean)
    .join(" ");

  const labelClassName = `truncate text-xs font-semibold ${
    isCancelled ? "text-gray-400 line-through" : "text-gray-900"
  }`;

  const content =
    size === "compact" ? (
      <span className="flex min-w-0 items-center gap-1.5">
        <span className="shrink-0 text-[11px] font-medium text-gray-500 tabular-nums">
          {timeRange}
        </span>
        <span className={labelClassName}>{label}</span>
      </span>
    ) : (
      <span className="block min-w-0">
        <span className="block text-[11px] font-medium text-gray-500 tabular-nums">
          {timeRange}
        </span>
        <span className={labelClassName}>{label}</span>
        {footer && <span className="mt-1 block">{footer}</span>}
      </span>
    );

  const statusIconSlot = statusIcon ? (
    <span
      aria-hidden="true"
      className="absolute top-1 right-1 flex h-3.5 w-3.5 items-center justify-center text-gray-500"
    >
      {statusIcon}
    </span>
  ) : null;

  if (!interactive) {
    return (
      <div className={containerClassName} style={style} aria-label={ariaLabel}>
        {content}
        {statusIconSlot}
      </div>
    );
  }

  return (
    <button
      type="button"
      className={containerClassName}
      style={style}
      onClick={onClick}
      aria-pressed={selected}
      aria-label={ariaLabel}
    >
      {content}
      {statusIconSlot}
    </button>
  );
}
