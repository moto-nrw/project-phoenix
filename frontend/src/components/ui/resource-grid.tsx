"use client";

/* oxlint-disable jsx-a11y/no-noninteractive-tabindex, jsx-a11y/no-static-element-interactions -- the labeled horizontal scroll region must be keyboard-focusable and arrow keys scroll it */

import { Plus } from "lucide-react";
import type { ReactNode } from "react";

/**
 * ResourceGrid is the generic rows-by-columns planning grid of the planning
 * redesign — the table skeleton spec'd in
 * docs/planung-redesign/docs/04-designsprache.md Abschnitt 6.2. It carries no
 * domain knowledge: rows are opaque objects, columns are plain data, and every
 * cell/header is a render-prop slot filled by the caller. The mapping of any
 * domain entity onto a row or a cell lives in the screen views, never here
 * (Fertig-Kriterium Y7). The half-year view and other consumers reuse the same
 * grid with entirely different cell content.
 */

export interface ResourceGridColumn {
  /** Stable key for React and cell lookups. */
  readonly key: string;
  /** Column head text, e.g. "Mo 13.07." or "KW 29". */
  readonly label: string;
  /** Optional second line under the label. */
  readonly sublabel?: string;
  /**
   * Marks the current column (today / current calendar week). It is tinted a
   * NEUTRAL gray, never a semantic hue.
   */
  readonly isCurrent?: boolean;
}

type ResourceGridColumnMode = "days" | "weeks";

interface ResourceGridProps<TRow> {
  readonly columns: readonly ResourceGridColumn[];
  readonly rows: readonly TRow[];
  readonly getRowKey: (row: TRow) => string;
  /** Sticky row-header content (name, subtitle, action menu — all from the caller). */
  readonly renderRowHeader: (row: TRow) => ReactNode;
  /**
   * Cell content per (row, column). Returning null lets the grid render the
   * labelled empty-cell button (if `emptyCellLabel` + `onEmptyCellClick` are
   * given) — the caller decides emptiness, the grid owns the empty affordance.
   *
   * The returned node lands in a `flex flex-col` wrapper carrying the
   * per-mode cell minimum height. Interactive content that should cover the
   * whole cell (a click target, a hover surface) needs `flex-1` to fill that
   * floor — otherwise it floats in the taller box.
   */
  readonly renderCell: (row: TRow, column: ResourceGridColumn) => ReactNode;
  /**
   * Drives the per-column width and the uniform cell height: a handful of wide
   * day columns vs. a couple dozen narrow week columns. Cell content itself is
   * the caller's.
   */
  readonly columnMode?: ResourceGridColumnMode;
  /** Top-left corner header above the sticky row-header column. */
  readonly cornerHeader?: ReactNode;
  /** aria-label of the empty-cell button — the caller supplies the wording. */
  readonly emptyCellLabel?: (row: TRow, column: ResourceGridColumn) => string;
  readonly onEmptyCellClick?: (row: TRow, column: ResourceGridColumn) => void;
  /** Footer slot rendered inside <tfoot>, e.g. a CapacityStrip row. */
  readonly footer?: ReactNode;
  /** Accessible label for the scroll region. */
  readonly ariaLabel?: string;
  /**
   * Id of an external hint element (e.g. a "swipe horizontally" note the
   * consumer renders above the grid) linked to the keyboard-focusable scroll
   * region via aria-describedby. Arrow keys scroll the region regardless.
   */
  readonly scrollHintId?: string;
  readonly className?: string;
}

// Declared per-column floor on the header cells. Under `table-layout: fixed`
// the browser ignores min-width on table cells — the floor that actually binds
// is the table's own minWidth below, computed from the same values.
const COLUMN_MIN_WIDTH_CLASS: Record<ResourceGridColumnMode, string> = {
  days: "min-w-[7.5rem]",
  weeks: "min-w-[3.25rem]",
};

/**
 * Raster metrics — the reason an empty and a filled plan look identical
 * (issue #2026). The table runs in `table-layout: fixed`, so column widths come
 * from the <colgroup> below and no longer from whatever a cell happens to
 * contain. Fixed layout ignores `min-width` on cells, so the per-column floor
 * has to be re-expressed as the table's own `minWidth` (row header + n columns)
 * — below that the scroll container takes over. `cellMinHeightClass` is the
 * matching floor for the row height: it applies to EVERY cell, empty or filled,
 * and is sized so a cell holding one plain entry does not grow past it.
 *
 * The skeleton (dienstplan-skeleton.tsx) imports these so it keeps rastering
 * like the real grid. `columnWidth` carries the same values as
 * COLUMN_MIN_WIDTH_CLASS above as plain lengths — Tailwind only picks up class
 * names written out as literals, hence the two shapes.
 */
export const RESOURCE_GRID_METRICS = {
  // 13rem statt der alten Mindestbreite von 180px: die Spalte war im
  // Auto-Layout je nach Inhalt 194–259px breit. Auf 180px (oder 192px)
  // festgenagelt würden längere Namen ("Zimmermann, Frank") plötzlich
  // abgeschnitten — gemessen im Dienstplan mit 21 Personen.
  rowHeaderWidth: "13rem",
  columnWidth: {
    days: "7.5rem",
    weeks: "3.25rem",
  },
  cellMinHeightClass: {
    days: "min-h-16",
    weeks: "min-h-11",
  },
} as const satisfies {
  rowHeaderWidth: string;
  columnWidth: Record<ResourceGridColumnMode, string>;
  cellMinHeightClass: Record<ResourceGridColumnMode, string>;
};

/** Table width below which the grid scrolls instead of squeezing columns. */
export function resourceGridMinWidth(
  columnCount: number,
  columnMode: ResourceGridColumnMode = "days",
): string {
  return `calc(${RESOURCE_GRID_METRICS.rowHeaderWidth} + ${columnCount} * ${RESOURCE_GRID_METRICS.columnWidth[columnMode]})`;
}

export function ResourceGrid<TRow>({
  columns,
  rows,
  getRowKey,
  renderRowHeader,
  renderCell,
  columnMode = "days",
  cornerHeader,
  emptyCellLabel,
  onEmptyCellClick,
  footer,
  ariaLabel,
  scrollHintId,
  className,
}: ResourceGridProps<TRow>) {
  const columnMinWidth = COLUMN_MIN_WIDTH_CLASS[columnMode];
  const cellWrapperClass = `flex ${RESOURCE_GRID_METRICS.cellMinHeightClass[columnMode]} flex-col`;

  return (
    // Radius, Rand und Ausschnitt sitzen auf der Fläche, gescrollt wird im
    // inneren Container. Sonst zeichnet der Browser die Scrollleiste über die
    // volle Breite der Box, quer durch die abgerundeten Ecken und über den
    // unteren Rand hinaus. Der Fokusring wandert per :has() mit nach außen,
    // weil der Ausschnitt einen Ring am inneren Element abschneiden würde.
    // overflow-clip beschneidet dabei ohne einen zusätzlichen Scroll-Container
    // zu erzeugen, damit die sticky Zeilenköpfe am inneren Scroll-Container
    // ausgerichtet bleiben.
    <div
      className={`max-w-full overflow-clip rounded-2xl border border-gray-200 has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-gray-900 ${className ?? ""}`}
    >
      <div
        role={ariaLabel ? "region" : undefined}
        aria-label={ariaLabel}
        aria-describedby={scrollHintId}
        tabIndex={0}
        onKeyDown={(event) => {
          if (event.target !== event.currentTarget) return;
          if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
          event.preventDefault();
          event.currentTarget.scrollBy({
            left: event.key === "ArrowLeft" ? -240 : 240,
            behavior: "smooth",
          });
        }}
        className="max-w-full overflow-x-auto overscroll-x-contain focus-visible:outline-none"
      >
        <table
          className="w-full table-fixed border-collapse text-sm"
          style={{ minWidth: resourceGridMinWidth(columns.length, columnMode) }}
        >
          {/* Fixed layout reads the column widths from here, so an empty and a
              filled plan raster identically (issue #2026). Only the row header
              is pinned; the data columns share the remaining width equally and
              never fall below the floor baked into the table's minWidth. */}
          <colgroup>
            <col style={{ width: RESOURCE_GRID_METRICS.rowHeaderWidth }} />
            {columns.map((column) => (
              <col key={column.key} />
            ))}
          </colgroup>
          <thead>
            <tr className="bg-gray-50 text-left">
              <th
                scope="col"
                className="sticky left-0 z-10 bg-gray-50 px-2 py-1.5 text-xs font-medium text-gray-500"
              >
                {cornerHeader}
              </th>
              {columns.map((column) => (
                <th
                  key={column.key}
                  scope="col"
                  className={`${columnMinWidth} px-2 py-1.5 text-xs font-medium text-gray-500 ${
                    column.isCurrent ? "bg-gray-100" : ""
                  }`}
                >
                  <span className="block">{column.label}</span>
                  {column.sublabel && (
                    <span className="block font-normal text-gray-400">
                      {column.sublabel}
                    </span>
                  )}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => {
              return (
                <tr key={getRowKey(row)} className="border-t border-gray-100">
                  <th
                    scope="row"
                    className="sticky left-0 z-10 bg-white px-2 py-1.5 text-left align-top font-normal"
                  >
                    {renderRowHeader(row)}
                  </th>
                  {columns.map((column) => {
                    const cell = renderCell(row, column);
                    const showEmptyButton =
                      cell == null && emptyCellLabel && onEmptyCellClick;
                    return (
                      <td
                        key={column.key}
                        className={`border-l border-gray-100 px-1 py-1 align-top ${
                          column.isCurrent ? "bg-gray-50" : ""
                        }`}
                      >
                        {/* The height floor sits on the wrapper, not on the
                            cell content: an empty cell, a bare cell without an
                            empty-cell affordance and a filled one all get the
                            same row height (issue #2026). */}
                        <div className={cellWrapperClass}>
                          {cell != null ? (
                            cell
                          ) : showEmptyButton ? (
                            <button
                              type="button"
                              onClick={() => onEmptyCellClick(row, column)}
                              aria-label={emptyCellLabel(row, column)}
                              className="flex w-full flex-1 items-center justify-center rounded-md text-gray-400 transition-colors hover:bg-gray-50 hover:text-gray-600 focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:outline-none"
                            >
                              <Plus className="h-4 w-4" aria-hidden="true" />
                            </button>
                          ) : null}
                        </div>
                      </td>
                    );
                  })}
                </tr>
              );
            })}
          </tbody>
          {footer && <tfoot>{footer}</tfoot>}
        </table>
      </div>
    </div>
  );
}
