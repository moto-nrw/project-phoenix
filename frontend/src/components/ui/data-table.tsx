"use client";

import { useCallback, useMemo, useState, type ReactNode } from "react";

import { LOCATION_COLORS } from "~/lib/location-helper";

type SortDirection = "asc" | "desc";

export interface DataTableColumn<T> {
  key: string;
  header: string;
  render: (row: T) => ReactNode;
  align?: "left" | "right" | "center";
  className?: string;
  headerClassName?: string;
  sortValue?: (row: T) => string | number;
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
  rowClassName?: (row: T) => string;
  defaultSortKey?: string;
  defaultSortDirection?: SortDirection;
}

const alignClass: Record<
  NonNullable<DataTableColumn<unknown>["align"]>,
  string
> = {
  left: "text-left",
  right: "text-right",
  center: "text-center",
};

export function DataTable<T>({
  columns,
  rows,
  getRowKey,
  onRowClick,
  emptyState,
  headerActions,
  caption,
  isLoading,
  rowClassName,
  defaultSortKey,
  defaultSortDirection = "asc",
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
      if (av < bv) return -dir;
      if (av > bv) return dir;
      return 0;
    });
  }, [rows, sort, columns]);

  return (
    <div className="w-full">
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

      <div className="overflow-x-auto rounded-2xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md">
        <table className="w-full border-collapse text-left text-sm">
          <thead>
            <tr className="border-b border-gray-100 text-xs font-medium text-gray-500">
              {columns.map((col) => {
                const align = alignClass[col.align ?? "left"];
                const sortable = Boolean(col.sortValue);
                const active = sort?.key === col.key;
                if (!sortable) {
                  return (
                    <th
                      key={col.key}
                      scope="col"
                      className={`px-5 py-3 ${align} ${col.headerClassName ?? ""}`}
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
                    className={`px-5 py-3 ${align} ${col.headerClassName ?? ""}`}
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
              <tr>
                <td
                  colSpan={columns.length}
                  className="px-5 py-10 text-center text-sm text-gray-500"
                >
                  Wird geladen…
                </td>
              </tr>
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
              sortedRows.map((row) => {
                const rowKey = getRowKey(row);
                const rowClasses = [
                  "border-b border-gray-50 last:border-0 transition-colors",
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
                          className={`px-5 py-3 align-middle text-gray-900 ${align} ${col.className ?? ""}`}
                        >
                          {col.render(row)}
                        </td>
                      );
                    })}
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

interface DataTableStatusBadgeProps {
  active: boolean;
  activeLabel?: string;
  inactiveLabel?: string;
}

export function DataTableStatusBadge({
  active,
  activeLabel = "Aktiv",
  inactiveLabel = "Inaktiv",
}: Readonly<DataTableStatusBadgeProps>) {
  const label = active ? activeLabel : inactiveLabel;
  const color = active ? LOCATION_COLORS.GROUP_ROOM : LOCATION_COLORS.HOME;
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
