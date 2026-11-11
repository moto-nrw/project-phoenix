"use client";

import {
  Fragment,
  useCallback,
  useEffect,
  useId,
  useMemo,
  useState,
  type KeyboardEvent,
  type ReactNode,
} from "react";

import { ChevronRight } from "lucide-react";

import {
  AT_LEAST_MD,
  useResolvedMediaQuery,
} from "~/lib/hooks/use-media-query";
import { MOTO_COLOR_PALETTE } from "~/lib/kit-palette";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { Skeleton } from "~/components/ui/skeleton";
import { StatusBadge } from "~/components/ui/status-badge";
import { StatusColorBadge } from "~/components/ui/status-color-badge";

type SortDirection = "asc" | "desc";

export interface DataTableColumn<T> {
  // The key "actions" marks the row-action column (the row's OverflowMenu).
  // It stays pinned to the right edge of the table, see PINNED_COLUMN_KEY.
  key: string;
  header: string;
  render: (row: T) => ReactNode;
  align?: "left" | "right" | "center";
  className?: string;
  headerClassName?: string;
  // Tooltip of the column head, when its one word needs an explanation.
  headerTitle?: string;
  // The head stays for assistive tech only (a column of row menus).
  headerSrOnly?: boolean;
  // null is a missing value and sorts after all present values, independent
  // of direction.
  sortValue?: (row: T) => string | number | null;
  // Role this column plays in the stacked phone layout (`stackedOnMobile`).
  // A "field" whose render returns null for a row has no line in that row.
  // "title" is the headline of a stacked row, "meta" the muted value beside
  // it, "field" a labelled line below, "hidden" is dropped on phones.
  // Defaults to "field".
  stacked?: "title" | "meta" | "field" | "hidden";
  // Extra classes for the cell in the "row" phone line, e.g. to lay a
  // stacked badge out side by side.
  stackedClassName?: string;
  // What the "row" phone line shows for this column, when it needs a
  // shorter form than the table cell ("15:30" instead of "🕒 15:30 Uhr").
  stackedRender?: (row: T) => ReactNode;
  // False drops the visible header where the value names itself: in the
  // "row" phone line in front of the value (a class, a group), in the
  // "lines" layout a "field" then takes the full width (an invoice's
  // positions); screen readers keep the header. Defaults to true.
  stackedLabel?: boolean;
}

/**
 * Row selection of a DataTable (#3834): a checkbox column in front, with an
 * "alle auswählen" box in the header that covers every row of the table, not
 * only the rows paged in so far. The caller owns the selection; the table
 * only reports changes. Keys are `String(getRowKey(row))`. The "row" and
 * "lines" stacked phone layouts show the row checkboxes too, without the
 * header box.
 */
interface DataTableSelection<T> {
  readonly selectedKeys: ReadonlySet<string>;
  /** One row or the whole table switched to `selected`. */
  readonly onChange: (keys: readonly string[], selected: boolean) => void;
  /** Name of the row for the checkbox's accessible label ("Mia Kaya"). */
  readonly rowLabel: (row: T) => string;
  /** Locks every checkbox, e.g. while a bulk action runs. */
  readonly disabled?: boolean;
}

/** Title of a table inside its own surface, e.g. one group of a list. */
export interface DataTableHeading {
  readonly title: string;
  readonly count?: number;
}

/**
 * Rows that open on a click or with Enter/Space, one at a time. The table owns
 * which row is open and draws the chevron in the first column. Rows without
 * `canExpand` stay plain: nothing to reveal, no pointer, no hover. On a row
 * that can open, a click opens it; `onRowClick` applies only to the others.
 */
interface DataTableExpandable<T> {
  readonly canExpand?: (row: T) => boolean;
  /** Accessible name of the row, e.g. "05.11.: Details öffnen". */
  readonly rowLabel: (row: T, expanded: boolean) => string;
}

/** One cell of a sub or summary row; it sits under the column `columnKey`. */
interface DataTableExtraCell {
  readonly columnKey: string;
  readonly colSpan?: number;
  readonly content: ReactNode;
}

/**
 * A row under a table row. `cells` line up with the table's columns, so a
 * column without a cell stays empty instead of shifting the rest; `content`
 * spans the whole width (an explanation, a nested history).
 */
export type DataTableExtraRow = {
  readonly key: string;
} & (
  | { readonly cells: readonly DataTableExtraCell[] }
  | { readonly content: ReactNode }
);

// Tint of a whole row. It lives here and not in `rowClassName` because the
// pinned action column has to carry the same tint opaquely.
type DataTableRowTone = "today" | "muted";

interface DataTableProps<T> {
  columns: DataTableColumn<T>[];
  rows: T[];
  getRowKey: (row: T) => string | number;
  onRowClick?: (row: T) => void;
  // Rows with their own buttons or menus must not also become an ARIA button:
  // nested interactive elements have ambiguous keyboard and screen-reader
  // semantics. Their explicit action is the keyboard path to the detail view.
  rowHasInteractiveControls?: boolean;
  emptyState?: ReactNode;
  headerActions?: ReactNode;
  caption?: string;
  isLoading?: boolean;
  // Skeleton rows shown while isLoading; match the typical result count so
  // the swap to real rows doesn't jump.
  loadingRowCount?: number;
  rowClassName?: (row: T) => string;
  defaultSortKey?: string;
  defaultSortDirection?: SortDirection;
  // When set, only the first N sorted rows render; a footer button reveals
  // another N per click. Use on tables whose row count is unbounded
  // (persons, accounts) — leave undefined on bounded views (orgs, devices).
  pageSize?: number;
  // Changing this value resets incremental pagination to the first page.
  // Callers with externally filtered rows should pass their active filter key.
  paginationResetKey?: string | number;
  // Below md, render the same rows stacked instead of as a table. Use it when
  // a phone viewport cannot show the columns that carry the point of the list
  // (a fourth column pushed off screen is a value nobody reads). Sorting,
  // paging, loading and empty state stay shared between both layouts —
  // per-column roles come from `DataTableColumn.stacked`. Cell editors must
  // keep drafts above the table: switching layouts remounts their controls.
  stackedOnMobile?: boolean;
  // "lines" (default) gives every field its own labelled line; "row" shows
  // one entry per line with title, meta and the stackedDetailKey column
  // under the title, so a phone still reads a list (#3834). "form" is for
  // rows whose fields are controls: the title with the meta value (a row
  // menu) beside it, then every field with its label above and the control
  // at full width, so all controls share one left and one right edge
  // (#4807). Only with stackedOnMobile.
  stackedLayout?: "lines" | "row" | "form";
  // "form" only: the headline of a stacked row when the title is itself a
  // control (an editable name). The title column then becomes a field.
  stackedTitle?: (row: T) => ReactNode;
  // The column the "row" phone line shows under the title; null or unset
  // shows title and meta only. Independent of hiddenColumns: the column
  // choice is a computer setting.
  stackedDetailKey?: string | null;
  // Column keys left out of the table. The caller keeps the full column list
  // so a column menu can offer the hidden ones again.
  hiddenColumns?: ReadonlySet<string>;
  selection?: DataTableSelection<T>;
  heading?: DataTableHeading;
  // The row whose detail stands beside the table (list left, detail right,
  // #5692). It carries aria-current and the selected tint in both layouts.
  activeRowKey?: string | number | null;
  expandable?: DataTableExpandable<T>;
  // Rows under a table row: always there (blocks of a day) or only while it is
  // open (`expanded`). Table layout only, the stacked phone rows leave them out.
  renderSubRows?: (row: T, expanded: boolean) => readonly DataTableExtraRow[];
  // A summary after a row and its sub rows, e.g. the sum closing a week. Table
  // layout only.
  summaryRow?: (row: T) => DataTableExtraRow | null;
  rowTone?: (row: T) => DataTableRowTone | null;
  // Narrower cells for a table with many columns.
  dense?: boolean;
}

const alignClass: Record<
  NonNullable<DataTableColumn<unknown>["align"]>,
  string
> = {
  left: "text-left",
  right: "text-right",
  center: "text-center",
};

// Class constants shared between the real table and its skeleton state so
// the two cannot drift apart — the skeleton IS this table's own markup.
const surfaceClass = "moto-content-surface overflow-hidden rounded-2xl border";
const tableClass = "w-full border-collapse text-left text-sm";
const headRowClass =
  "border-b border-gray-100 text-xs font-medium text-gray-500";
// Die Zellenmasse in drei Stufen (#2824): 12 px Zeilenpolster auf dem grossen
// Monitor, 8 px auf einem kleinen oder flachen Bildschirm. Eine Liste ist die
// haeufigste Seite des Portals, und die 8 px je Zeile summieren sich ueber
// zwanzig Zeilen auf mehr als einen halben Bildschirm.
// font-medium on the cell itself: a <th> is bold by browser default, so a
// column without a sort button rendered heavier than its sortable
// neighbours, whose button carries the weight (#3834).
const headCellClass =
  "px-5 py-3 font-medium max-sm:px-3 compact:px-4 compact:py-2";
const denseHeadCellClass = "px-2.5 py-3 font-medium compact:py-2";
const bodyRowClass = "border-b border-gray-50 last:border-0";

// The row-action column stays at the right edge of the surface while wider
// columns scroll beneath it (#4975): on a tablet or phone the menu with
// „Herunterladen“ or „Löschen“ must not hide behind a sideways scroll. When
// the table fits, sticky changes nothing. The cell needs its own background,
// which follows the row's hover and selection.
const PINNED_COLUMN_KEY = "actions";
// The row shown beside the table: the tint plus a dark bar on its leading
// edge, so it stays apart from a row under the pointer.
const ACTIVE_ROW_CLASS =
  "bg-gray-50 shadow-[inset_3px_0_0_0_var(--color-gray-900)]";
const pinnedCellClass = "sticky right-0";

const ROW_TONE_CLASS: Record<DataTableRowTone, string> = {
  today: "bg-moto-blue-soft/60",
  muted: "bg-gray-50/60",
};

// A gradient in the row's translucent colour over white gives the same colour
// as the row, but opaque, so wider columns do not shine through while they
// scroll beneath the pinned cell.
const PINNED_TONE_CLASS: Record<DataTableRowTone, string> = {
  today:
    "bg-linear-to-r from-moto-blue-soft/60 to-moto-blue-soft/60 group-hover:bg-none",
  muted: "bg-linear-to-r from-gray-50/60 to-gray-50/60 group-hover:bg-none",
};

function pinnedBodyCellClass(
  selected: boolean,
  clickable: boolean,
  tone: DataTableRowTone | null,
): string {
  if (selected) return `${pinnedCellClass} bg-gray-50`;
  const tint = tone ? PINNED_TONE_CLASS[tone] : "";
  return clickable
    ? `${pinnedCellClass} bg-white ${tint} transition-colors group-hover:bg-gray-50`
    : `${pinnedCellClass} bg-white ${tint}`;
}
const bodyCellClass =
  "px-5 py-3 align-middle max-sm:px-3 compact:px-4 compact:py-2";
const denseBodyCellClass = "px-2.5 py-3 align-middle compact:py-2";
const extraCellClass = "px-5 py-2 max-sm:px-3 compact:px-4";
const denseExtraCellClass = "px-2.5 py-2";
const subRowClass = `${bodyRowClass} bg-gray-50 text-xs text-gray-500`;
const summaryRowClass = `${bodyRowClass} bg-gray-50 text-xs font-medium text-gray-600`;

// Deterministic width cycle: varied line lengths read as real data without
// Math.random() (SSR-hydration-safe). First column widest, like a name.
const skeletonWidths = ["w-3/4", "w-1/2", "w-2/3", "w-2/5", "w-3/5"];

function skeletonWidth(row: number, col: number): string {
  if (col === 0) return "w-32";
  return skeletonWidths[(row + col) % skeletonWidths.length] ?? "w-2/3";
}

function SkeletonBodyRows({
  columnCount,
  rows,
}: Readonly<{ columnCount: number; rows: number }>) {
  return (
    <>
      {Array.from({ length: rows }, (_, r) => (
        <tr key={r} className={bodyRowClass} aria-hidden="true">
          {Array.from({ length: columnCount }, (_, c) => (
            <td key={c} className={bodyCellClass}>
              <Skeleton className={`h-4 ${skeletonWidth(r, c)}`} />
            </td>
          ))}
        </tr>
      ))}
    </>
  );
}

/**
 * A sub or summary row. Walks the shown columns so that every row has as many
 * cells as the head, whatever the caller left out; the sticky column keeps its
 * opaque cell.
 */
function ExtraRow<T>({
  row,
  columns,
  cellCount,
  rowClass,
  cellClass,
  leadingEmptyCell,
  indentFirstCell,
}: Readonly<{
  row: DataTableExtraRow;
  columns: readonly DataTableColumn<T>[];
  cellCount: number;
  rowClass: string;
  cellClass: string;
  leadingEmptyCell: boolean;
  indentFirstCell: boolean;
}>) {
  if ("content" in row) {
    return (
      <tr className={rowClass}>
        <td colSpan={cellCount} className={cellClass}>
          {row.content}
        </td>
      </tr>
    );
  }
  const cellsByColumn = new Map(row.cells.map((c) => [c.columnKey, c]));
  const cells: ReactNode[] = [];
  for (let index = 0; index < columns.length; index += 1) {
    const col = columns[index];
    if (!col) continue;
    const cell = cellsByColumn.get(col.key);
    const pinned =
      col.key === PINNED_COLUMN_KEY ? `${pinnedCellClass} bg-gray-50` : "";
    cells.push(
      <td
        key={col.key}
        colSpan={cell?.colSpan}
        className={`${cellClass} ${alignClass[col.align ?? "left"]} ${pinned}`}
      >
        {/* Under the text of the row above, past its chevron. */}
        {indentFirstCell && index === 0 ? (
          <div className="pl-5.5">{cell?.content}</div>
        ) : (
          cell?.content
        )}
      </td>,
    );
    index += Math.max(cell?.colSpan ?? 1, 1) - 1;
  }
  return (
    <tr className={rowClass}>
      {leadingEmptyCell ? <td className={cellClass} /> : null}
      {cells}
    </tr>
  );
}

/** One labelled line of a stacked row; the label column keeps values aligned. */
function StackedField({
  label,
  children,
}: Readonly<{ label: string; children: ReactNode }>) {
  return (
    <div className="flex gap-3">
      <dt className="w-28 shrink-0 text-gray-500">{label}</dt>
      <dd className="min-w-0 flex-1 text-right">{children}</dd>
    </div>
  );
}

/**
 * The phone form row (#4807): the headline with the meta value (a row menu)
 * beside it, then one field per column with its label above. Controls take
 * the full width, so every field of every row starts and ends at the same
 * edge instead of hanging off a label column.
 */
function FormStackedRow<T>({
  row,
  headline,
  meta,
  fields,
}: Readonly<{
  row: T;
  headline: ReactNode;
  meta: DataTableColumn<T> | undefined;
  fields: DataTableColumn<T>[];
}>) {
  return (
    <>
      <div className="flex min-h-8 items-center justify-between gap-3">
        <span className="min-w-0 font-medium break-words text-gray-900">
          {headline}
        </span>
        {meta ? (
          <span className="-my-1 -mr-2 flex shrink-0 items-center">
            {meta.render(row)}
          </span>
        ) : null}
      </div>
      {fields.length > 0 && (
        <dl className="mt-3 space-y-3 text-sm">
          {fields.map((col) => {
            const value = col.render(row);
            return value === null ? null : (
              <div key={col.key}>
                <dt className="mb-1 text-gray-600">{col.header}</dt>
                <dd className="min-w-0">{value}</dd>
              </div>
            );
          })}
        </dl>
      )}
    </>
  );
}

const STACKED_ROW_PADDING: Record<"lines" | "row" | "form", string> = {
  lines: "p-4",
  row: "px-4 py-2",
  form: "py-4 first:pt-0 last:pb-0",
};

// A control inside a stacked row (the selection checkbox) that must not
// also trigger the row's own click.
const STACKED_ROW_CONTROL = "[data-row-control]";

/**
 * The checkbox of a phone row. The row ignores clicks from this label
 * (STACKED_ROW_CONTROL): the checkbox marks the row, it must not also open
 * it.
 */
function StackedRowCheckbox<T>({
  row,
  rowKey,
  selection,
}: Readonly<{
  row: T;
  rowKey: string;
  selection: DataTableSelection<T>;
}>) {
  const selected = selection.selectedKeys.has(rowKey);
  return (
    <label
      data-row-control=""
      className="-m-2 flex shrink-0 cursor-pointer items-center p-2"
    >
      <Checkbox
        checked={selected}
        disabled={selection.disabled}
        onChange={() => selection.onChange([rowKey], !selected)}
      />
      <span className="sr-only">{`${selection.rowLabel(row)} auswählen`}</span>
    </label>
  );
}

/**
 * The phone list row (#3834): one entry per line. Optional checkbox, the
 * title with one chosen detail column under it, the meta value at the
 * right. Every other column stays off the phone: a list that grows back
 * into several lines per entry reads like cards again.
 */
function RowStackedRow<T>({
  row,
  rowKey,
  title,
  meta,
  detail,
  selection,
}: Readonly<{
  row: T;
  rowKey: string;
  title: DataTableColumn<T> | undefined;
  meta: DataTableColumn<T> | undefined;
  detail: DataTableColumn<T> | undefined;
  selection?: DataTableSelection<T>;
}>) {
  return (
    <div className="flex min-h-8 items-center gap-3">
      {selection ? (
        <StackedRowCheckbox row={row} rowKey={rowKey} selection={selection} />
      ) : null}
      {/* The title keeps at least 5rem and the meta value at most 9.5rem
          (one badge): a second badge wraps below the first instead of
          taking the name's room, and the name may use that second line too
          rather than being cut. */}
      <div className="min-w-20 flex-1">
        <span className="line-clamp-2 break-words">{title?.render(row)}</span>
        {detail ? (
          <div className="mt-0.5 flex min-w-0 items-center gap-1 text-sm">
            {detail.stackedLabel === false ? null : (
              <span className="shrink-0 text-gray-500">{detail.header}</span>
            )}
            <span className="min-w-0">
              {(detail.stackedRender ?? detail.render)(row)}
            </span>
          </div>
        ) : null}
      </div>
      {meta ? (
        <span
          className={`flex max-w-[9.5rem] min-w-0 justify-end ${meta.stackedClassName ?? ""}`}
        >
          {(meta.stackedRender ?? meta.render)(row)}
        </span>
      ) : null}
    </div>
  );
}

/**
 * Title row INSIDE the table surface (#3834): a grouped list puts each group's
 * title on its own card, never free on the patterned page background
 * (BAUARTEN-SPEC, Teil 3) — the same place SectionCard gives a tile group.
 */
function SurfaceHeading({ heading }: Readonly<{ heading: DataTableHeading }>) {
  return (
    <div className="flex items-center justify-between gap-3 border-b border-gray-100 px-5 py-3 max-sm:px-4">
      <h3 className="text-base font-semibold text-balance text-gray-900">
        {heading.title}
      </h3>
      {heading.count === undefined ? null : (
        <StatusBadge tone="gray" compact label={String(heading.count)} />
      )}
    </div>
  );
}

/**
 * The phone layout of DataTable: the same sorted, paged rows rendered stacked
 * inside one surface. Enabled per call site via `stackedOnMobile`.
 */
function StackedRows<T>({
  columns,
  rows,
  getRowKey,
  onRowClick,
  rowHasInteractiveControls,
  rowClassName,
  isLoading,
  loadingRowCount,
  emptyState,
  hasMore,
  loadMore,
  totalCount,
  layout,
  stackedTitle,
  selection,
  detail,
  heading,
  activeRowKey,
}: Readonly<{
  columns: DataTableColumn<T>[];
  rows: T[];
  getRowKey: (row: T) => string | number;
  onRowClick?: (row: T) => void;
  rowHasInteractiveControls?: boolean;
  rowClassName?: (row: T) => string;
  isLoading?: boolean;
  loadingRowCount: number;
  emptyState?: ReactNode;
  hasMore: boolean;
  loadMore: () => void;
  totalCount: number;
  layout: "lines" | "row" | "form";
  stackedTitle?: (row: T) => ReactNode;
  selection?: DataTableSelection<T>;
  detail?: DataTableColumn<T>;
  heading?: DataTableHeading;
  activeRowKey?: string | number | null;
}>) {
  const title =
    layout === "form" && stackedTitle
      ? undefined
      : (columns.find((c) => c.stacked === "title") ?? columns[0]);
  const meta = columns.find((c) => c.stacked === "meta");

  const fields = columns.filter(
    (c) => c !== title && c !== meta && c.stacked !== "hidden",
  );
  const clickable = Boolean(onRowClick);
  const keyboardClickable =
    clickable && !rowHasInteractiveControls && selection === undefined;

  if (isLoading) {
    return (
      <div className={surfaceClass} aria-hidden="true">
        <ul className="divide-y divide-gray-100">
          {Array.from({ length: loadingRowCount }, (_, i) => (
            <li key={i} className="space-y-2 p-4">
              <Skeleton className="h-4 w-40" />
              <Skeleton className="h-3 w-full" />
              <Skeleton className="h-3 w-2/3" />
            </li>
          ))}
        </ul>
      </div>
    );
  }

  if (rows.length === 0) {
    return (
      <div className={`${surfaceClass} p-6`}>
        {emptyState ?? (
          <p className="text-center text-sm text-gray-500">
            Keine Einträge vorhanden.
          </p>
        )}
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {/* The form rows stand in the card that holds them, like its other
          fields; a second frame inside the card only costs width (#4807). */}
      <div className={layout === "form" ? "" : surfaceClass}>
        {heading ? <SurfaceHeading heading={heading} /> : null}
        <ul className="divide-y divide-gray-100">
          {rows.map((row) => {
            const active =
              activeRowKey != null && getRowKey(row) === activeRowKey;
            return (
              <li
                key={getRowKey(row)}
                aria-current={active ? "true" : undefined}
                className={`${STACKED_ROW_PADDING[layout]} ${clickable ? "cursor-pointer" : ""} ${active ? ACTIVE_ROW_CLASS : ""} ${rowClassName ? rowClassName(row) : ""}`}
                onClick={
                  onRowClick
                    ? (event) => {
                        if (
                          event.target instanceof Element &&
                          event.target.closest(STACKED_ROW_CONTROL)
                        ) {
                          return;
                        }
                        onRowClick(row);
                      }
                    : undefined
                }
                onKeyDown={
                  keyboardClickable
                    ? (event) => {
                        if (event.target !== event.currentTarget) return;
                        if (event.key === "Enter" || event.key === " ") {
                          event.preventDefault();
                          onRowClick?.(row);
                        }
                      }
                    : undefined
                }
                tabIndex={keyboardClickable ? 0 : undefined}
                role={keyboardClickable ? "button" : undefined}
              >
                {layout === "form" ? (
                  <FormStackedRow
                    row={row}
                    headline={
                      stackedTitle ? stackedTitle(row) : title?.render(row)
                    }
                    meta={meta}
                    fields={fields}
                  />
                ) : layout === "row" ? (
                  <RowStackedRow
                    row={row}
                    rowKey={String(getRowKey(row))}
                    title={title}
                    meta={meta}
                    detail={detail}
                    selection={selection}
                  />
                ) : (
                  <>
                    <div className="flex items-baseline justify-between gap-3">
                      {selection ? (
                        <StackedRowCheckbox
                          row={row}
                          rowKey={String(getRowKey(row))}
                          selection={selection}
                        />
                      ) : null}
                      <span className="min-w-0 flex-1">
                        {title?.render(row)}
                      </span>
                      {meta ? (
                        <span className="shrink-0 text-xs text-gray-500">
                          {meta.render(row)}
                        </span>
                      ) : null}
                    </div>
                    {fields.length > 0 && (
                      <dl className="mt-2 space-y-1 text-sm">
                        {fields.map((col) => {
                          // A cell without a value for this row (null) drops
                          // its line instead of showing a bare label.
                          const value = col.render(row);
                          if (value === null) return null;
                          // Without its label a field takes the full width
                          // (an invoice's positions with their amounts).
                          if (col.stackedLabel === false) {
                            return (
                              <div key={col.key}>
                                <dt className="sr-only">{col.header}</dt>
                                <dd className="min-w-0">{value}</dd>
                              </div>
                            );
                          }
                          return (
                            <StackedField key={col.key} label={col.header}>
                              {value}
                            </StackedField>
                          );
                        })}
                      </dl>
                    )}
                  </>
                )}
              </li>
            );
          })}
        </ul>
      </div>
      {hasMore && (
        <Button
          type="button"
          variant="surface"
          size="md"
          onClick={loadMore}
          className="w-full"
        >
          {`Mehr laden (${rows.length} von ${totalCount})`}
        </Button>
      )}
    </div>
  );
}

/**
 * Standalone loading table for call sites that don't have column definitions
 * at hand (page-level fallbacks). Reuses the exact class constants of the
 * real DataTable above; header cells show bars since the labels are unknown.
 * Prefer `<DataTable isLoading>` whenever the columns exist — that renders
 * the real header labels.
 */
export function DataTableSkeleton({
  rows = 6,
  columns = 5,
  caption = false,
}: Readonly<{ rows?: number; columns?: number; caption?: boolean }>) {
  return (
    <div className="w-full" aria-hidden="true">
      {caption && <Skeleton className="mb-3 h-4 w-56" />}
      <div className={surfaceClass}>
        <div className="overflow-x-auto">
          <table className={tableClass}>
            <thead>
              <tr className={headRowClass}>
                {Array.from({ length: columns }, (_, c) => (
                  <th key={c} scope="col" className={headCellClass}>
                    <Skeleton className={`h-3 ${c === 0 ? "w-24" : "w-16"}`} />
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              <SkeletonBodyRows columnCount={columns} rows={rows} />
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

export function DataTable<T>({
  columns,
  rows,
  getRowKey,
  onRowClick,
  rowHasInteractiveControls = false,
  emptyState,
  headerActions,
  caption,
  isLoading,
  loadingRowCount = 6,
  rowClassName,
  defaultSortKey,
  defaultSortDirection = "asc",
  pageSize,
  paginationResetKey,
  stackedOnMobile = false,
  stackedLayout = "lines",
  stackedTitle,
  stackedDetailKey,
  hiddenColumns,
  selection,
  heading,
  activeRowKey,
  expandable,
  renderSubRows,
  summaryRow,
  rowTone,
  dense = false,
}: Readonly<DataTableProps<T>>) {
  const clickable = Boolean(onRowClick);
  const headCell = dense ? denseHeadCellClass : headCellClass;
  const bodyCell = dense ? denseBodyCellClass : bodyCellClass;
  const extraCell = dense ? denseExtraCellClass : extraCellClass;
  const [expandedKey, setExpandedKey] = useState<string | number | null>(null);
  const shownColumns = useMemo(
    () =>
      hiddenColumns && hiddenColumns.size > 0
        ? columns.filter((col) => !hiddenColumns.has(col.key))
        : columns,
    [columns, hiddenColumns],
  );
  // Selection checkboxes are controls inside the row, so the row itself must
  // not also become an ARIA button (see rowHasInteractiveControls).
  const rowControls = rowHasInteractiveControls || selection !== undefined;
  const stackedDetail =
    stackedDetailKey == null
      ? undefined
      : columns.find((col) => col.key === stackedDetailKey);
  const cellCount = shownColumns.length + (selection ? 1 : 0);
  const selectAllId = useId();
  const keyboardClickable = clickable && !rowControls;
  // Only one of the two layouts belongs in the document (#6220). Both run the
  // same cell renderers, so a named control would otherwise exist twice:
  // duplicate ids break its label, and the API error path focuses the first
  // `name` match, which may be the hidden copy. Until the breakpoint is known
  // (server render, hydration) both stay and CSS picks one, so nothing jumps.
  // Match Tailwind's md query exactly, including fractional widths and the
  // browser's default font size. A max-width in px can leave a CSS/JS gap.
  const desktop = useResolvedMediaQuery(AT_LEAST_MD);
  const showStacked = stackedOnMobile && desktop !== true;
  const showTable = !stackedOnMobile || desktop !== false;

  const [sort, setSort] = useState<{
    key: string;
    direction: SortDirection;
  } | null>(
    defaultSortKey
      ? { key: defaultSortKey, direction: defaultSortDirection }
      : null,
  );

  const toggleSort = useCallback((key: string) => {
    setSort((prev) =>
      prev?.key === key
        ? { key, direction: prev.direction === "asc" ? "desc" : "asc" }
        : { key, direction: "asc" },
    );
  }, []);

  const sortedRows = useMemo(() => {
    if (!sort) return rows;
    const col = columns.find((c) => c.key === sort.key);
    if (!col?.sortValue) return rows;
    const getValue = col.sortValue;
    const dir = sort.direction === "asc" ? 1 : -1;
    return [...rows].sort((a, b) => {
      const av = getValue(a);
      const bv = getValue(b);
      if (av === null) return bv === null ? 0 : 1;
      if (bv === null) return -1;
      // Branch on type so mixed string/number columns don't fall back to
      // lexicographic compare and so numeric sort doesn't run through the
      // `<` operator (which coerces undefined to NaN and shuffles rows).
      if (typeof av === "number" && typeof bv === "number") {
        return (av - bv) * dir;
      }
      return String(av).localeCompare(String(bv)) * dir;
    });
  }, [rows, sort, columns]);

  const [visibleCount, setVisibleCount] = useState(
    pageSize ?? Number.POSITIVE_INFINITY,
  );
  // If the caller changes the page size or active filters, snap the visible
  // window back to the first page so a narrower result never inherits a
  // previously expanded row count.
  useEffect(() => {
    setVisibleCount(pageSize ?? Number.POSITIVE_INFINITY);
  }, [pageSize, paginationResetKey]);
  const visibleRows = useMemo(() => {
    if (visibleCount >= sortedRows.length) return sortedRows;
    return sortedRows.slice(0, visibleCount);
  }, [sortedRows, visibleCount]);
  const hasMore = visibleCount < sortedRows.length;
  const loadMore = useCallback(() => {
    setVisibleCount((c) => c + (pageSize ?? sortedRows.length));
  }, [pageSize, sortedRows.length]);

  const allRowKeys = useMemo(
    () => sortedRows.map((row) => String(getRowKey(row))),
    [sortedRows, getRowKey],
  );
  const selectedRowCount = selection
    ? allRowKeys.filter((key) => selection.selectedKeys.has(key)).length
    : 0;
  const allSelected =
    allRowKeys.length > 0 && selectedRowCount === allRowKeys.length;
  const someSelected = selectedRowCount > 0 && !allSelected;

  return (
    // Flex-Spalte, damit die Tabellenfläche wächst, wenn die Seite ihr Platz
    // gibt: als letzte Fläche einer Tenant-Seite reicht sie dann bis zur
    // Unterkante des Bildschirms (`.moto-tenant-body`). Ohne den Platz
    // ändert die Spalte nichts -- die Kinder stehen wie zuvor untereinander.
    <div className="flex w-full flex-col">
      {isLoading && (
        <output aria-live="polite" className="sr-only">
          Wird geladen…
        </output>
      )}
      {(caption ?? headerActions) && (
        <div className="mb-3 flex items-center justify-between gap-3">
          {caption ? (
            <p className="text-sm text-gray-600">{caption}</p>
          ) : (
            <span />
          )}
          {headerActions ? (
            <div className="flex items-center gap-2">{headerActions}</div>
          ) : null}
        </div>
      )}

      {showStacked && (
        <div className="md:hidden" data-testid="data-table-stacked">
          <StackedRows
            // The phone layout has its own detail choice. Desktop column
            // visibility must therefore not remove the title or status that
            // defines every phone row.
            columns={columns}
            rows={visibleRows}
            getRowKey={getRowKey}
            onRowClick={onRowClick}
            rowHasInteractiveControls={rowHasInteractiveControls}
            rowClassName={rowClassName}
            isLoading={isLoading}
            loadingRowCount={loadingRowCount}
            emptyState={emptyState}
            hasMore={hasMore}
            loadMore={loadMore}
            totalCount={sortedRows.length}
            layout={stackedLayout}
            stackedTitle={stackedTitle}
            selection={selection}
            detail={stackedDetail}
            heading={heading}
            activeRowKey={activeRowKey}
          />
        </div>
      )}

      {/* Radius, Rand und Ausschnitt bleiben auf der Kartenfläche, gescrollt
          wird im inneren Container. Sonst zeichnet der Browser die
          Scrollleiste quer durch die abgerundeten Ecken und über den unteren
          Rand hinaus. */}
      {showTable && (
        <div
          className={
            stackedOnMobile
              ? `${surfaceClass} hidden grow md:block`
              : `${surfaceClass} grow`
          }
          data-testid={stackedOnMobile ? "data-table-table" : undefined}
        >
          {heading ? <SurfaceHeading heading={heading} /> : null}
          <div className="overflow-x-auto">
            <table className={tableClass}>
              <thead>
                <tr className={headRowClass}>
                  {selection ? (
                    <th scope="col" className={`${headCell} w-12 pr-0!`}>
                      <label
                        htmlFor={selectAllId}
                        className="flex cursor-pointer items-center"
                      >
                        <Checkbox
                          id={selectAllId}
                          checked={allSelected}
                          ref={(input: HTMLInputElement | null) => {
                            if (input) input.indeterminate = someSelected;
                          }}
                          disabled={
                            selection.disabled || allRowKeys.length === 0
                          }
                          onChange={() =>
                            selection.onChange(allRowKeys, !allSelected)
                          }
                        />
                        <span className="sr-only">Alle auswählen</span>
                      </label>
                    </th>
                  ) : null}
                  {shownColumns.map((col) => {
                    const align = alignClass[col.align ?? "left"];
                    const sortable = Boolean(col.sortValue);
                    const active = sort?.key === col.key;
                    const pinned =
                      col.key === PINNED_COLUMN_KEY
                        ? `${pinnedCellClass} bg-white`
                        : "";
                    if (!sortable) {
                      return (
                        <th
                          key={col.key}
                          scope="col"
                          title={col.headerTitle}
                          className={`${headCell} ${align} ${pinned} ${col.headerClassName ?? ""}`}
                        >
                          {col.headerSrOnly ? (
                            <span className="sr-only">{col.header}</span>
                          ) : (
                            col.header
                          )}
                        </th>
                      );
                    }
                    const ariaSort: "ascending" | "descending" | "none" = active
                      ? sort?.direction === "asc"
                        ? "ascending"
                        : "descending"
                      : "none";
                    const indicator = active
                      ? sort?.direction === "asc"
                        ? "↑"
                        : "↓"
                      : "↕";
                    // Accessible name combines the visible header with a state hint.
                    // We deliberately avoid embedding the column header verbatim
                    // into a separate aria-label so the button's name is always
                    // strictly more specific than the column header text — this
                    // keeps name-based queries that target row-action buttons
                    // (e.g. {name: "Konten"}) from matching the sort header.
                    // The aria-sort attribute on the <th> communicates the
                    // current sort direction to assistive tech independently.
                    const stateHint = active
                      ? sort?.direction === "asc"
                        ? "aufsteigend sortiert"
                        : "absteigend sortiert"
                      : "Spalte sortieren";
                    return (
                      <th
                        key={col.key}
                        scope="col"
                        aria-sort={ariaSort}
                        title={col.headerTitle}
                        className={`${headCell} ${align} ${pinned} ${col.headerClassName ?? ""}`}
                      >
                        <button
                          type="button"
                          onClick={() => toggleSort(col.key)}
                          className={`inline-flex items-center gap-1 font-medium transition-colors select-none hover:text-gray-700 focus:rounded focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-400 ${align}`}
                        >
                          <span>{col.header}</span>
                          <span
                            aria-hidden="true"
                            className={
                              active ? "text-gray-700" : "text-gray-300"
                            }
                          >
                            {indicator}
                          </span>
                          <span className="sr-only">{` – ${stateHint}`}</span>
                        </button>
                      </th>
                    );
                  })}
                </tr>
              </thead>
              <tbody>
                {isLoading ? (
                  <SkeletonBodyRows
                    columnCount={cellCount}
                    rows={loadingRowCount}
                  />
                ) : sortedRows.length === 0 ? (
                  <tr>
                    <td
                      colSpan={cellCount}
                      className="compact:py-6 px-5 py-10 text-center text-sm text-gray-500"
                    >
                      {emptyState ?? "Keine Einträge vorhanden."}
                    </td>
                  </tr>
                ) : (
                  visibleRows.map((row) => {
                    const rowKey = getRowKey(row);
                    const active =
                      activeRowKey != null && rowKey === activeRowKey;
                    const selected =
                      selection?.selectedKeys.has(String(rowKey)) ?? false;
                    const canExpand =
                      expandable !== undefined &&
                      (expandable.canExpand?.(row) ?? true);
                    const expanded = canExpand && expandedKey === rowKey;
                    const toggle = () =>
                      setExpandedKey((prev) =>
                        prev === rowKey ? null : rowKey,
                      );
                    const tinted = selected || active || expanded;
                    const interactive = clickable || canExpand;
                    const tone = tinted ? null : (rowTone?.(row) ?? null);
                    const rowClasses = [
                      `${bodyRowClass} transition-colors`,
                      interactive
                        ? "group cursor-pointer hover:bg-gray-50"
                        : "",
                      tinted ? "bg-gray-50" : "",
                      tone ? ROW_TONE_CLASS[tone] : "",
                      active ? ACTIVE_ROW_CLASS : "",
                      rowClassName ? rowClassName(row) : "",
                    ]
                      .filter(Boolean)
                      .join(" ");
                    const handleRowKey =
                      canExpand || keyboardClickable
                        ? (event: KeyboardEvent<HTMLTableRowElement>) => {
                            if (event.target !== event.currentTarget) return;
                            if (event.key !== "Enter" && event.key !== " ") {
                              return;
                            }
                            event.preventDefault();
                            if (canExpand) toggle();
                            else onRowClick?.(row);
                          }
                        : undefined;
                    const summary = summaryRow?.(row);
                    return (
                      <Fragment key={rowKey}>
                        <tr
                          aria-current={active ? "true" : undefined}
                          aria-expanded={canExpand ? expanded : undefined}
                          aria-label={
                            canExpand
                              ? expandable.rowLabel(row, expanded)
                              : undefined
                          }
                          className={rowClasses}
                          onClick={
                            canExpand
                              ? toggle
                              : onRowClick
                                ? () => onRowClick(row)
                                : undefined
                          }
                          onKeyDown={handleRowKey}
                          tabIndex={
                            canExpand || keyboardClickable ? 0 : undefined
                          }
                          role={
                            !canExpand && keyboardClickable
                              ? "button"
                              : undefined
                          }
                        >
                          {selection ? (
                            <td
                              className={`${bodyCell} w-12 pr-0!`}
                              // The checkbox toggles the mark; it must not also
                              // open the row.
                              onClick={(event) => event.stopPropagation()}
                            >
                              <label className="flex cursor-pointer items-center">
                                <Checkbox
                                  checked={selected}
                                  disabled={selection.disabled}
                                  onChange={() =>
                                    selection.onChange(
                                      [String(rowKey)],
                                      !selected,
                                    )
                                  }
                                />
                                <span className="sr-only">
                                  {`${selection.rowLabel(row)} auswählen`}
                                </span>
                              </label>
                            </td>
                          ) : null}
                          {shownColumns.map((col, colIndex) => {
                            const align = alignClass[col.align ?? "left"];
                            const pinned =
                              col.key === PINNED_COLUMN_KEY
                                ? pinnedBodyCellClass(tinted, interactive, tone)
                                : "";
                            return (
                              <td
                                key={col.key}
                                className={`${bodyCell} text-gray-900 ${align} ${pinned} ${col.className ?? ""}`}
                              >
                                {expandable && colIndex === 0 ? (
                                  <div className="flex items-center gap-2">
                                    {canExpand ? (
                                      <ChevronRight
                                        aria-hidden="true"
                                        className={`h-3.5 w-3.5 shrink-0 text-gray-400 transition-transform ${expanded ? "rotate-90" : ""}`}
                                      />
                                    ) : (
                                      <span className="h-3.5 w-3.5 shrink-0" />
                                    )}
                                    <div className="min-w-0">
                                      {col.render(row)}
                                    </div>
                                  </div>
                                ) : (
                                  col.render(row)
                                )}
                              </td>
                            );
                          })}
                        </tr>
                        {renderSubRows?.(row, expanded).map((subRow) => (
                          <ExtraRow
                            key={subRow.key}
                            row={subRow}
                            columns={shownColumns}
                            cellCount={cellCount}
                            rowClass={subRowClass}
                            cellClass={extraCell}
                            leadingEmptyCell={selection !== undefined}
                            indentFirstCell={expandable !== undefined}
                          />
                        ))}
                        {summary ? (
                          <ExtraRow
                            row={summary}
                            columns={shownColumns}
                            cellCount={cellCount}
                            rowClass={summaryRowClass}
                            cellClass={extraCell}
                            leadingEmptyCell={selection !== undefined}
                            indentFirstCell={false}
                          />
                        ) : null}
                      </Fragment>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>
          {/* Unter dem Scrollbereich, nicht in der Tabelle: in einer Zeile
            stünde der Knopf in der Mitte der ganzen Tabellenbreite und auf
            dem Handy halb außerhalb der Karte (#4975). */}
          {!isLoading && hasMore && (
            <div className="border-t border-gray-100 px-5 py-3 text-center">
              <Button
                type="button"
                variant="surface"
                size="md"
                onClick={loadMore}
              >
                {`Mehr laden (${visibleRows.length} von ${sortedRows.length})`}
              </Button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

interface DataTableStatusBadgeProps {
  active: boolean;
  activeLabel?: string;
  inactiveLabel?: string;
  // Tri-state: when the underlying value is unknown / not disclosed, render a
  // neutral badge instead of asserting the inactive state.
  unknown?: boolean;
  unknownLabel?: string;
}

export function DataTableStatusBadge({
  active,
  activeLabel = "Aktiv",
  inactiveLabel = "Inaktiv",
  unknown = false,
  unknownLabel = "Unbekannt",
}: Readonly<DataTableStatusBadgeProps>) {
  const label = unknown ? unknownLabel : active ? activeLabel : inactiveLabel;
  // The three states must stay visually distinct — pointing "inactive" at
  // UNKNOWN made it identical to the unknown branch, leaving the label text as
  // the only difference. Inactive keeps the red this badge carried before the
  // palette move.
  const color = unknown
    ? MOTO_COLOR_PALETTE.stone.base
    : active
      ? MOTO_COLOR_PALETTE.green.base
      : MOTO_COLOR_PALETTE.red.base;
  // Same pill as StatusBadge and StatusColorBadge (#5584): the tint of the
  // state's colour with its accessible text shade.
  return <StatusColorBadge label={label} color={color} />;
}
