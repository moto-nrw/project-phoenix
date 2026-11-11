/**
 * CoverageIndicator names the staffing state of a planned position: a status
 * dot plus an "Ist/Soll" number pair (or free-form text for aggregates like a
 * weekly-hours total). Keep the shared anatomy and colors consistent.
 *
 * The `state` prop is always derived by the caller from domain data (an open
 * `GapInstance`, a quittierte Lücke, a plain covered position, …).
 * CoverageIndicator never computes coverage itself; it only renders a state
 * it is told.
 */

import { COVERAGE_DELTA_TEXT_COLOR, PLANNING_COLORS } from "./planning-colors";

export type CoverageState = "covered" | "gap" | "acknowledged";
type CoverageIndicatorSize = "sm" | "md";
/**
 * Delta-Tönung des Freitext-`label` (Wochensummen): neutral hält die
 * zustandsabhängige Graufärbung, `under` färbt rot (Unterdeckung), `over`
 * dunkles Amber (Überdeckung). Wirkt ausschließlich auf die Freitext-Label-Farbe.
 */
type CoverageTone = "neutral" | "under" | "over";

interface CoverageIndicatorProps {
  /** Deckungszustand, vom Aufrufer aus Fachdaten abgeleitet. */
  readonly state: CoverageState;
  /** Ist-Zahl des "Ist/Soll"-Paars, z. B. 1 in "1/3". */
  readonly current?: number;
  /** Soll-Zahl des "Ist/Soll"-Paars, z. B. 3 in "1/3". */
  readonly total?: number;
  /**
   * Freier Zahltext statt des "Ist/Soll"-Zahlenpaars, z. B. eine Wochensumme
   * "19,5/20,25 h". Hat Vorrang vor `current`/`total`, wenn beides gesetzt ist.
   */
  readonly label?: string;
  /**
   * Delta-Tönung des Freitext-`label`. Nur wirksam, wenn `label` gesetzt ist;
   * `neutral` (Default) ändert nichts am Bestandsverhalten.
   */
  readonly tone?: CoverageTone;
  /** Zusatzvermerk für state="acknowledged", z. B. "bewusst unbesetzt". */
  readonly note?: string;
  readonly size?: CoverageIndicatorSize;
  /** Überschreibt die automatisch erzeugte ausgeschriebene Beschreibung. */
  readonly title?: string;
  readonly className?: string;
}

const DOT_COLOR: Record<CoverageState, string> = {
  covered: PLANNING_COLORS.covered,
  gap: PLANNING_COLORS.gap,
  acknowledged: PLANNING_COLORS.acknowledged,
};

const DOT_SIZE_PX: Record<CoverageIndicatorSize, string> = {
  md: "8px",
  sm: "6px",
};

const NUMBER_COLOR_CLASS: Record<CoverageState, string> = {
  covered: "text-gray-600",
  gap: "font-semibold text-gray-900",
  acknowledged: "text-gray-600",
};

function defaultDescription({
  state,
  current,
  total,
  note,
}: {
  state: CoverageState;
  current: number | undefined;
  total: number | undefined;
  note: string | undefined;
}): string {
  if (state === "acknowledged") {
    return note ?? "Bewusst unbesetzt";
  }
  if (current === undefined || total === undefined) {
    return state === "gap" ? "Offene Lücke" : "Besetzt";
  }
  if (state === "gap") {
    const open = total - current;
    return `${current} von ${total} Positionen besetzt, ${open} offen`;
  }
  return `${current} von ${total} Positionen besetzt`;
}

export function CoverageIndicator({
  state,
  current,
  total,
  label,
  tone = "neutral",
  note,
  size = "md",
  title,
  className,
}: CoverageIndicatorProps) {
  const numberSizeClass = "text-xs";
  const numberColorClass = NUMBER_COLOR_CLASS[state];
  const description =
    title ?? defaultDescription({ state, current, total, note });

  let numberContent: React.ReactNode = null;
  if (label !== undefined) {
    const toneColor =
      tone === "neutral" ? undefined : COVERAGE_DELTA_TEXT_COLOR[tone];
    numberContent = (
      <span
        className={`${numberSizeClass} tabular-nums ${numberColorClass}`}
        style={toneColor ? { color: toneColor } : undefined}
      >
        {label}
      </span>
    );
  } else if (current !== undefined && total !== undefined) {
    const isUnderstaffed = state === "gap" && current < total;
    numberContent = (
      <span className={`${numberSizeClass} tabular-nums ${numberColorClass}`}>
        <span
          style={
            isUnderstaffed
              ? { color: PLANNING_COLORS.understaffedText }
              : undefined
          }
        >
          {current}
        </span>
        {`/${total}`}
      </span>
    );
  }

  return (
    <span
      title={description}
      aria-label={description}
      className={`inline-flex items-center gap-1 ${className ?? ""}`}
    >
      <span
        aria-hidden="true"
        className="rounded-full"
        style={{
          width: DOT_SIZE_PX[size],
          height: DOT_SIZE_PX[size],
          backgroundColor: DOT_COLOR[state],
        }}
      />
      {numberContent}
      {state === "acknowledged" && note && (
        <span className="text-xs text-gray-500">{note}</span>
      )}
    </span>
  );
}
