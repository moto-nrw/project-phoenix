import type { CSSProperties, ReactNode } from "react";

/**
 * CapacityStrip is the symmetric summary row of the planning grids
 * (docs/planung-redesign/docs/04-designsprache.md Abschnitt 6.2): a gray
 * footer/header band with one preformatted value per column and a leading row
 * label. By default it renders as a single <tr> so it drops straight into a
 * ResourceGrid <tfoot> slot and lines up with the same columns (`as="tr"`,
 * the default). It can also render as a standalone <div> row for non-table
 * grids such as the Betreuungsplan day-header CSS grid (`as="div"`, with
 * `gridTemplateColumns` reproducing the parent grid's column template).
 *
 * It is generic: values arrive as preformatted content, and understaffing is a
 * per-cell flag the caller sets once it has a minimum-staffing threshold. Until
 * such a threshold exists, no cell is marked and the strip renders plain
 * numbers — the kit never computes a threshold itself.
 */

interface CapacityStripCellBase {
  /** Stable key, typically the matching column key. */
  readonly key: string;
  /**
   * When true, the value renders in understaffing red (#FF3130, text only) and
   * semibold. Caller-driven — omit it to leave the cell unmarked (the default).
   */
  readonly understaffed?: boolean;
}

export type CapacityStripCell =
  | (CapacityStripCellBase & {
      /** Preformatted value, e.g. a person count or "~42 · 6 P.". */
      readonly content: ReactNode;
      readonly reductions?: undefined;
    })
  | (CapacityStripCellBase & {
      /** Raw registered count — reductions are subtracted from this to get
       *  the displayed value (docs/06-betreuungsplan.md Abschnitt 7, F10). */
      readonly content: number;
      /**
       * F10 change-request docking point: when set, the cell shows the
       * reduced value (content minus excused minus sick) and a tooltip
       * breakdown ("42 angemeldet, davon 3 abgemeldet"). Omit it to show the
       * unreduced value with no tooltip.
       */
      readonly reductions: {
        readonly excused: number;
        readonly sick: number;
      };
    });

interface CapacityStripProps {
  /** Leading cell label, e.g. "Kapazität 12-16". */
  readonly rowLabel: ReactNode;
  readonly cells: readonly CapacityStripCell[];
  /**
   * Sticks the label cell to the left so it aligns under a sticky row-header
   * column. Default on; turn off for standalone use without a sticky column.
   */
  readonly stickyLabel?: boolean;
  readonly className?: string;
  /**
   * Render as a table row (default, drops into a ResourceGrid <tfoot>) or as
   * a standalone div row for non-table grids. Default "tr" — existing
   * consumers are unaffected.
   */
  readonly as?: "tr" | "div";
  /**
   * Div mode only: the CSS grid-template-columns of the parent grid, so the
   * strip's columns line up with it. Ignored in "tr" mode, where the table's
   * own column layout already aligns the cells.
   */
  readonly gridTemplateColumns?: string;
  /**
   * Footer (default, `border-t`) or header (`border-b`) position — the
   * Betreuungsplan day-header renders above its grid and needs the border
   * flipped (docs/04-designsprache.md Abschnitt 6.2).
   */
  readonly position?: "footer" | "header";
  /**
   * Width utility class for the leading label cell. Default `min-w-[180px]`
   * matches the ResourceGrid sticky person column. A narrow parent column
   * (e.g. the 64px time gutter of the Betreuungsplan day header) overrides
   * this so the label cell stops forcing extra width onto the grid.
   */
  readonly labelWidthClassName?: string;
}

/** Understaffing-red — only ever used as a text color, never as a fill. */
const UNDERSTAFFED_TEXT_COLOR = "#FF3130";

function cellDisplay(cell: CapacityStripCell): {
  content: ReactNode;
  title?: string;
} {
  // Bei 0 Abmeldungen ist der Wert ungemindert — kein "davon 0
  // abgemeldet"-Tooltip.
  if (cell.reductions && cell.reductions.excused + cell.reductions.sick > 0) {
    const totalReduction = cell.reductions.excused + cell.reductions.sick;
    return {
      // Bei inkonsistenten Daten (mehr Abmeldungen als Anmeldungen) nie eine
      // negative Kopfzahl anzeigen.
      content: Math.max(0, cell.content - totalReduction),
      title: `${cell.content} angemeldet, davon ${totalReduction} abgemeldet`,
    };
  }
  return { content: cell.content };
}

function CellValue({ cell }: { readonly cell: CapacityStripCell }) {
  return (
    <span
      className={cell.understaffed ? "font-semibold" : undefined}
      style={cell.understaffed ? { color: UNDERSTAFFED_TEXT_COLOR } : undefined}
    >
      {cellDisplay(cell).content}
    </span>
  );
}

export function CapacityStrip({
  rowLabel,
  cells,
  stickyLabel = true,
  className,
  as = "tr",
  gridTemplateColumns,
  position = "footer",
  labelWidthClassName = "min-w-[180px]",
}: CapacityStripProps) {
  const borderClass =
    position === "header"
      ? "border-b border-gray-200"
      : "border-t border-gray-200";

  if (as === "div") {
    return (
      <div
        role="row"
        style={{ gridTemplateColumns } as CSSProperties}
        className={`grid ${borderClass} bg-gray-50 ${className ?? ""}`.trim()}
      >
        <div
          role="cell"
          className={`${stickyLabel ? "sticky left-0 z-10" : ""} ${labelWidthClassName} bg-gray-50 px-2 py-1.5 text-left text-xs font-medium text-gray-500`}
        >
          {rowLabel}
        </div>
        {cells.map((cell) => (
          <div
            key={cell.key}
            role="cell"
            title={cellDisplay(cell).title}
            className="border-l border-gray-100 px-2 py-1.5 text-center text-xs text-gray-600 tabular-nums"
          >
            <CellValue cell={cell} />
          </div>
        ))}
      </div>
    );
  }

  return (
    <tr className={`${borderClass} bg-gray-50 ${className ?? ""}`.trim()}>
      <th
        scope="row"
        className={`${stickyLabel ? "sticky left-0 z-10" : ""} ${labelWidthClassName} bg-gray-50 px-2 py-1.5 text-left text-xs font-medium text-gray-500`}
      >
        {rowLabel}
      </th>
      {cells.map((cell) => (
        <td
          key={cell.key}
          title={cellDisplay(cell).title}
          className="border-l border-gray-100 px-2 py-1.5 text-center text-xs text-gray-600 tabular-nums"
        >
          <CellValue cell={cell} />
        </td>
      ))}
    </tr>
  );
}
