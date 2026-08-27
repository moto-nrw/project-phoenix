"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

import { LOCATION_COLORS } from "~/lib/location-helper";
import { Skeleton } from "~/components/ui/skeleton";

type SortDirection = "asc" | "desc";

export interface DataTableColumn<T> {
  key: string;
  header: string;
  render: (row: T) => ReactNode;
  align?: "left" | "right" | "center";
  className?: string;
  headerClassName?: string;
  sortValue?: (row: T) => string | number;
  // Role this column plays in the stacked phone layout (`stackedOnMobile`).
  // "title" is the headline of a stacked row, "meta" the muted value beside
  // it, "field" a labelled line below, "hidden" is dropped on phones.
  // Defaults to "field".
  stacked?: "title" | "meta" | "field" | "hidden";
}

interface DataTableProps<T> {
  columns: DataTableColumn<T>[];
  rows: T[];
  getRowKey: (row: T) => string | number;
  onRowClick?: (row: T) => void;
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
  // per-column roles come from `DataTableColumn.stacked`.
  stackedOnMobile?: boolean;
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
const surfaceClass =
  "moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md";
const tableClass = "w-full border-collapse text-left text-sm";
const headRowClass =
  "border-b border-gray-100 text-xs font-medium text-gray-500";
const headCellClass = "px-3 py-3 sm:px-5";
const bodyRowClass = "border-b border-gray-50 last:border-0";
const bodyCellClass = "px-3 py-3 align-middle sm:px-5";

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
              <Skeleton className={`h-4 rounded ${skeletonWidth(r, c)}`} />
            </td>
          ))}
        </tr>
      ))}
    </>
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
 * The phone layout of DataTable: the same sorted, paged rows rendered stacked
 * inside one surface. Enabled per call site via `stackedOnMobile`.
 */
function StackedRows<T>({
  columns,
  rows,
  getRowKey,
  onRowClick,
  rowClassName,
  isLoading,
  loadingRowCount,
  emptyState,
  hasMore,
  loadMore,
  totalCount,
}: Readonly<{
  columns: DataTableColumn<T>[];
  rows: T[];
  getRowKey: (row: T) => string | number;
  onRowClick?: (row: T) => void;
  rowClassName?: (row: T) => string;
  isLoading?: boolean;
  loadingRowCount: number;
  emptyState?: ReactNode;
  hasMore: boolean;
  loadMore: () => void;
  totalCount: number;
}>) {
  const title = columns.find((c) => c.stacked === "title") ?? columns[0];
  const meta = columns.find((c) => c.stacked === "meta");
  const fields = columns.filter(
    (c) => c !== title && c !== meta && c.stacked !== "hidden",
  );
  const clickable = Boolean(onRowClick);

  if (isLoading) {
    return (
      <div className={surfaceClass} aria-hidden="true">
        <ul className="divide-y divide-gray-100">
          {Array.from({ length: loadingRowCount }, (_, i) => (
            <li key={i} className="space-y-2 p-4">
              <Skeleton className="h-4 w-40 rounded" />
              <Skeleton className="h-3 w-full rounded" />
              <Skeleton className="h-3 w-2/3 rounded" />
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
      <div className={surfaceClass}>
        <ul className="divide-y divide-gray-100">
          {rows.map((row) => (
            <li
              key={getRowKey(row)}
              className={`p-4 ${clickable ? "cursor-pointer" : ""} ${rowClassName ? rowClassName(row) : ""}`}
              onClick={onRowClick ? () => onRowClick(row) : undefined}
              onKeyDown={
                onRowClick
                  ? (event) => {
                      if (event.target !== event.currentTarget) return;
                      if (event.key === "Enter" || event.key === " ") {
                        event.preventDefault();
                        onRowClick(row);
                      }
                    }
                  : undefined
              }
              tabIndex={clickable ? 0 : undefined}
              role={clickable ? "button" : undefined}
            >
              <div className="flex items-baseline justify-between gap-3">
                <span className="min-w-0">{title?.render(row)}</span>
                {meta ? (
                  <span className="shrink-0 text-xs text-gray-500">
                    {meta.render(row)}
                  </span>
                ) : null}
              </div>
              {fields.length > 0 && (
                <dl className="mt-2 space-y-1 text-sm">
                  {fields.map((col) => (
                    <StackedField key={col.key} label={col.header}>
                      {col.render(row)}
                    </StackedField>
                  ))}
                </dl>
              )}
            </li>
          ))}
        </ul>
      </div>
      {hasMore && (
        <button
          type="button"
          onClick={loadMore}
          className="w-full rounded-lg border border-gray-200 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-400"
        >
          {`Mehr laden (${rows.length} von ${totalCount})`}
        </button>
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
      {caption && <Skeleton className="mb-3 h-4 w-56 rounded" />}
      <div className={surfaceClass}>
        <div className="overflow-x-auto">
          <table className={tableClass}>
            <thead>
              <tr className={headRowClass}>
                {Array.from({ length: columns }, (_, c) => (
                  <th key={c} scope="col" className={headCellClass}>
                    <Skeleton
                      className={`h-3 rounded ${c === 0 ? "w-24" : "w-16"}`}
                    />
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
}: Readonly<DataTableProps<T>>) {
  const clickable = Boolean(onRowClick);

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

  return (
    <div className="w-full">
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

      {stackedOnMobile && (
        <div className="md:hidden" data-testid="data-table-stacked">
          <StackedRows
            columns={columns}
            rows={visibleRows}
            getRowKey={getRowKey}
            onRowClick={onRowClick}
            rowClassName={rowClassName}
            isLoading={isLoading}
            loadingRowCount={loadingRowCount}
            emptyState={emptyState}
            hasMore={hasMore}
            loadMore={loadMore}
            totalCount={sortedRows.length}
          />
        </div>
      )}

      {/* Radius, Rand und Ausschnitt bleiben auf der Kartenfläche, gescrollt
          wird im inneren Container. Sonst zeichnet der Browser die
          Scrollleiste quer durch die abgerundeten Ecken und über den unteren
          Rand hinaus. */}
      <div
        className={
          stackedOnMobile ? `${surfaceClass} hidden md:block` : surfaceClass
        }
        data-testid={stackedOnMobile ? "data-table-table" : undefined}
      >
        <div className="overflow-x-auto">
          <table className={tableClass}>
            <thead>
              <tr className={headRowClass}>
                {columns.map((col) => {
                  const align = alignClass[col.align ?? "left"];
                  const sortable = Boolean(col.sortValue);
                  const active = sort?.key === col.key;
                  if (!sortable) {
                    return (
                      <th
                        key={col.key}
                        scope="col"
                        className={`${headCellClass} ${align} ${col.headerClassName ?? ""}`}
                      >
                        {col.header}
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
                      className={`${headCellClass} ${align} ${col.headerClassName ?? ""}`}
                    >
                      <button
                        type="button"
                        onClick={() => toggleSort(col.key)}
                        className={`inline-flex items-center gap-1 font-medium transition-colors select-none hover:text-gray-700 focus:rounded focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-400 ${align}`}
                      >
                        <span>{col.header}</span>
                        <span
                          aria-hidden="true"
                          className={active ? "text-gray-700" : "text-gray-300"}
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
                  columnCount={columns.length}
                  rows={loadingRowCount}
                />
              ) : sortedRows.length === 0 ? (
                <tr>
                  <td
                    colSpan={columns.length}
                    className="px-5 py-10 text-center text-sm text-gray-500"
                  >
                    {emptyState ?? "Keine Einträge vorhanden."}
                  </td>
                </tr>
              ) : (
                visibleRows.map((row) => {
                  const rowKey = getRowKey(row);
                  const rowClasses = [
                    `${bodyRowClass} transition-colors`,
                    clickable ? "cursor-pointer hover:bg-gray-50" : "",
                    rowClassName ? rowClassName(row) : "",
                  ]
                    .filter(Boolean)
                    .join(" ");
                  return (
                    <tr
                      key={rowKey}
                      className={rowClasses}
                      onClick={onRowClick ? () => onRowClick(row) : undefined}
                      onKeyDown={
                        onRowClick
                          ? (event) => {
                              if (event.target !== event.currentTarget) {
                                return;
                              }
                              if (event.key === "Enter" || event.key === " ") {
                                event.preventDefault();
                                onRowClick(row);
                              }
                            }
                          : undefined
                      }
                      tabIndex={clickable ? 0 : undefined}
                      role={clickable ? "button" : undefined}
                    >
                      {columns.map((col) => {
                        const align = alignClass[col.align ?? "left"];
                        return (
                          <td
                            key={col.key}
                            className={`${bodyCellClass} text-gray-900 ${align} ${col.className ?? ""}`}
                          >
                            {col.render(row)}
                          </td>
                        );
                      })}
                    </tr>
                  );
                })
              )}
              {!isLoading && hasMore && (
                <tr className="border-t border-gray-100">
                  <td
                    colSpan={columns.length}
                    className="px-5 py-3 text-center"
                  >
                    <button
                      type="button"
                      onClick={loadMore}
                      className="rounded-lg border border-gray-200 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-400"
                    >
                      {`Mehr laden (${visibleRows.length} von ${sortedRows.length})`}
                    </button>
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
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
  // the only difference. DANGER keeps the red this badge carried before the
  // palette move, when inactive was LOCATION_COLORS.HOME (#FF3130).
  const color = unknown
    ? LOCATION_COLORS.UNKNOWN
    : active
      ? LOCATION_COLORS.GROUP_ROOM
      : LOCATION_COLORS.DANGER;
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full bg-gray-50 px-3 py-1 text-xs font-medium">
      <span
        aria-hidden
        className="inline-block h-1.5 w-1.5 rounded-full"
        style={{ backgroundColor: color }}
      />
      <span style={{ color }}>{label}</span>
    </span>
  );
}
