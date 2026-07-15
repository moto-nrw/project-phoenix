import type { ReactNode } from "react";

/**
 * CapacityStrip is the symmetric summary row of the planning grids
 * (docs/planung-redesign/docs/04-designsprache.md Abschnitt 6.2): a gray
 * footer/header band with one preformatted value per column and a leading row
 * label. It renders as a single <tr> so it drops straight into a ResourceGrid
 * <tfoot> slot and lines up with the same columns.
 *
 * It is generic: values arrive as preformatted content, and understaffing is a
 * per-cell flag the caller sets once it has a minimum-staffing threshold. Until
 * such a threshold exists, no cell is marked and the strip renders plain
 * numbers — the kit never computes a threshold itself.
 */

export interface CapacityStripCell {
  /** Stable key, typically the matching column key. */
  readonly key: string;
  /** Preformatted value, e.g. a person count or "~42 · 6 P.". */
  readonly content: ReactNode;
  /**
   * When true, the value renders in understaffing red (#FF3130, text only) and
   * semibold. Caller-driven — omit it to leave the cell unmarked (the default).
   */
  readonly understaffed?: boolean;
}

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
}

/** Understaffing-red — only ever used as a text color, never as a fill. */
const UNDERSTAFFED_TEXT_COLOR = "#FF3130";

export function CapacityStrip({
  rowLabel,
  cells,
  stickyLabel = true,
  className,
}: CapacityStripProps) {
  return (
    <tr
      className={`border-t border-gray-200 bg-gray-50 ${className ?? ""}`.trim()}
    >
      <th
        scope="row"
        className={`${
          stickyLabel ? "sticky left-0 z-10" : ""
        }min-w-[180px] bg-gray-50 px-2 py-1.5 text-left text-xs font-medium text-gray-500`}
      >
        {rowLabel}
      </th>
      {cells.map((cell) => (
        <td
          key={cell.key}
          className="border-l border-gray-100 px-2 py-1.5 text-center text-xs text-gray-600 tabular-nums"
        >
          <span
            className={cell.understaffed ? "font-semibold" : undefined}
            style={
              cell.understaffed ? { color: UNDERSTAFFED_TEXT_COLOR } : undefined
            }
          >
            {cell.content}
          </span>
        </td>
      ))}
    </tr>
  );
}
