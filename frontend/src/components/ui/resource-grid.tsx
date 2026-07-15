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
   */
  readonly renderCell: (row: TRow, column: ResourceGridColumn) => ReactNode;
  /**
   * Drives only the per-column minimum width: a handful of wide day columns vs.
   * a couple dozen narrow week columns. Cell content itself is the caller's.
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

const COLUMN_MIN_WIDTH_CLASS: Record<ResourceGridColumnMode, string> = {
  days: "min-w-[7.5rem]",
  weeks: "min-w-[3.25rem]",
};

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

  return (
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
      className={`max-w-full overflow-x-auto overscroll-x-contain rounded-2xl border border-gray-200 focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:outline-none ${className ?? ""}`}
    >
      <table className="w-full border-collapse text-sm">
        <thead>
          <tr className="bg-gray-50 text-left">
            <th
              scope="col"
              className="sticky left-0 z-10 min-w-[180px] bg-gray-50 px-2 py-1.5 text-xs font-medium text-gray-500"
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
                  className="sticky left-0 z-10 min-w-[180px] bg-white px-2 py-1.5 text-left align-top font-normal"
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
                      {cell != null ? (
                        cell
                      ) : showEmptyButton ? (
                        <button
                          type="button"
                          onClick={() => onEmptyCellClick(row, column)}
                          aria-label={emptyCellLabel(row, column)}
                          className="flex min-h-14 w-full items-center justify-center rounded-md text-gray-400 transition-colors hover:bg-gray-50 hover:text-gray-600 focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:outline-none"
                        >
                          <Plus className="h-4 w-4" aria-hidden="true" />
                        </button>
                      ) : null}
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
  );
}
